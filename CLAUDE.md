# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Levee SDK for Go - Official SDK for integrating the Levee API (email marketing and SaaS customer platform) into Go applications.

- **Module**: `github.com/almatuck/levee-go`
- **Go Version**: 1.21+
- **API Base URL**: `https://levee.com/sdk/v1` (customizable)

## Commands

```bash
# Build/compile
go build ./...

# Run all tests (none currently exist)
go test ./...

# Run a single test
go test -run TestName ./...
```

No Makefile or build scripts - pure Go library consumed by parent projects.

## Architecture

Single-package library with Go files organized by domain:

| File | Purpose |
|------|---------|
| `client.go` | Core client, HTTP handling, functional options pattern |
| `handlers.go` | Embeddable HTTP handlers for white-label webhooks/tracking |
| `ws.go` | WebSocket handler for LLM chat streaming (gRPC-to-WebSocket bridge) |
| `llm.go` | LLM/AI chat client with gRPC streaming support |
| `llm.proto` | Protocol buffer definitions for LLM service |
| `llmpb/` | Generated protobuf Go code |
| `content.go` | CMS read-only endpoints (posts, pages, categories) |
| `site.go` | Site configuration (settings, menus, authors) |
| `contacts.go` | Contact CRUD, tags, activity, unsubscribe |
| `emails.go` | Transactional emails, delivery status/events |
| `sequences.go` | Drip campaign enrollment management |
| `billing.go` | Stripe customers, checkouts, subscriptions, metered usage |
| `customers.go` | Read-only customer billing history |
| `webhooks.go` | Webhook registration, testing, delivery logs |
| `tracking.go` | Custom event tracking |
| `stats.go` | Analytics (email, revenue, contact stats) |
| `lists.go` | Email list subscriptions |
| `orders.go` | Order creation with checkout URLs |

## Embedded HTTP Handlers

The SDK provides embeddable HTTP handlers for white-label integration. These allow email tracking pixels and webhooks to be served from the embedding application's domain.

```go
mux := http.NewServeMux()
client := levee.New(apiKey)
client.RegisterHandlers(mux, "/levee",
    levee.WithUnsubscribeRedirect("/unsubscribed"),
    levee.WithStripeWebhookSecret(stripeSecret),
)
```

**Mounted endpoints:**
| Path | Purpose |
|------|---------|
| `GET /levee/e/o/:token` | Email open tracking (serves 1x1 GIF) |
| `GET /levee/e/c/:token` | Click tracking (redirects to destination) |
| `GET /levee/e/u/:token` | One-click unsubscribe |
| `GET /levee/confirm-email` | Double opt-in confirmation |
| `POST /levee/webhooks/stripe` | Stripe webhook receiver |
| `POST /levee/webhooks/ses` | AWS SES bounce/complaint receiver |
| `GET /levee/ws/chat` | WebSocket LLM chat (requires `WithLLMClient()`) |

Handlers forward events to Levee API asynchronously (tracking) or synchronously (webhooks).

## LLM/AI Integration

The SDK includes a gRPC client for LLM chat with streaming support:

```go
llm := levee.NewLLMClient(apiKey, levee.WithGRPCAddress("llm.levee.com:9889"))
defer llm.Close()

// Streaming chat
session, _ := llm.NewChatSession(ctx, levee.ChatRequest{...})
session.Send(ctx, "Hello", func(chunk levee.StreamChunk) error {
    fmt.Print(chunk.Content)
    return nil
})
```

The WebSocket handler (`ws.go`) bridges browser WebSocket connections to gRPC streams for browser-based AI chat.

## Key Patterns

**Functional Options** for configuration:
```go
client := levee.New(apiKey, levee.WithBaseURL(url), levee.WithHTTPClient(httpClient))
```

**Context-first API** - all methods accept `context.Context` as first parameter:
```go
func (c *Client) CreateContact(ctx context.Context, contact Contact) (*ContactResponse, error)
```

**Request/Response structs** with JSON tags for each operation.

**Monetary values** stored as `int64` cents (divide by 100 for display).

## Code Conventions

- One function per operation - use parameters/options for variations, not separate functions
- All API methods are receivers on `*Client`
- Use `url.PathEscape()` for dynamic path segments
- Error wrapping with `fmt.Errorf` and `%w`
- JSON struct tags use snake_case with `omitempty` for optional fields

## Dependencies

- `google.golang.org/grpc` - gRPC client for LLM streaming
- `google.golang.org/protobuf` - Protocol buffer support
- `github.com/gorilla/websocket` - WebSocket support for browser LLM chat
