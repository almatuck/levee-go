package levee

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/almatuck/levee-go/llmpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// LLMClient provides access to the Levee LLM gateway.
type LLMClient struct {
	apiKey     string
	baseURL    string // Base URL for HTTP API (e.g., "https://levee.example.com")
	grpcAddr   string // gRPC address override (host:port), if empty uses auto-discovery
	useTLS     bool   // Determined from baseURL scheme (https = true)
	httpClient *http.Client
	conn       *grpc.ClientConn
	client     llmpb.LLMServiceClient
	mu         sync.Mutex
}

// LLMOption is a functional option for configuring the LLM client.
type LLMOption func(*LLMClient)

// WithGRPCAddress sets the gRPC server address explicitly (host:port).
// If not set, the address will be auto-discovered from the config endpoint.
func WithGRPCAddress(addr string) LLMOption {
	return func(c *LLMClient) {
		c.grpcAddr = addr
	}
}

// WithLLMHTTPClient sets a custom HTTP client for config API calls.
func WithLLMHTTPClient(client *http.Client) LLMOption {
	return func(c *LLMClient) {
		c.httpClient = client
	}
}

// NewLLMClient creates a new LLM client.
//
// baseURL is the Levee API URL (e.g., "https://levee.example.com").
// TLS is automatically determined from the URL scheme:
//   - https:// → TLS enabled
//   - http:// → plaintext (development only)
//
// The gRPC port is auto-discovered from /sdk/v1/llm/config unless
// explicitly set with WithGRPCAddress.
func NewLLMClient(apiKey string, baseURL string, opts ...LLMOption) *LLMClient {
	c := &LLMClient{
		apiKey:     apiKey,
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		useTLS:     strings.HasPrefix(baseURL, "https://"),
		httpClient: http.DefaultClient,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// llmConfigResponse represents the response from /sdk/v1/llm/config
type llmConfigResponse struct {
	Available           bool     `json:"available"`
	GRPCPort            int      `json:"grpc_port"`
	DefaultProvider     string   `json:"default_provider"`
	ConfiguredProviders []string `json:"configured_providers"`
}

// fetchConfig fetches the LLM configuration from the API.
func (c *LLMClient) fetchConfig(ctx context.Context) (*llmConfigResponse, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("baseURL is required for auto-discovery")
	}

	reqURL := c.baseURL + "/sdk/v1/llm/config"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create config request: %w", err)
	}

	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch LLM config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("config request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var config llmConfigResponse
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode config response: %w", err)
	}

	return &config, nil
}

// connect establishes the gRPC connection if not already connected.
func (c *LLMClient) connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return nil
	}

	// Determine gRPC address
	grpcAddr := c.grpcAddr
	if grpcAddr == "" {
		// Auto-discover from config endpoint
		config, err := c.fetchConfig(context.Background())
		if err != nil {
			return fmt.Errorf("failed to auto-discover gRPC address: %w", err)
		}

		if !config.Available {
			return fmt.Errorf("LLM service is not available for this organization")
		}

		// Extract host from baseURL
		parsedURL, err := url.Parse(c.baseURL)
		if err != nil {
			return fmt.Errorf("failed to parse baseURL: %w", err)
		}

		grpcAddr = fmt.Sprintf("%s:%d", parsedURL.Hostname(), config.GRPCPort)
		c.grpcAddr = grpcAddr // Cache for future connections
	}

	// Create gRPC connection with appropriate credentials (TLS determined from baseURL scheme)
	var opts []grpc.DialOption
	if c.useTLS {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.NewClient(grpcAddr, opts...)
	if err != nil {
		return fmt.Errorf("failed to connect to LLM server at %s: %w", grpcAddr, err)
	}

	c.conn = conn
	c.client = llmpb.NewLLMServiceClient(conn)
	return nil
}

// Close closes the gRPC connection.
func (c *LLMClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		c.client = nil
		return err
	}
	return nil
}

// ChatMessage represents a message in a conversation.
type ChatMessage struct {
	Role    string `json:"role"`    // "user", "assistant", "system"
	Content string `json:"content"`
}

// ChatRequest represents an LLM chat request.
type ChatRequest struct {
	Messages     []ChatMessage
	SystemPrompt string
	Model        string // "haiku", "sonnet", "opus" or full model ID
	MaxTokens    int32
	Temperature  float32
}

// ChatResponse represents an LLM chat response.
type ChatResponse struct {
	Content      string
	Model        string
	InputTokens  int64
	OutputTokens int64
	CostUSD      float64
	LatencyMs    int64
	StopReason   string
}

