package levee

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// HandlerConfig configures the embedded HTTP handlers.
type HandlerConfig struct {
	// UnsubscribeRedirect is the URL to redirect to after unsubscribe (default: /unsubscribed)
	UnsubscribeRedirect string
	// ConfirmRedirect is the URL to redirect to after email confirmation (default: /confirmed)
	ConfirmRedirect string
	// ConfirmExpiredRedirect is the URL to redirect to if confirmation token expired (default: /confirm-expired)
	ConfirmExpiredRedirect string
	// StripeWebhookSecret is the Stripe webhook signing secret for signature verification
	StripeWebhookSecret string
	// LLMClient is the optional LLM client for WebSocket chat handler
	LLMClient *LLMClient
	// WSCheckOrigin is the origin checker for WebSocket connections (nil allows all)
	WSCheckOrigin func(r *http.Request) bool
}

// HandlerOption is a functional option for configuring handlers.
type HandlerOption func(*HandlerConfig)

// WithUnsubscribeRedirect sets the redirect URL after unsubscribe.
func WithUnsubscribeRedirect(url string) HandlerOption {
	return func(c *HandlerConfig) {
		c.UnsubscribeRedirect = url
	}
}

// WithConfirmRedirect sets the redirect URL after email confirmation.
func WithConfirmRedirect(url string) HandlerOption {
	return func(c *HandlerConfig) {
		c.ConfirmRedirect = url
	}
}

// WithConfirmExpiredRedirect sets the redirect URL for expired confirmation tokens.
func WithConfirmExpiredRedirect(url string) HandlerOption {
	return func(c *HandlerConfig) {
		c.ConfirmExpiredRedirect = url
	}
}

// WithStripeWebhookSecret sets the Stripe webhook signing secret.
func WithStripeWebhookSecret(secret string) HandlerOption {
	return func(c *HandlerConfig) {
		c.StripeWebhookSecret = secret
	}
}

// WithLLMClient sets the LLM client for WebSocket chat handler.
func WithLLMClient(llm *LLMClient) HandlerOption {
	return func(c *HandlerConfig) {
		c.LLMClient = llm
	}
}

// WithWSCheckOrigin sets the origin checker for WebSocket connections.
func WithWSCheckOrigin(fn func(r *http.Request) bool) HandlerOption {
	return func(c *HandlerConfig) {
		c.WSCheckOrigin = fn
	}
}

// 1x1 transparent GIF (43 bytes)
var transparentGIF = []byte{
	0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00,
	0x01, 0x00, 0x80, 0x00, 0x00, 0xff, 0xff, 0xff,
	0x00, 0x00, 0x00, 0x21, 0xf9, 0x04, 0x01, 0x00,
	0x00, 0x00, 0x00, 0x2c, 0x00, 0x00, 0x00, 0x00,
	0x01, 0x00, 0x01, 0x00, 0x00, 0x02, 0x02, 0x44,
	0x01, 0x00, 0x3b,
}

// NewHandlerConfig creates a new HandlerConfig with the given options.
// Use this when registering handlers individually with custom routers.
func NewHandlerConfig(opts ...HandlerOption) *HandlerConfig {
	cfg := &HandlerConfig{
		UnsubscribeRedirect:    "/unsubscribed",
		ConfirmRedirect:        "/confirmed",
		ConfirmExpiredRedirect: "/confirm-expired",
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

// RegisterHandlers registers all Levee HTTP handlers on the given mux with the specified prefix.
// Example: client.RegisterHandlers(mux, "/levee") registers handlers at /levee/e/o/:token, etc.
func (c *Client) RegisterHandlers(mux *http.ServeMux, prefix string, opts ...HandlerOption) {
	cfg := &HandlerConfig{
		UnsubscribeRedirect:    "/unsubscribed",
		ConfirmRedirect:        "/confirmed",
		ConfirmExpiredRedirect: "/confirm-expired",
	}

	for _, opt := range opts {
		opt(cfg)
	}

	// Email tracking
	mux.HandleFunc(prefix+"/e/o/", c.handleOpenTracking())
	mux.HandleFunc(prefix+"/e/c/", c.handleClickTracking())
	mux.HandleFunc(prefix+"/e/u/", c.handleUnsubscribe(cfg))

	// Email confirmation
	mux.HandleFunc(prefix+"/confirm-email", c.handleConfirmEmail(cfg))

	// Webhooks
	mux.HandleFunc(prefix+"/webhooks/stripe", c.handleStripeWebhook(cfg))
	mux.HandleFunc(prefix+"/webhooks/ses", c.handleSESWebhook())

	// WebSocket LLM chat (if LLM client provided)
	if cfg.LLMClient != nil {
		var wsOpts []WSOption
		if cfg.WSCheckOrigin != nil {
			wsOpts = append(wsOpts, WithCheckOrigin(cfg.WSCheckOrigin))
		}
		mux.HandleFunc(prefix+"/ws/chat", c.HandleChatWebSocket(cfg.LLMClient, wsOpts...))
	}
}

// handleOpenTracking handles email open tracking pixel requests.
// GET /prefix/e/o/:token
func (c *Client) handleOpenTracking() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		token := extractToken(r.URL.Path, "/e/o/")
		if token == "" {
			http.Error(w, "Missing token", http.StatusBadRequest)
			return
		}

		// Record open asynchronously
		go func() {
			ctx := context.Background()
			c.RecordOpen(ctx, token)
		}()

		// Return 1x1 transparent GIF
		w.Header().Set("Content-Type", "image/gif")
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		w.Write(transparentGIF)
	}
}

