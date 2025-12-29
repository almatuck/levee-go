package levee

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/almatuck/levee-go/llmpb"
	"github.com/gorilla/websocket"
)

// WebSocket message types for LLM chat
const (
	WSMsgTypeStart      = "start"
	WSMsgTypeMessage    = "message"
	WSMsgTypeAbort      = "abort"
	WSMsgTypeChunk      = "chunk"
	WSMsgTypeCompletion = "completion"
	WSMsgTypeError      = "error"
	WSMsgTypeStarted    = "started"
	WSMsgTypeToolCall   = "tool_call"
	WSMsgTypeToolResult = "tool_result"
)

// WSMessage is the base WebSocket message envelope.
type WSMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// WSStartRequest starts a new chat session.
type WSStartRequest struct {
	SystemPrompt string        `json:"system_prompt,omitempty"`
	Model        string        `json:"model,omitempty"` // "haiku", "sonnet", "opus"
	MaxTokens    int32         `json:"max_tokens,omitempty"`
	Temperature  float32       `json:"temperature,omitempty"`
	Messages     []ChatMessage `json:"messages,omitempty"`
}

// WSUserMessage sends a user message.
type WSUserMessage struct {
	Content string `json:"content"`
}

// WSAbortRequest aborts the current generation.
type WSAbortRequest struct {
	Reason string `json:"reason,omitempty"`
}

// WSToolResult provides a tool call result.
type WSToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Result     string `json:"result"`
	IsError    bool   `json:"is_error,omitempty"`
}

// WSStartedResponse confirms session started.
type WSStartedResponse struct {
	SessionID string `json:"session_id"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
}

// WSChunkResponse streams content chunks.
type WSChunkResponse struct {
	Content string `json:"content"`
	Index   int32  `json:"index"`
}

// WSToolCallResponse indicates LLM wants to call a tool.
type WSToolCallResponse struct {
	ToolCallID    string `json:"tool_call_id"`
	Name          string `json:"name"`
	ArgumentsJSON string `json:"arguments_json"`
}

// WSCompletionResponse indicates generation complete.
type WSCompletionResponse struct {
	FullContent  string  `json:"full_content"`
	StopReason   string  `json:"stop_reason"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
	LatencyMs    int64   `json:"latency_ms"`
}

// WSErrorResponse indicates an error.
type WSErrorResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

// WSConfig configures the WebSocket handler.
type WSConfig struct {
	// CheckOrigin is called to check the origin of the WebSocket request.
	// If nil, allows all origins.
	CheckOrigin func(r *http.Request) bool
}

// WSOption is a functional option for configuring the WebSocket handler.
type WSOption func(*WSConfig)

// WithCheckOrigin sets the origin checker for WebSocket connections.
func WithCheckOrigin(fn func(r *http.Request) bool) WSOption {
	return func(c *WSConfig) {
		c.CheckOrigin = fn
	}
}

// upgrader is the WebSocket upgrader with default settings.
var defaultUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins by default
	},
}

// HandleChatWebSocket returns a handler for WebSocket LLM chat.
// This bridges WebSocket connections to the gRPC LLM stream.
// Route: GET /your-prefix/ws/chat (upgrades to WebSocket)
func (c *Client) HandleChatWebSocket(llm *LLMClient, opts ...WSOption) http.HandlerFunc {
	cfg := &WSConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	upgrader := defaultUpgrader
	if cfg.CheckOrigin != nil {
		upgrader.CheckOrigin = cfg.CheckOrigin
	}

	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		session := &wsSession{
			conn:   conn,
			llm:    llm,
			ctx:    r.Context(),
			sendMu: sync.Mutex{},
		}

		session.run()
	}
}

// wsSession manages a single WebSocket chat session.
type wsSession struct {
	conn     *websocket.Conn
	llm      *LLMClient
	ctx      context.Context
	stream   llmpb.LLMService_ChatClient
	sendMu   sync.Mutex
	started  bool
}

// run is the main loop for the WebSocket session.
func (s *wsSession) run() {
	for {
		_, message, err := s.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// Log error if needed
			}
			return
		}

		var msg WSMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			s.sendError("invalid_json", "Invalid JSON message", false)
			continue
		}

		switch msg.Type {
		case WSMsgTypeStart:
			s.handleStart(msg.Data)
		case WSMsgTypeMessage:
			s.handleMessage(msg.Data)
		case WSMsgTypeAbort:
			s.handleAbort(msg.Data)
		case WSMsgTypeToolResult:
			s.handleToolResult(msg.Data)
		default:
			s.sendError("unknown_type", fmt.Sprintf("Unknown message type: %s", msg.Type), false)
		}
	}
}