// Chat sends a simple (non-streaming) chat request.
func (c *LLMClient) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if err := c.connect(); err != nil {
		return nil, err
	}

	// Convert messages
	messages := make([]*llmpb.Message, 0, len(req.Messages))
	for _, msg := range req.Messages {
		messages = append(messages, &llmpb.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	resp, err := c.client.SimpleChat(ctx, &llmpb.SimpleChatRequest{
		ApiKey:       c.apiKey,
		Messages:     messages,
		SystemPrompt: req.SystemPrompt,
		Model:        req.Model,
		MaxTokens:    req.MaxTokens,
		Temperature:  req.Temperature,
	})
	if err != nil {
		return nil, fmt.Errorf("chat request failed: %w", err)
	}

	return &ChatResponse{
		Content:      resp.Content,
		Model:        resp.Model,
		InputTokens:  resp.InputTokens,
		OutputTokens: resp.OutputTokens,
		CostUSD:      resp.CostUsd,
		LatencyMs:    resp.LatencyMs,
		StopReason:   resp.StopReason,
	}, nil
}

// StreamChunk represents a chunk of streamed content.
type StreamChunk struct {
	Content string
	Index   int32
}

// StreamCallback is called for each chunk during streaming.
type StreamCallback func(chunk StreamChunk) error

// ChatSession represents an active chat session for bidirectional streaming.
type ChatSession struct {
	stream llmpb.LLMService_ChatClient
	apiKey string
	done   bool
	mu     sync.Mutex
}

// NewChatSession starts a new bidirectional chat session.
func (c *LLMClient) NewChatSession(ctx context.Context, req ChatRequest) (*ChatSession, error) {
	if err := c.connect(); err != nil {
		return nil, err
	}

	stream, err := c.client.Chat(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start chat session: %w", err)
	}

	// Convert initial messages
	messages := make([]*llmpb.Message, 0, len(req.Messages))
	for _, msg := range req.Messages {
		messages = append(messages, &llmpb.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Send start request
	err = stream.Send(&llmpb.ChatRequest{
		Request: &llmpb.ChatRequest_Start{
			Start: &llmpb.StartChatRequest{
				ApiKey:       c.apiKey,
				SystemPrompt: req.SystemPrompt,
				Model:        req.Model,
				MaxTokens:    req.MaxTokens,
				Temperature:  req.Temperature,
				Messages:     messages,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send start request: %w", err)
	}

	return &ChatSession{
		stream: stream,
		apiKey: c.apiKey,
	}, nil
}

// Send sends a user message and streams the response.
func (s *ChatSession) Send(ctx context.Context, content string, callback StreamCallback) (*ChatResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.done {
		return nil, fmt.Errorf("session is closed")
	}

	// Send user message
	err := s.stream.Send(&llmpb.ChatRequest{
		Request: &llmpb.ChatRequest_Message{
			Message: &llmpb.UserMessage{
				Content: content,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	// Stream responses until completion
	var fullContent string
	var completion *llmpb.CompletionResponse

	for {
		resp, err := s.stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("stream receive error: %w", err)
		}

		switch r := resp.Response.(type) {
		case *llmpb.ChatResponse_SessionStarted:
			// Session started, continue
		case *llmpb.ChatResponse_Chunk:
			fullContent += r.Chunk.Content
			if callback != nil {
				if err := callback(StreamChunk{Content: r.Chunk.Content, Index: r.Chunk.Index}); err != nil {
					return nil, err
				}
			}
		case *llmpb.ChatResponse_Completion:
			completion = r.Completion
			// Don't break - there might be more responses
		case *llmpb.ChatResponse_Error:
			return nil, fmt.Errorf("LLM error: %s", r.Error.Message)
		case *llmpb.ChatResponse_Aborted:
			return nil, fmt.Errorf("generation aborted: %s", r.Aborted.Reason)
		}

		if completion != nil {
			break
		}
	}

	if completion == nil {
		return &ChatResponse{Content: fullContent}, nil
	}

	return &ChatResponse{
		Content:      completion.FullContent,
		StopReason:   completion.StopReason,
		InputTokens:  completion.InputTokens,
		OutputTokens: completion.OutputTokens,
		CostUSD:      completion.CostUsd,
		LatencyMs:    completion.LatencyMs,
	}, nil
}

// Abort aborts the current generation.
func (s *ChatSession) Abort(reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.stream.Send(&llmpb.ChatRequest{
		Request: &llmpb.ChatRequest_Abort{
			Abort: &llmpb.AbortRequest{
				Reason: reason,
			},
		},
	})
}

// Close closes the chat session.
func (s *ChatSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.done = true
	return s.stream.CloseSend()
}

// ChatStream sends a message and streams the response via callback.
// This is a convenience method for simple streaming use cases.
func (c *LLMClient) ChatStream(ctx context.Context, req ChatRequest, callback StreamCallback) (*ChatResponse, error) {
	session, err := c.NewChatSession(ctx, ChatRequest{
		SystemPrompt: req.SystemPrompt,
		Model:        req.Model,
		MaxTokens:    req.MaxTokens,
		Temperature:  req.Temperature,
	})
	if err != nil {
		return nil, err
	}
	defer session.Close()

	// If there are existing messages, we need to send them as the initial context
	// and then send the last user message
	if len(req.Messages) == 0 {
		return nil, fmt.Errorf("at least one message is required")
	}

	lastMsg := req.Messages[len(req.Messages)-1]
	if lastMsg.Role != "user" {
		return nil, fmt.Errorf("last message must be from user")
	}

	return session.Send(ctx, lastMsg.Content, callback)
}