// handleClickTracking handles email click tracking requests.
// GET /prefix/e/c/:token?url=...
func (c *Client) handleClickTracking() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		token := extractToken(r.URL.Path, "/e/c/")
		if token == "" {
			http.Error(w, "Missing token", http.StatusBadRequest)
			return
		}

		redirectURL := r.URL.Query().Get("url")
		if redirectURL == "" {
			http.Error(w, "Missing url parameter", http.StatusBadRequest)
			return
		}

		// Record click asynchronously
		go func() {
			ctx := context.Background()
			c.RecordClick(ctx, token, redirectURL)
		}()

		// Redirect to destination
		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
	}
}

// handleUnsubscribe handles one-click unsubscribe requests.
// GET /prefix/e/u/:token
func (c *Client) handleUnsubscribe(cfg *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		token := extractToken(r.URL.Path, "/e/u/")
		if token == "" {
			http.Error(w, "Missing token", http.StatusBadRequest)
			return
		}

		// Record unsubscribe (synchronous - we want to confirm it worked)
		ctx := r.Context()
		err := c.RecordUnsubscribe(ctx, token)
		if err != nil {
			http.Error(w, "Failed to unsubscribe", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, cfg.UnsubscribeRedirect, http.StatusTemporaryRedirect)
	}
}

// handleConfirmEmail handles double opt-in email confirmation.
// GET /prefix/confirm-email?token=...
func (c *Client) handleConfirmEmail(cfg *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "Missing token", http.StatusBadRequest)
			return
		}

		ctx := r.Context()
		resp, err := c.ConfirmEmail(ctx, token)
		if err != nil {
			http.Redirect(w, r, cfg.ConfirmExpiredRedirect, http.StatusTemporaryRedirect)
			return
		}

		redirect := cfg.ConfirmRedirect
		if resp.RedirectURL != "" {
			redirect = resp.RedirectURL
		}

		http.Redirect(w, r, redirect, http.StatusTemporaryRedirect)
	}
}