// handleStart initializes the gRPC stream and starts a chat session.
func (s *wsSession) handleStart(data json.RawMessage) {
	if s.started {
		s.sendError("already_started", "Session already started", false)
		return
	}

	var req WSStartRequest
	if err := json.Unmarshal(data, &req); err != nil {
		s.sendError("invalid_data", "Invalid start request", false)
		return
	}

	// Connect to gRPC if needed
	if err := s.llm.connect(); err != nil {
		s.sendError("connection_failed", err.Error(), true)
		return
	}

	// Start bidirectional stream
	stream, err := s.llm.client.Chat(s.ctx)
	if err != nil {
		s.sendError("stream_failed", err.Error(), true)
		return
	}
	s.stream = stream

	// Convert messages
	messages := make([]*llmpb.Message, 0, len(req.Messages))
	for _, msg := range req.Messages {
		messages = append(messages, &llmpb.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Send start request to gRPC
	err = stream.Send(&llmpb.ChatRequest{
		Request: &llmpb.ChatRequest_Start{
			Start: &llmpb.StartChatRequest{
				ApiKey:       s.llm.apiKey,
				SystemPrompt: req.SystemPrompt,
				Model:        req.Model,
				MaxTokens:    req.MaxTokens,
				Temperature:  req.Temperature,
				Messages:     messages,
			},
		},
	})
	if err != nil {
		s.sendError("start_failed", err.Error(), true)
		return
	}

	s.started = true

	// Start goroutine to read gRPC responses
	go s.readGRPCResponses()
}

// handleMessage sends a user message to the gRPC stream.
func (s *wsSession) handleMessage(data json.RawMessage) {
	if !s.started || s.stream == nil {
		s.sendError("not_started", "Session not started", false)
		return
	}

	var msg WSUserMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		s.sendError("invalid_data", "Invalid message", false)
		return
	}

	err := s.stream.Send(&llmpb.ChatRequest{
		Request: &llmpb.ChatRequest_Message{
			Message: &llmpb.UserMessage{
				Content: msg.Content,
			},
		},
	})
	if err != nil {
		s.sendError("send_failed", err.Error(), true)
	}
}

// handleAbort aborts the current generation.
func (s *wsSession) handleAbort(data json.RawMessage) {
	if !s.started || s.stream == nil {
		return
	}

	var req WSAbortRequest
	json.Unmarshal(data, &req)

	s.stream.Send(&llmpb.ChatRequest{
		Request: &llmpb.ChatRequest_Abort{
			Abort: &llmpb.AbortRequest{
				Reason: req.Reason,
			},
		},
	})
}

// handleToolResult sends a tool result to the gRPC stream.
func (s *wsSession) handleToolResult(data json.RawMessage) {
	if !s.started || s.stream == nil {
		s.sendError("not_started", "Session not started", false)
		return
	}

	var result WSToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		s.sendError("invalid_data", "Invalid tool result", false)
		return
	}

	err := s.stream.Send(&llmpb.ChatRequest{
		Request: &llmpb.ChatRequest_ToolResult{
			ToolResult: &llmpb.ToolResult{
				ToolCallId: result.ToolCallID,
				Result:     result.Result,
				IsError:    result.IsError,
			},
		},
	})
	if err != nil {
		s.sendError("send_failed", err.Error(), true)
	}
}

// readGRPCResponses reads from the gRPC stream and forwards to WebSocket.
func (s *wsSession) readGRPCResponses() {
	for {
		resp, err := s.stream.Recv()
		if err == io.EOF {
			return
		}
		if err != nil {
			s.sendError("stream_error", err.Error(), false)
			return
		}

		switch r := resp.Response.(type) {
		case *llmpb.ChatResponse_SessionStarted:
			s.send(WSMsgTypeStarted, WSStartedResponse{
				SessionID: r.SessionStarted.SessionId,
				Provider:  r.SessionStarted.Provider,
				Model:     r.SessionStarted.Model,
			})

		case *llmpb.ChatResponse_Chunk:
			s.send(WSMsgTypeChunk, WSChunkResponse{
				Content: r.Chunk.Content,
				Index:   r.Chunk.Index,
			})

		case *llmpb.ChatResponse_ToolCall:
			s.send(WSMsgTypeToolCall, WSToolCallResponse{
				ToolCallID:    r.ToolCall.ToolCallId,
				Name:          r.ToolCall.Name,
				ArgumentsJSON: r.ToolCall.ArgumentsJson,
			})

		case *llmpb.ChatResponse_Completion:
			s.send(WSMsgTypeCompletion, WSCompletionResponse{
				FullContent:  r.Completion.FullContent,
				StopReason:   r.Completion.StopReason,
				InputTokens:  r.Completion.InputTokens,
				OutputTokens: r.Completion.OutputTokens,
				CostUSD:      r.Completion.CostUsd,
				LatencyMs:    r.Completion.LatencyMs,
			})

		case *llmpb.ChatResponse_Error:
			s.send(WSMsgTypeError, WSErrorResponse{
				Code:      r.Error.Code,
				Message:   r.Error.Message,
				Retryable: r.Error.Retryable,
			})

		case *llmpb.ChatResponse_Aborted:
			s.send(WSMsgTypeError, WSErrorResponse{
				Code:    "aborted",
				Message: r.Aborted.Reason,
			})
		}
	}
}

// send marshals and sends a message over WebSocket.
func (s *wsSession) send(msgType string, data interface{}) {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()

	dataBytes, err := json.Marshal(data)
	if err != nil {
		return
	}

	msg := WSMessage{
		Type: msgType,
		Data: dataBytes,
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return
	}

	s.conn.WriteMessage(websocket.TextMessage, msgBytes)
}

// sendError sends an error message over WebSocket.
func (s *wsSession) sendError(code, message string, retryable bool) {
	s.send(WSMsgTypeError, WSErrorResponse{
		Code:      code,
		Message:   message,
		Retryable: retryable,
	})
}