// handleStripeWebhook handles Stripe webhook events.
// POST /prefix/webhooks/stripe
func (c *Client) handleStripeWebhook(cfg *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}

		// Verify signature if secret is configured
		if cfg.StripeWebhookSecret != "" {
			signature := r.Header.Get("Stripe-Signature")
			if !verifyStripeSignature(body, signature, cfg.StripeWebhookSecret) {
				http.Error(w, "Invalid signature", http.StatusUnauthorized)
				return
			}
		}

		// Forward to Levee API
		ctx := r.Context()
		err = c.ForwardStripeWebhook(ctx, body, r.Header.Get("Stripe-Signature"))
		if err != nil {
			http.Error(w, "Failed to process webhook", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"received": true}`))
	}
}

// handleSESWebhook handles AWS SES bounce/complaint notifications.
// POST /prefix/webhooks/ses
func (c *Client) handleSESWebhook() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}

		// Check for SNS subscription confirmation
		var snsMessage struct {
			Type         string `json:"Type"`
			SubscribeURL string `json:"SubscribeURL"`
		}
		if err := json.Unmarshal(body, &snsMessage); err == nil {
			if snsMessage.Type == "SubscriptionConfirmation" && snsMessage.SubscribeURL != "" {
				// Confirm SNS subscription
				resp, err := http.Get(snsMessage.SubscribeURL)
				if err == nil {
					resp.Body.Close()
				}
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		// Forward to Levee API
		ctx := r.Context()
		err = c.ForwardSESWebhook(ctx, body)
		if err != nil {
			http.Error(w, "Failed to process webhook", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// extractToken extracts the token from a URL path after the given prefix.
func extractToken(path, prefix string) string {
	idx := strings.LastIndex(path, prefix)
	if idx == -1 {
		return ""
	}
	return strings.TrimSuffix(path[idx+len(prefix):], "/")
}

// getToken extracts token from request using multiple methods:
// 1. r.PathValue("token") - Go 1.22+ / go-zero
// 2. URL path extraction - http.ServeMux fallback
func getToken(r *http.Request, pathPrefix string) string {
	// Try PathValue first (Go 1.22+ / go-zero)
	if token := r.PathValue("token"); token != "" {
		return token
	}
	// Fallback to path extraction
	return extractToken(r.URL.Path, pathPrefix)
}

// ============================================================================
// Exported Handlers - For custom router integration (go-zero, chi, gorilla, etc.)
// ============================================================================

// HandleOpenTracking returns a handler for email open tracking.
// Serves a 1x1 transparent GIF and records the open event.
// Route: GET /your-prefix/e/o/:token
func (c *Client) HandleOpenTracking(cfg *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		token := getToken(r, "/e/o/")
		if token == "" {
			http.Error(w, "Missing token", http.StatusBadRequest)
			return
		}

		// Record open asynchronously
		go func() {
			ctx := context.Background()
			c.RecordOpen(ctx, token)
		}()

		// Return 1x1 transparent GIF
		w.Header().Set("Content-Type", "image/gif")
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		w.Write(transparentGIF)
	}
}

// HandleClickTracking returns a handler for email click tracking.
// Records the click and redirects to the destination URL.
// Route: GET /your-prefix/e/c/:token?url=...
func (c *Client) HandleClickTracking(cfg *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		token := getToken(r, "/e/c/")
		if token == "" {
			http.Error(w, "Missing token", http.StatusBadRequest)
			return
		}

		redirectURL := r.URL.Query().Get("url")
		if redirectURL == "" {
			http.Error(w, "Missing url parameter", http.StatusBadRequest)
			return
		}

		// Record click asynchronously
		go func() {
			ctx := context.Background()
			c.RecordClick(ctx, token, redirectURL)
		}()

		// Redirect to destination
		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
	}
}

// HandleUnsubscribe returns a handler for one-click unsubscribe.
// Records the unsubscribe and redirects to the configured URL.
// Route: GET /your-prefix/e/u/:token
func (c *Client) HandleUnsubscribe(cfg *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		token := getToken(r, "/e/u/")
		if token == "" {
			http.Error(w, "Missing token", http.StatusBadRequest)
			return
		}

		// Record unsubscribe (synchronous - we want to confirm it worked)
		ctx := r.Context()
		_ = c.RecordUnsubscribe(ctx, token)

		http.Redirect(w, r, cfg.UnsubscribeRedirect, http.StatusTemporaryRedirect)
	}
}

// HandleConfirmEmail returns a handler for double opt-in email confirmation.
// Route: GET /your-prefix/confirm-email?token=...
func (c *Client) HandleConfirmEmail(cfg *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "Missing token", http.StatusBadRequest)
			return
		}

		ctx := r.Context()
		resp, err := c.ConfirmEmail(ctx, token)
		if err != nil {
			http.Redirect(w, r, cfg.ConfirmExpiredRedirect, http.StatusTemporaryRedirect)
			return
		}

		redirect := cfg.ConfirmRedirect
		if resp.RedirectURL != "" {
			redirect = resp.RedirectURL
		}

		http.Redirect(w, r, redirect, http.StatusTemporaryRedirect)
	}
}

// HandleStripeWebhook returns a handler for Stripe webhook events.
// Verifies signature and forwards to Levee API.
// Route: POST /your-prefix/webhooks/stripe
func (c *Client) HandleStripeWebhook(cfg *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}

		// Verify signature if secret is configured
		if cfg.StripeWebhookSecret != "" {
			signature := r.Header.Get("Stripe-Signature")
			if !verifyStripeSignature(body, signature, cfg.StripeWebhookSecret) {
				http.Error(w, "Invalid signature", http.StatusUnauthorized)
				return
			}
		}

		// Forward to Levee API
		ctx := r.Context()
		err = c.ForwardStripeWebhook(ctx, body, r.Header.Get("Stripe-Signature"))
		if err != nil {
			http.Error(w, "Failed to process webhook", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"received": true}`))
	}
}

// HandleSESWebhook returns a handler for AWS SES bounce/complaint notifications.
// Handles SNS subscription confirmation and forwards events to Levee API.
// Route: POST /your-prefix/webhooks/ses
func (c *Client) HandleSESWebhook(cfg *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}

		// Check for SNS subscription confirmation
		var snsMessage struct {
			Type         string `json:"Type"`
			SubscribeURL string `json:"SubscribeURL"`
		}
		if err := json.Unmarshal(body, &snsMessage); err == nil {
			if snsMessage.Type == "SubscriptionConfirmation" && snsMessage.SubscribeURL != "" {
				// Confirm SNS subscription
				resp, err := http.Get(snsMessage.SubscribeURL)
				if err == nil {
					resp.Body.Close()
				}
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		// Forward to Levee API
		ctx := r.Context()
		err = c.ForwardSESWebhook(ctx, body)
		if err != nil {
			http.Error(w, "Failed to process webhook", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// verifyStripeSignature verifies a Stripe webhook signature.
func verifyStripeSignature(payload []byte, signature, secret string) bool {
	if signature == "" {
		return false
	}

	// Parse signature header
	var timestamp, sig string
	for _, part := range strings.Split(signature, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			timestamp = kv[1]
		case "v1":
			sig = kv[1]
		}
	}

	if timestamp == "" || sig == "" {
		return false
	}

	// Compute expected signature
	signedPayload := timestamp + "." + string(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedPayload))
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(sig))
}

// Tracking API methods

// RecordOpen records an email open event.
func (c *Client) RecordOpen(ctx context.Context, token string) error {
	resp, err := c.doRequest(ctx, http.MethodPost, "/sdk/v1/tracking/open", map[string]string{
		"token": token,
	})
	if err != nil {
		return err
	}
	return decodeResponse(resp, nil)
}

// RecordClick records an email click event.
func (c *Client) RecordClick(ctx context.Context, token, url string) error {
	resp, err := c.doRequest(ctx, http.MethodPost, "/sdk/v1/tracking/click", map[string]string{
		"token": token,
		"url":   url,
	})
	if err != nil {
		return err
	}
	return decodeResponse(resp, nil)
}

// RecordUnsubscribe records an unsubscribe event.
func (c *Client) RecordUnsubscribe(ctx context.Context, token string) error {
	resp, err := c.doRequest(ctx, http.MethodPost, "/sdk/v1/tracking/unsubscribe", map[string]string{
		"token": token,
	})
	if err != nil {
		return err
	}
	return decodeResponse(resp, nil)
}

// ConfirmEmailResponse is the response from confirming an email.
type ConfirmEmailResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message,omitempty"`
	RedirectURL string `json:"redirect_url,omitempty"`
}

// ConfirmEmail confirms an email subscription (double opt-in).
func (c *Client) ConfirmEmail(ctx context.Context, token string) (*ConfirmEmailResponse, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/sdk/v1/tracking/confirm", map[string]string{
		"token": token,
	})
	if err != nil {
		return nil, err
	}

	var result ConfirmEmailResponse
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// ForwardStripeWebhook forwards a Stripe webhook payload to Levee.
func (c *Client) ForwardStripeWebhook(ctx context.Context, payload []byte, signature string) error {
	// Use webhookURL which strips /sdk/v1 from baseURL
	webhookURL := c.webhookBaseURL() + "/webhooks/stripe"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, strings.NewReader(string(payload)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Stripe-Signature", signature)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to forward webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook forward failed with status %d", resp.StatusCode)
	}

	return nil
}

// ForwardSESWebhook forwards an SES webhook payload to Levee.
func (c *Client) ForwardSESWebhook(ctx context.Context, payload []byte) error {
	// Use webhookURL which strips /sdk/v1 from baseURL
	webhookURL := c.webhookBaseURL() + "/webhooks/ses"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, strings.NewReader(string(payload)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to forward webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook forward failed with status %d", resp.StatusCode)
	}

	return nil
}

// webhookBaseURL returns the base URL for webhook endpoints (without /sdk/v1 suffix).
func (c *Client) webhookBaseURL() string {
	// Strip /sdk/v1 suffix if present
	base := c.baseURL
	if strings.HasSuffix(base, "/sdk/v1") {
		base = base[:len(base)-7]
	}
	return base
}
