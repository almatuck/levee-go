# Levee SDK for Go

Official Go SDK for integrating [Levee](https://levee.sh) into your Go applications.

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Authentication](#authentication)
- [Embedded HTTP Handlers](#embedded-http-handlers)
- [LLM/AI Chat](#llmai-chat)
- [Content/CMS](#contentcms)
- [Site Configuration](#site-configuration)
- [Contacts](#contacts)
- [Email Lists](#email-lists)
- [Transactional Emails](#transactional-emails)
- [Email Sequences](#email-sequences)
- [Orders & Checkout](#orders--checkout)
- [Products](#products)
- [Funnels](#funnels)
- [Quizzes](#quizzes)
- [Offers](#offers)
- [Workshops](#workshops)
- [Customer Billing History](#customer-billing-history)
- [Event Tracking](#event-tracking)
- [Billing](#billing)
- [Webhooks](#webhooks)
- [Stats & Analytics](#stats--analytics)
- [Error Handling](#error-handling)
- [Complete Example](#complete-example)

## Installation

```bash
go get github.com/almatuck/levee-go
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    levee "github.com/almatuck/levee-go"
)

func main() {
    // Initialize the client with your API key
    client, err := levee.NewClient("lv_your_api_key_here")
    if err != nil {
        log.Fatal(err)
    }

    // Create a contact using the Contacts resource
    contact, err := client.Contacts.CreateContact(context.Background(), &levee.ContactRequest{
        Email: "user@example.com",
        Name:  "John Doe",
    })
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Contact created: %s", contact.ID)
}
```

## Configuration

### Basic Configuration

```go
// Default configuration - connects to https://levee.sh
client, err := levee.NewClient("lv_your_api_key")
if err != nil {
    log.Fatal(err)
}
```

### Custom Base URL

For self-hosted instances or development environments:

```go
client, err := levee.NewClient("lv_your_api_key",
    levee.WithBaseURL("https://your-domain.com"),
)
```

### Custom HTTP Client

For custom timeouts, proxies, or transport settings:

```go
import (
    "net/http"
    "time"
)

httpClient := &http.Client{
    Timeout: 60 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
    },
}

client, err := levee.NewClient("lv_your_api_key",
    levee.WithHTTPClient(httpClient),
)
```

### Timeout Configuration

```go
client, err := levee.NewClient("lv_your_api_key",
    levee.WithTimeout(60 * time.Second),
)
```

---

## Authentication

The Auth resource provides customer authentication for your application. This enables end-user login/registration flows for your SaaS application.

### Register a Customer

```go
resp, err := client.Auth.Register(ctx, &levee.SDKRegisterRequest{
    OrgSlug:  "your-org",
    Email:    "user@example.com",
    Password: "securepassword123",
    Name:     "John Doe",
})
if err != nil {
    log.Fatal(err)
}

log.Printf("Registered user: %s", resp.Customer.Email)
log.Printf("Access token: %s", resp.Token)
log.Printf("Refresh token: %s", resp.RefreshToken)
```

### Login

```go
resp, err := client.Auth.Login(ctx, &levee.SDKLoginRequest{
    OrgSlug:  "your-org",
    Email:    "user@example.com",
    Password: "securepassword123",
})
if err != nil {
    log.Fatal(err)
}

log.Printf("Logged in: %s", resp.Customer.Email)
log.Printf("Token expires at: %s", resp.ExpiresAt)
```

### Refresh Token

```go
resp, err := client.Auth.RefreshToken(ctx, &levee.SDKRefreshTokenRequest{
    RefreshToken: "existing_refresh_token",
})
if err != nil {
    log.Fatal(err)
}

log.Printf("New access token: %s", resp.Token)
log.Printf("New refresh token: %s", resp.RefreshToken)
```

### Forgot Password

```go
resp, err := client.Auth.ForgotPassword(ctx, &levee.SDKForgotPasswordRequest{
    OrgSlug: "your-org",
    Email:   "user@example.com",
})
if err != nil {
    log.Fatal(err)
}

log.Printf("Password reset email sent: %v", resp.Success)
```

### Reset Password

```go
resp, err := client.Auth.ResetPassword(ctx, &levee.SDKResetPasswordRequest{
    Token:           "reset_token_from_email",
    Password:        "newpassword123",
    ConfirmPassword: "newpassword123",
})
if err != nil {
    log.Fatal(err)
}

log.Printf("Password reset: %v", resp.Success)
```

### Verify Email

```go
resp, err := client.Auth.VerifyEmail(ctx, &levee.SDKVerifyEmailRequest{
    Token: "verification_token_from_email",
})
if err != nil {
    log.Fatal(err)
}

log.Printf("Email verified: %v", resp.Success)
```

### Change Password

Change a customer's password while logged in (requires current password verification).

```go
resp, err := client.Auth.ChangePassword(ctx, &levee.SDKChangePasswordRequest{
    OrgSlug:         "my-org",
    Email:           "user@example.com",
    CurrentPassword: "oldpassword123",
    NewPassword:     "newpassword456",
})
if err != nil {
    log.Fatal(err)
}

log.Printf("Password changed: %v", resp.Success)
```

---

## Embedded HTTP Handlers

The SDK provides embeddable HTTP handlers for white-label integration. This allows email tracking pixels, unsubscribe links, and webhooks to be served from your application's domain instead of Levee's domain.

### Register Handlers

```go
mux := http.NewServeMux()
client, _ := levee.NewClient("lv_your_api_key")

// Register all Levee handlers with a prefix
client.RegisterHandlers(mux, "/levee")
```

This registers the following endpoints on your server:

| Endpoint                      | Purpose                                          |
| ----------------------------- | ------------------------------------------------ |
| `GET /levee/e/o/:token`       | Email open tracking (serves 1x1 transparent GIF) |
| `GET /levee/e/c/:token`       | Click tracking (redirects to destination URL)    |
| `GET /levee/e/u/:token`       | One-click unsubscribe                            |
| `GET /levee/confirm-email`    | Double opt-in email confirmation                 |
| `POST /levee/webhooks/stripe` | Stripe webhook receiver                          |
| `POST /levee/webhooks/ses`    | AWS SES bounce/complaint receiver                |

### Configuration Options

```go
client.RegisterHandlers(mux, "/levee",
    // Redirect URL after unsubscribe (default: /unsubscribed)
    levee.WithUnsubscribeRedirect("/email-preferences"),

    // Redirect URL after email confirmation (default: /confirmed)
    levee.WithConfirmRedirect("/welcome"),

    // Redirect URL for expired confirmation tokens (default: /confirm-expired)
    levee.WithConfirmExpiredRedirect("/link-expired"),

    // Stripe webhook secret for signature verification
    levee.WithStripeWebhookSecret(os.Getenv("STRIPE_WEBHOOK_SECRET")),
)
```

### Complete Example

```go
package main

import (
    "log"
    "net/http"
    "os"

    levee "github.com/almatuck/levee-go"
)

func main() {
    mux := http.NewServeMux()
    client, err := levee.NewClient(os.Getenv("LEVEE_API_KEY"))
    if err != nil {
        log.Fatal(err)
    }

    // Register Levee handlers
    client.RegisterHandlers(mux, "/levee",
        levee.WithUnsubscribeRedirect("/unsubscribed"),
        levee.WithStripeWebhookSecret(os.Getenv("STRIPE_WEBHOOK_SECRET")),
    )

    // Your application routes
    mux.HandleFunc("/", homeHandler)
    mux.HandleFunc("/api/signup", signupHandler)

    log.Fatal(http.ListenAndServe(":8080", mux))
}
```

### Required Pages

Your application must provide these pages for redirects:

| Page                     | Purpose                                | Default Path       |
| ------------------------ | -------------------------------------- | ------------------ |
| Unsubscribe confirmation | Shown after user unsubscribes          | `/unsubscribed`    |
| Email confirmed          | Shown after double opt-in confirmation | `/confirmed`       |
| Link expired             | Shown when confirmation token expires  | `/confirm-expired` |

Override defaults with `WithUnsubscribeRedirect()`, `WithConfirmRedirect()`, and `WithConfirmExpiredRedirect()`.

### Email Configuration

When sending emails through Levee, configure your email templates to use your domain for tracking URLs:

```go
// When creating emails, Levee will use these URLs:
// Open tracking:  https://yourdomain.com/levee/e/o/{token}
// Click tracking: https://yourdomain.com/levee/e/c/{token}?url={destination}
// Unsubscribe:    https://yourdomain.com/levee/e/u/{token}
// Confirm email:  https://yourdomain.com/levee/confirm-email?token={token}
```

Set your tracking domain in Levee dashboard or via API to match your embedded handler prefix.

### Webhook Configuration

Configure your third-party services to send webhooks to your domain:

- **Stripe**: Set webhook URL to `https://yourdomain.com/levee/webhooks/stripe`
- **AWS SES**: Set SNS notification URL to `https://yourdomain.com/levee/webhooks/ses`

The handlers forward events to Levee API for processing while serving tracking pixels and handling redirects locally.

### How It Works

The embedded handlers make Levee completely invisible to your end users:

```
+----------------------------------------------------------------+
|  Your App (brandivize.com)                                     |
|                                                                |
|  +---------------+    +--------------------------------+       |
|  | Your Routes   |    | Levee SDK Handlers             |       |
|  | /             |    | /levee/e/o/:token -> serves GIF|       |
|  | /dashboard    |    | /levee/e/c/:token -> redirects |       |
|  | /api/*        |    | /levee/e/u/:token -> unsubscribe|      |
|  +---------------+    | /levee/webhooks/* -> forwards  |       |
|                       +--------------------------------+       |
+----------------------------------------------------------------+
                                    |
                                    v (async forwarding)
                         +---------------------+
                         |  Levee API          |
                         |  levee.sh/sdk/v1/* |
                         +---------------------+
```

- **Email opens**: GIF served locally, open recorded asynchronously
- **Link clicks**: Redirect happens locally, click recorded asynchronously
- **Unsubscribes**: Processed synchronously, then redirects to your page
- **Webhooks**: Verified locally (Stripe), then forwarded to Levee API

---

## LLM/AI Chat

The SDK provides access to Levee's LLM gateway for AI-powered chat capabilities. Supports both simple request/response and streaming via gRPC or WebSocket.

### Simple Chat (Non-Streaming)

```go
llm := levee.NewLLMClient("lv_your_api_key")
defer llm.Close()

resp, err := llm.Chat(ctx, levee.ChatRequest{
    Messages: []levee.ChatMessage{
        {Role: "user", Content: "What is the capital of France?"},
    },
    Model:       "sonnet", // "haiku", "sonnet", or "opus"
    MaxTokens:   1024,
    Temperature: 0.7,
})

log.Printf("Response: %s", resp.Content)
log.Printf("Tokens: %d in, %d out, Cost: $%.4f", resp.InputTokens, resp.OutputTokens, resp.CostUSD)
```

### Streaming Chat (gRPC)

```go
llm := levee.NewLLMClient("lv_your_api_key",
    levee.WithGRPCAddress("llm.levee.sh:9889"),
)
defer llm.Close()

// Create a streaming session
session, err := llm.NewChatSession(ctx, levee.ChatRequest{
    SystemPrompt: "You are a helpful assistant.",
    Model:        "sonnet",
    MaxTokens:    2048,
})
defer session.Close()

// Send message and stream response
resp, err := session.Send(ctx, "Tell me a story", func(chunk levee.StreamChunk) error {
    fmt.Print(chunk.Content) // Print each token as it arrives
    return nil
})

log.Printf("\nTotal tokens: %d", resp.OutputTokens)
```

### Convenience Streaming Method

```go
resp, err := llm.ChatStream(ctx, levee.ChatRequest{
    Messages: []levee.ChatMessage{
        {Role: "user", Content: "Explain quantum computing"},
    },
    Model:     "haiku",
    MaxTokens: 1024,
}, func(chunk levee.StreamChunk) error {
    fmt.Print(chunk.Content)
    return nil
})
```

### WebSocket Chat Handler (Embedded)

For browser-based streaming, the SDK provides an embeddable WebSocket handler:

```go
mux := http.NewServeMux()
client, _ := levee.NewClient("lv_your_api_key")
llm := levee.NewLLMClient("lv_your_api_key")

// Register handlers including WebSocket chat
client.RegisterHandlers(mux, "/levee",
    levee.WithLLMClient(llm),
    levee.WithWSCheckOrigin(func(r *http.Request) bool {
        return r.Host == "yourdomain.com" // Origin validation
    }),
)

// WebSocket endpoint available at: ws://yourdomain.com/levee/ws/chat
```

### WebSocket Protocol

The WebSocket chat uses JSON messages:

**Client Messages:**

```json
// Start session
{"type": "start", "data": {"system_prompt": "...", "model": "sonnet", "max_tokens": 1024}}

// Send message
{"type": "message", "data": {"content": "Hello!"}}

// Abort generation
{"type": "abort", "data": {"reason": "user cancelled"}}
```

**Server Messages:**

```json
// Session started
{"type": "started", "data": {"session_id": "...", "provider": "anthropic", "model": "claude-3-sonnet"}}

// Content chunk (streaming)
{"type": "chunk", "data": {"content": "Hello", "index": 0}}

// Completion
{"type": "completion", "data": {"full_content": "...", "stop_reason": "end_turn", "input_tokens": 10, "output_tokens": 50}}

// Error
{"type": "error", "data": {"code": "rate_limit", "message": "...", "retryable": true}}
```

---

## Content/CMS

Access published content for your static site or application.

### List Posts

```go
posts, err := client.Content.ListContentPosts(ctx, 1, 10, "tutorials") // page, pageSize, categorySlug

for _, post := range posts.Posts {
    log.Printf("%s: %s", post.Slug, post.Title)
}
log.Printf("Total posts: %d", posts.Total)
```

### Get a Post

```go
post, err := client.Content.GetContentPost(ctx, "getting-started-guide")

log.Printf("Title: %s", post.Title)
log.Printf("Content: %s", post.Content)
log.Printf("Category: %s", post.CategoryName)
log.Printf("Published: %s", post.PublishedAt)
```

### List Pages

```go
pages, err := client.Content.ListContentPages(ctx, 1, 20) // page, pageSize

for _, page := range pages.Pages {
    log.Printf("%s: %s (template: %s)", page.Slug, page.Title, page.TemplateName)
}
```

### Get a Page

```go
page, err := client.Content.GetContentPage(ctx, "about-us")

log.Printf("Title: %s", page.Title)
log.Printf("Content: %s", page.Content)
log.Printf("Meta Title: %s", page.MetaTitle)
```

### List Categories

```go
categories, err := client.Content.ListContentCategories(ctx)

for _, cat := range categories.Categories {
    log.Printf("%s: %s", cat.Slug, cat.Name)
}
```

---

## Site Configuration

Access site settings, navigation menus, and author information for building your frontend.

### Get Site Settings

```go
settings, err := client.Site.GetSiteSettings(ctx)

log.Printf("Site: %s - %s", settings.SiteName, settings.Tagline)
log.Printf("Logo: %s", settings.LogoUrl)
log.Printf("Contact: %s", settings.ContactEmail)
log.Printf("Social: %v", settings.SocialLinks) // map[string]string
log.Printf("Default Meta: %s", settings.MetaTitleTemplate)
```

### List Navigation Menus

```go
// Get all menus
menus, err := client.Site.ListNavigationMenus(ctx, "")

// Filter by location (header, footer, sidebar)
menus, err := client.Site.ListNavigationMenus(ctx, "header")

for _, menu := range menus.Menus {
    log.Printf("Menu: %s (%s)", menu.Name, menu.Location)
    for _, item := range menu.Items {
        log.Printf("  - %s: %s", item.Label, item.Url)
        for _, child := range item.Children {
            log.Printf("    - %s: %s", child.Label, child.Url)
        }
    }
}
```

### Get a Menu by Slug

```go
menu, err := client.Site.GetNavigationMenu(ctx, "main-nav")

for _, item := range menu.Items {
    log.Printf("%s -> %s", item.Label, item.Url)
}
```

### List Authors

```go
authors, err := client.Site.ListAuthors(ctx)

for _, author := range authors.Authors {
    log.Printf("%s: %s", author.DisplayName, author.Bio)
}
```

### Get Author by ID

```go
author, err := client.Site.GetAuthor(ctx, "author-123")

log.Printf("Name: %s", author.DisplayName)
log.Printf("Bio: %s", author.Bio)
log.Printf("Avatar: %s", author.AvatarUrl)
log.Printf("Twitter: @%s", author.TwitterHandle)
```

---

## Contacts

Contacts are the core of Levee. Create and manage contacts when users sign up, submit forms, or interact with your application.

### Create a Contact

```go
contact, err := client.Contacts.CreateContact(ctx, &levee.ContactRequest{
    Email: "user@example.com",
    Name:  "John Doe",
    Tags:  []string{"signup"},
})
```

### Get a Contact

```go
// Get by ID or email
contact, err := client.Contacts.GetContact(ctx, "user@example.com")

log.Printf("Contact: %s, Status: %s, Tags: %v", contact.Name, contact.Status, contact.Tags)
log.Printf("Emails sent: %d, opened: %d, clicked: %d", contact.EmailsSent, contact.EmailsOpened, contact.EmailsClicked)
```

### Update a Contact

```go
contact, err := client.Contacts.UpdateContact(ctx, "user@example.com", &levee.UpdateContactRequest{
    Name:    "John Smith",
    Phone:   "+1-555-123-4567",
    Company: "Acme Inc",
    CustomFields: map[string]string{
        "plan": "enterprise",
    },
})
```

### Manage Contact Tags

```go
// Add tags
_, err := client.Contacts.AddContactTags(ctx, "user@example.com", &levee.AddContactTagsRequest{
    Tags: []string{"vip", "enterprise"},
})

// Remove tags
_, err := client.Contacts.RemoveContactTags(ctx, "user@example.com", &levee.RemoveContactTagsRequest{
    Tags: []string{"trial"},
})
```

### View Contact Activity

```go
activities, err := client.Contacts.ListContactActivity(ctx, "user@example.com", 50)
for _, activity := range activities.Activities {
    log.Printf("%s: %s - %s", activity.Timestamp, activity.Event, activity.Details)
}
```

### Global Unsubscribe

```go
// Unsubscribe from all communications
_, err := client.Contacts.GlobalUnsubscribe(ctx, &levee.GlobalUnsubscribeRequest{
    Email:  "user@example.com",
    Reason: "User requested to stop all emails",
})
```

---

## Email Lists

Manage email list subscriptions for newsletters, updates, and marketing.

### Subscribe to a List

```go
_, err := client.Lists.SubscribeToList(ctx, "newsletter", &levee.SubscribeRequest{
    Email: "user@example.com",
    Name:  "John Doe",
})
```

### Unsubscribe from a List

```go
_, err := client.Lists.UnsubscribeFromList(ctx, "newsletter", &levee.SubscribeRequest{
    Email: "user@example.com",
})
```

---

## Transactional Emails

Send transactional emails for receipts, notifications, password resets, and more.

### Send an Email

```go
// Using a template
resp, err := client.Emails.SendEmail(ctx, &levee.SendEmailRequest{
    To:           "user@example.com",
    TemplateSlug: "welcome-email",
    Variables: map[string]string{
        "name":      "John",
        "login_url": "https://app.example.com/login",
    },
})

// Custom email
resp, err := client.Emails.SendEmail(ctx, &levee.SendEmailRequest{
    To:       "user@example.com",
    Subject:  "Your order has shipped!",
    Body:     "<h1>Order Shipped</h1><p>Your order #12345 is on its way.</p>",
    TextBody: "Order Shipped\n\nYour order #12345 is on its way.",
    FromName: "Acme Store",
    Tags:     []string{"shipping", "order-12345"},
})

log.Printf("Email sent, message ID: %s, status: %s", resp.MessageID, resp.Status)
```

### Check Email Status

```go
status, err := client.Emails.GetEmailStatus(ctx, messageID)

log.Printf("Email to %s: %s", status.To, status.Status)
log.Printf("Sent: %s, Delivered: %s, Opened: %s", status.SentAt, status.DeliveredAt, status.OpenedAt)
log.Printf("Opens: %d, Clicks: %d", status.Opens, status.Clicks)
```

### Get Email Events

```go
events, err := client.Emails.ListEmailEvents(ctx, messageID)
for _, event := range events.Events {
    log.Printf("%s: %s %s", event.Timestamp, event.Event, event.Data)
}
// Output:
// 2024-01-15T10:00:00Z: sent
// 2024-01-15T10:00:02Z: delivered
// 2024-01-15T10:15:30Z: opened
// 2024-01-15T10:16:45Z: clicked https://example.com/link
```

---

## Email Sequences

Enroll contacts in automated email sequences for onboarding, nurturing, and drip campaigns.

### Enroll in a Sequence

```go
resp, err := client.Sequences.EnrollInSequence(ctx, &levee.EnrollSequenceRequest{
    SequenceSlug: "onboarding",
    Email:        "user@example.com",
    Variables: map[string]string{
        "first_name": "John",
        "plan":       "Pro",
    },
})

log.Printf("Enrollment ID: %s, Status: %s", resp.EnrollmentID, resp.Status)
```

### Get Sequence Enrollments

```go
// Get all enrollments for an email
enrollments, err := client.Sequences.GetSequenceEnrollments(ctx, "user@example.com", "")

// Get specific sequence enrollment
enrollments, err := client.Sequences.GetSequenceEnrollments(ctx, "user@example.com", "onboarding")

for _, e := range enrollments.Enrollments {
    log.Printf("Sequence: %s, Step %d/%d, Status: %s",
        e.SequenceName, e.CurrentStep, e.TotalSteps, e.Status)
    log.Printf("Next email at: %s", e.NextEmailAt)
}
```

### Unenroll from Sequences

```go
// Unenroll from specific sequence
_, err := client.Sequences.UnenrollFromSequence(ctx, &levee.UnenrollSequenceRequest{
    Email:        "user@example.com",
    SequenceSlug: "onboarding",
})

// Unenroll from all sequences
_, err := client.Sequences.UnenrollFromSequence(ctx, &levee.UnenrollSequenceRequest{
    Email: "user@example.com",
})
```

### Pause and Resume

```go
// Pause enrollment
_, err := client.Sequences.PauseSequenceEnrollment(ctx, &levee.PauseSequenceRequest{
    Email:        "user@example.com",
    SequenceSlug: "onboarding",
})

// Resume enrollment
_, err := client.Sequences.ResumeSequenceEnrollment(ctx, &levee.ResumeSequenceRequest{
    Email:        "user@example.com",
    SequenceSlug: "onboarding",
})
```

---

## Orders & Checkout

Create checkout sessions for products, courses, or any purchasable items.

### Create an Order

```go
order, err := client.Orders.CreateOrder(ctx, &levee.OrderRequest{
    Email:       "user@example.com",
    ProductSlug: "pro-plan",
    SuccessUrl:  "https://yourapp.com/success",
    CancelUrl:   "https://yourapp.com/cancel",
})

// Redirect user to checkout
http.Redirect(w, r, order.CheckoutUrl, http.StatusSeeOther)
```

---

## Products

Access product information for your catalog.

### Get Product

```go
product, err := client.Products.GetProduct(ctx, "pro-plan")

log.Printf("Product: %s - %s", product.Name, product.Description)
log.Printf("Type: %s, Category: %s", product.Type, product.Category)
for _, price := range product.Prices {
    log.Printf("  Price: $%.2f/%s", float64(price.UnitAmountCents)/100, price.RecurringInterval)
}
```

---

## Funnels

Access funnel step information for multi-step sales processes.

### Get Funnel Step

```go
step, err := client.Funnels.GetFunnelStep(ctx, "onboarding-step-1")

log.Printf("Step: %s", step.Title)
log.Printf("Type: %s", step.StepType)
log.Printf("Next step ID: %d", step.NextStepID)
```

---

## Quizzes

Access and submit quizzes for lead qualification or assessments.

### Get Quiz

```go
quiz, err := client.Quizzes.GetQuiz(ctx, "product-fit")

log.Printf("Quiz: %s", quiz.Title)
for _, q := range quiz.Questions {
    log.Printf("  Q: %s", q.Question)
}
```

### Submit Quiz

```go
result, err := client.Quizzes.SubmitQuiz(ctx, "product-fit", &levee.QuizSubmitRequest{
    Email: "user@example.com",
    Answers: map[string]string{
        "q1": "answer1",
        "q2": "answer2",
    },
})

log.Printf("Segments: %v", result.Segments)
log.Printf("Redirect: %s", result.RedirectUrl)
```

---

## Offers

Process special offers and promotions.

### Process Offer

```go
result, err := client.Offers.ProcessOffer(ctx, &levee.OfferRequest{
    SessionID: "checkout_session_id",
    StepSlug:  "upsell-1",
    Accept:    true,
})

log.Printf("Success: %v", result.Success)
log.Printf("Next URL: %s", result.NextUrl)
```

---

## Workshops

Access workshop and event information.

### Get Workshop

```go
workshop, err := client.Workshops.GetWorkshop(ctx, "intro-webinar")

log.Printf("Workshop: %s", workshop.Title)
log.Printf("Date: %s to %s", workshop.StartDate, workshop.EndDate)
log.Printf("Seats remaining: %d", workshop.SeatsRemaining)
```

### Get Workshop by Product

```go
workshop, err := client.Workshops.GetWorkshopByProduct(ctx, "webinar-product")

log.Printf("Workshop for product: %s", workshop.Title)
```

---

## Customer Billing History

Access customer billing data including invoices, orders, subscriptions, and payments. Perfect for building customer account pages.

### Get Customer Info

```go
customer, err := client.Customers.GetCustomerByEmail(ctx, "user@example.com")

log.Printf("Customer: %s", customer.Name)
log.Printf("Total spent: $%.2f", float64(customer.TotalSpent)/100)
log.Printf("Orders: %d, Subscriptions: %d", customer.OrderCount, customer.SubscriptionCount)
```

### List Invoices

```go
invoices, err := client.Customers.ListCustomerInvoices(ctx, "user@example.com", 10)
for _, inv := range invoices.Invoices {
    log.Printf("Invoice #%s: $%.2f %s", inv.Number, float64(inv.AmountPaid)/100, inv.Status)
    if inv.InvoicePdfUrl != "" {
        log.Printf("  PDF: %s", inv.InvoicePdfUrl)
    }
}
```

### List Orders

```go
orders, err := client.Customers.ListCustomerOrders(ctx, "user@example.com", 10)
for _, order := range orders.Orders {
    log.Printf("Order %s: $%.2f %s", order.OrderNumber, float64(order.TotalCents)/100, order.Status)
    for _, item := range order.Items {
        log.Printf("  - %s x%d: $%.2f", item.ProductName, item.Quantity, float64(item.TotalPrice)/100)
    }
}
```

### List Subscriptions

```go
subs, err := client.Customers.ListCustomerSubscriptions(ctx, "user@example.com")
for _, sub := range subs.Subscriptions {
    log.Printf("Subscription: %s - %s ($%.2f/%s)",
        sub.ProductName, sub.Status, float64(sub.AmountCents)/100, sub.Interval)
    log.Printf("  Current period: %s to %s", sub.CurrentPeriodStart, sub.CurrentPeriodEnd)
}
```

### List Payments

```go
payments, err := client.Customers.ListCustomerPayments(ctx, "user@example.com", 20)
for _, p := range payments.Payments {
    log.Printf("Payment: $%.2f %s via %s", float64(p.AmountCents)/100, p.Status, p.PaymentMethod)
    if p.ReceiptUrl != "" {
        log.Printf("  Receipt: %s", p.ReceiptUrl)
    }
}
```

### Update Customer

Update a customer's profile information (name, phone, avatar, status, metadata).

```go
customer, err := client.Customers.UpdateCustomer(ctx, "customer-uuid", &levee.SDKUpdateCustomerRequest{
    Name:      "John Doe",
    Phone:     "+1-555-1234",
    AvatarUrl: "https://example.com/avatar.jpg",
    Status:    "active",
    Metadata:  `{"plan": "pro", "source": "website"}`,
})
if err != nil {
    log.Fatal(err)
}

log.Printf("Updated customer: %s (%s)", customer.Name, customer.Email)
```

### Delete Customer

Permanently delete a customer (GDPR compliance - hard delete).

```go
resp, err := client.Customers.DeleteCustomer(ctx, "customer-uuid")
if err != nil {
    log.Fatal(err)
}

log.Printf("Customer deleted: %v", resp.Success)
```

---

## Event Tracking

Track custom events for analytics, automation triggers, and user behavior analysis.

### Track Events

```go
_, err := client.Events.TrackEvent(ctx, &levee.EventRequest{
    Event: "purchase_completed",
    Email: "user@example.com",
    Properties: map[string]interface{}{
        "product":  "pro-plan",
        "amount":   "99.00",
        "currency": "usd",
    },
})
```

---

## Billing

Full Stripe billing integration for customers, subscriptions, and usage-based billing.

### Create a Customer

```go
customer, err := client.Billing.CreateCustomer(ctx, &levee.CustomerRequest{
    Email: "user@example.com",
    Name:  "John Doe",
})
```

### Create a Checkout Session

```go
checkout, err := client.Billing.CreateCheckoutSession(ctx, &levee.CheckoutRequest{
    CustomerEmail: "user@example.com",
    LineItems: []levee.CheckoutItem{
        {PriceID: "price_xxx", Quantity: 1},
    },
    Mode:       "subscription",
    SuccessUrl: "https://yourapp.com/success",
    CancelUrl:  "https://yourapp.com/cancel",
})
// Redirect to checkout.CheckoutUrl
```

### Create a Subscription

```go
sub, err := client.Billing.CreateSubscription(ctx, &levee.SubscriptionRequest{
    CustomerID: "cust_123",
    PriceIds:   []string{"price_xxx"},
})
```

### Cancel a Subscription

```go
_, err := client.Billing.CancelSubscription(ctx, "sub_123")
```

### Record Metered Usage

```go
_, err := client.Billing.RecordUsage(ctx, &levee.UsageRequest{
    SubscriptionItemID: "si_xxx",
    Quantity:           150,
})
```

### Customer Portal

```go
portal, err := client.Billing.GetCustomerPortal(ctx, &levee.PortalRequest{
    CustomerID: "cust_123",
    ReturnUrl:  "https://yourapp.com/settings",
})
// Redirect to portal.PortalUrl
```

---

## Webhooks

Register webhook endpoints to receive real-time events from Levee.

### Register a Webhook

```go
resp, err := client.Webhooks.RegisterWebhook(ctx, &levee.RegisterWebhookRequest{
    Url: "https://yourapp.com/webhooks/levee",
    Events: []string{
        "contact.created",
        "email.opened",
        "payment.succeeded",
        "subscription.created",
    },
})

log.Printf("Webhook ID: %s", resp.WebhookID)
log.Printf("Secret: %s (save this for signature verification!)", resp.Secret)
```

### Available Webhook Events

```
// Contact events
contact.created
contact.updated
contact.unsubscribed
contact.bounced

// Email events
email.sent
email.delivered
email.opened
email.clicked
email.bounced
email.complained

// Sequence events
sequence.enrolled
sequence.completed
sequence.paused
sequence.resumed

// Payment events
payment.succeeded
payment.failed
payment.refunded

// Subscription events
subscription.created
subscription.updated
subscription.cancelled
subscription.renewed

// Order events
order.created
order.completed
order.refunded
```

### List Webhooks

```go
webhooks, err := client.Webhooks.ListWebhooks(ctx)
for _, wh := range webhooks.Webhooks {
    log.Printf("Webhook %s: %s", wh.ID, wh.Url)
    log.Printf("  Events: %v", wh.Events)
    log.Printf("  Success rate: %d/%d", wh.DeliveriesSuccess, wh.DeliveriesTotal)
}
```

### Get a Webhook

```go
wh, err := client.Webhooks.GetWebhook(ctx, webhookID)
log.Printf("Webhook: %s -> %s", wh.ID, wh.Url)
```

### Update a Webhook

```go
wh, err := client.Webhooks.UpdateWebhook(ctx, webhookID, &levee.UpdateWebhookRequest{
    Events: []string{
        "payment.succeeded",
        "payment.failed",
    },
    Active: true,
})
```

### Test a Webhook

```go
result, err := client.Webhooks.TestWebhook(ctx, webhookID)
if result.Success {
    log.Printf("Webhook test succeeded! Status: %d", result.StatusCode)
} else {
    log.Printf("Webhook test failed: %s", result.Error)
}
```

### View Webhook Logs

```go
logs, err := client.Webhooks.ListWebhookLogs(ctx, webhookID, 20)
for _, l := range logs.Logs {
    log.Printf("%s: %s (status %d, %dms)", l.DeliveredAt, l.Event, l.StatusCode, l.Duration)
}
```

### Delete a Webhook

```go
_, err := client.Webhooks.DeleteWebhook(ctx, webhookID)
```

---

## Stats & Analytics

Access statistics and analytics for your organization.

### Overview Stats

```go
stats, err := client.Stats.GetStatsOverview(ctx,
    "2024-01-01T00:00:00Z", // startDate
    "2024-01-31T23:59:59Z", // endDate
)

log.Printf("Contacts: %d total, %d new, %d active", stats.TotalContacts, stats.NewContacts, stats.ActiveContacts)
log.Printf("Emails: %d sent, %.1f%% open rate, %.1f%% click rate", stats.EmailsSent, stats.OpenRate, stats.ClickRate)
log.Printf("Revenue: $%.2f from %d orders", float64(stats.TotalRevenue)/100, stats.OrderCount)
```

### Email Stats

```go
emailStats, err := client.Stats.GetEmailStats(ctx,
    "2024-01-01T00:00:00Z", // startDate
    "2024-01-31T23:59:59Z", // endDate
    "day",                   // groupBy: day, week, or month
)

log.Printf("Totals: %d sent, %.1f%% open rate, %.1f%% click rate",
    emailStats.TotalSent, emailStats.AvgOpenRate, emailStats.AvgClickRate)

for _, day := range emailStats.Stats {
    log.Printf("%s: %d sent, %d opened (%.1f%%)", day.Date, day.Sent, day.Opened, day.OpenRate)
}
```

### Revenue Stats

```go
revenueStats, err := client.Stats.GetRevenueStats(ctx,
    "2024-01-01T00:00:00Z", // startDate
    "2024-01-31T23:59:59Z", // endDate
    "week",                  // groupBy
)

log.Printf("Total revenue: $%.2f", float64(revenueStats.TotalRevenue)/100)
log.Printf("MRR: $%.2f", float64(revenueStats.Mrr)/100)
log.Printf("Orders: %d, Subscriptions: %d, Churned: %d",
    revenueStats.TotalOrders, revenueStats.TotalSubscriptions, revenueStats.TotalChurned)
```

### Contact Stats

```go
contactStats, err := client.Stats.GetContactStats(ctx,
    "2024-01-01T00:00:00Z", // startDate
    "2024-01-31T23:59:59Z", // endDate
    "day",                   // groupBy
)

log.Printf("Active: %d, Unsubscribed: %d, Net growth: %d",
    contactStats.TotalActive, contactStats.TotalUnsubscribed, contactStats.NetGrowth)
```

---

## Error Handling

The SDK returns errors for API failures, network issues, and validation problems.

```go
contact, err := client.Contacts.CreateContact(ctx, &levee.ContactRequest{
    Email: "user@example.com",
})
if err != nil {
    // Error format: "API error (status 400): {\"error\": \"email is required\"}"
    log.Printf("Levee API error: %v", err)
    return
}
```

---

## Complete Example

A complete SaaS application integration:

```go
package main

import (
    "context"
    "encoding/json"
    "log"
    "net/http"
    "os"

    levee "github.com/almatuck/levee-go"
)

var client *levee.Client

func init() {
    var err error
    client, err = levee.NewClient(os.Getenv("LEVEE_API_KEY"))
    if err != nil {
        log.Fatal(err)
    }
}

func handleSignup(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Email string `json:"email"`
        Name  string `json:"name"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    ctx := r.Context()

    // Create contact and enroll in onboarding sequence
    contact, _ := client.Contacts.CreateContact(ctx, &levee.ContactRequest{
        Email:      req.Email,
        Name:       req.Name,
        FunnelSlug: "signup",
        Tags:       []string{"trial"},
    })

    // Enroll in onboarding sequence
    client.Sequences.EnrollInSequence(ctx, &levee.EnrollSequenceRequest{
        SequenceSlug: "onboarding",
        Email:        req.Email,
        Variables: map[string]string{
            "first_name": req.Name,
        },
    })

    // Send welcome email
    client.Emails.SendEmail(ctx, &levee.SendEmailRequest{
        To:           req.Email,
        TemplateSlug: "welcome",
        Variables: map[string]string{
            "name": req.Name,
        },
    })

    json.NewEncoder(w).Encode(map[string]string{
        "contact_id": contact.ID,
    })
}

func main() {
    http.HandleFunc("/api/signup", handleSignup)
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

---

## API Reference

All methods use the resource-based pattern: `client.Resource.Method(ctx, ...)`

| Resource.Method                                                   | Description                                    |
| ----------------------------------------------------------------- | ---------------------------------------------- |
| **Client**                                                        |                                                |
| `NewClient(apiKey, opts...)`                                      | Create a new client (returns `*Client, error`) |
| `WithBaseURL(url)`                                                | Set custom API base URL                        |
| `WithHTTPClient(client)`                                          | Set custom HTTP client                         |
| `WithTimeout(duration)`                                           | Set HTTP request timeout                       |
| **Auth**                                                          |                                                |
| `Auth.Register(ctx, *SDKRegisterRequest)`                         | Register a new customer account                |
| `Auth.Login(ctx, *SDKLoginRequest)`                               | Authenticate and get tokens                    |
| `Auth.RefreshToken(ctx, *SDKRefreshTokenRequest)`                 | Exchange refresh token for new tokens          |
| `Auth.ForgotPassword(ctx, *SDKForgotPasswordRequest)`             | Initiate password reset                        |
| `Auth.ResetPassword(ctx, *SDKResetPasswordRequest)`               | Complete password reset                        |
| `Auth.VerifyEmail(ctx, *SDKVerifyEmailRequest)`                   | Verify email address                           |
| `Auth.ChangePassword(ctx, *SDKChangePasswordRequest)`             | Change password while logged in                |
| **Billing**                                                       |                                                |
| `Billing.CreateCustomer(ctx, *CustomerRequest)`                   | Create billing customer                        |
| `Billing.CreateCheckoutSession(ctx, *CheckoutRequest)`            | Create Stripe checkout                         |
| `Billing.CreateSubscription(ctx, *SubscriptionRequest)`           | Create subscription                            |
| `Billing.CancelSubscription(ctx, subscriptionID)`                 | Cancel subscription                            |
| `Billing.RecordUsage(ctx, *UsageRequest)`                         | Record metered usage                           |
| `Billing.GetCustomerPortal(ctx, *PortalRequest)`                  | Get portal URL                                 |
| **Contacts**                                                      |                                                |
| `Contacts.CreateContact(ctx, *ContactRequest)`                    | Create or get a contact                        |
| `Contacts.GetContact(ctx, idOrEmail)`                             | Get contact details                            |
| `Contacts.UpdateContact(ctx, id, *UpdateContactRequest)`          | Update a contact                               |
| `Contacts.AddContactTags(ctx, id, *AddContactTagsRequest)`        | Add tags to contact                            |
| `Contacts.RemoveContactTags(ctx, id, *RemoveContactTagsRequest)`  | Remove tags from contact                       |
| `Contacts.ListContactActivity(ctx, id, limit)`                    | Get contact activity                           |
| `Contacts.GlobalUnsubscribe(ctx, *GlobalUnsubscribeRequest)`      | Unsubscribe from all                           |
| **Content**                                                       |                                                |
| `Content.ListContentPosts(ctx, page, pageSize, categorySlug)`     | List published posts                           |
| `Content.GetContentPost(ctx, slug)`                               | Get post by slug                               |
| `Content.ListContentPages(ctx, page, pageSize)`                   | List published pages                           |
| `Content.GetContentPage(ctx, slug)`                               | Get page by slug                               |
| `Content.ListContentCategories(ctx)`                              | List content categories                        |
| **Customers**                                                     |                                                |
| `Customers.GetCustomerByEmail(ctx, email)`                        | Get customer info                              |
| `Customers.ListCustomerInvoices(ctx, email, limit)`               | List invoices                                  |
| `Customers.ListCustomerOrders(ctx, email, limit)`                 | List orders                                    |
| `Customers.ListCustomerSubscriptions(ctx, email)`                 | List subscriptions                             |
| `Customers.ListCustomerPayments(ctx, email, limit)`               | List payments                                  |
| `Customers.UpdateCustomer(ctx, id, *SDKUpdateCustomerRequest)`    | Update customer profile                        |
| `Customers.DeleteCustomer(ctx, id)`                               | Delete customer (GDPR)                         |
| **Emails**                                                        |                                                |
| `Emails.SendEmail(ctx, *SendEmailRequest)`                        | Send transactional email                       |
| `Emails.GetEmailStatus(ctx, messageID)`                           | Get email delivery status                      |
| `Emails.ListEmailEvents(ctx, messageID)`                          | Get email tracking events                      |
| **Events**                                                        |                                                |
| `Events.TrackEvent(ctx, *EventRequest)`                           | Track custom event                             |
| **Funnels**                                                       |                                                |
| `Funnels.GetFunnelStep(ctx, slug)`                                | Get funnel step info                           |
| **Lists**                                                         |                                                |
| `Lists.SubscribeToList(ctx, slug, *SubscribeRequest)`             | Subscribe to list                              |
| `Lists.UnsubscribeFromList(ctx, slug, *SubscribeRequest)`         | Unsubscribe from list                          |
| **Llm**                                                           |                                                |
| `Llm.Chat(ctx, *LLMChatRequest)`                                  | Simple chat via HTTP                           |
| `Llm.Config(ctx)`                                                 | Get LLM configuration                          |
| **Offers**                                                        |                                                |
| `Offers.ProcessOffer(ctx, *OfferRequest)`                         | Process an offer                               |
| **Orders**                                                        |                                                |
| `Orders.CreateOrder(ctx, *OrderRequest)`                          | Create checkout session                        |
| **Products**                                                      |                                                |
| `Products.GetProduct(ctx, slug)`                                  | Get product by slug                            |
| **Quizzes**                                                       |                                                |
| `Quizzes.GetQuiz(ctx, slug)`                                      | Get quiz by slug                               |
| `Quizzes.SubmitQuiz(ctx, slug, *QuizSubmitRequest)`               | Submit quiz answers                            |
| **Sequences**                                                     |                                                |
| `Sequences.EnrollInSequence(ctx, *EnrollSequenceRequest)`         | Enroll in sequence                             |
| `Sequences.GetSequenceEnrollments(ctx, email, sequenceSlug)`      | Get enrollments                                |
| `Sequences.UnenrollFromSequence(ctx, *UnenrollSequenceRequest)`   | Unenroll from sequence                         |
| `Sequences.PauseSequenceEnrollment(ctx, *PauseSequenceRequest)`   | Pause enrollment                               |
| `Sequences.ResumeSequenceEnrollment(ctx, *ResumeSequenceRequest)` | Resume enrollment                              |
| **Site**                                                          |                                                |
| `Site.GetSiteSettings(ctx)`                                       | Get site branding/settings                     |
| `Site.ListNavigationMenus(ctx, location)`                         | List navigation menus                          |
| `Site.GetNavigationMenu(ctx, slug)`                               | Get menu by slug                               |
| `Site.ListAuthors(ctx)`                                           | List all authors                               |
| `Site.GetAuthor(ctx, id)`                                         | Get author by ID                               |
| **Stats**                                                         |                                                |
| `Stats.GetStatsOverview(ctx, startDate, endDate)`                 | Get overview stats                             |
| `Stats.GetEmailStats(ctx, startDate, endDate, groupBy)`           | Get email stats                                |
| `Stats.GetRevenueStats(ctx, startDate, endDate, groupBy)`         | Get revenue stats                              |
| `Stats.GetContactStats(ctx, startDate, endDate, groupBy)`         | Get contact stats                              |
| **Tracking**                                                      |                                                |
| `Tracking.TrackOpen(ctx, *TrackOpenRequest)`                      | Track email open                               |
| `Tracking.TrackClick(ctx, *TrackClickRequest)`                    | Track link click                               |
| `Tracking.TrackUnsubscribe(ctx, *TrackUnsubscribeRequest)`        | Track unsubscribe                              |
| `Tracking.TrackConfirm(ctx, *TrackConfirmRequest)`                | Track email confirmation                       |
| **Webhooks**                                                      |                                                |
| `Webhooks.RegisterWebhook(ctx, *RegisterWebhookRequest)`          | Register webhook                               |
| `Webhooks.ListWebhooks(ctx)`                                      | List webhooks                                  |
| `Webhooks.GetWebhook(ctx, webhookID)`                             | Get webhook details                            |
| `Webhooks.UpdateWebhook(ctx, id, *UpdateWebhookRequest)`          | Update webhook                                 |
| `Webhooks.DeleteWebhook(ctx, webhookID)`                          | Delete webhook                                 |
| `Webhooks.TestWebhook(ctx, webhookID)`                            | Send test event                                |
| `Webhooks.ListWebhookLogs(ctx, webhookID, limit)`                 | Get delivery logs                              |
| **Workshops**                                                     |                                                |
| `Workshops.GetWorkshop(ctx, slug)`                                | Get workshop by slug                           |
| `Workshops.GetWorkshopByProduct(ctx, productSlug)`                | Get workshop by product                        |
| **Embedded Handlers**                                             |                                                |
| `RegisterHandlers(mux, prefix, opts...)`                          | Register HTTP handlers on mux                  |
| `WithUnsubscribeRedirect(url)`                                    | Set unsubscribe redirect URL                   |
| `WithConfirmRedirect(url)`                                        | Set confirmation redirect URL                  |
| `WithConfirmExpiredRedirect(url)`                                 | Set expired token redirect URL                 |
| `WithStripeWebhookSecret(secret)`                                 | Set Stripe webhook secret                      |
| `WithLLMClient(llm)`                                              | Enable WebSocket chat handler                  |
| `WithWSCheckOrigin(fn)`                                           | Set WebSocket origin checker                   |
| **LLM Client (gRPC)**                                             |                                                |
| `NewLLMClient(apiKey, opts...)`                                   | Create LLM client for streaming                |
| `WithGRPCAddress(addr)`                                           | Set gRPC server address                        |
| `Chat(ctx, ChatRequest)`                                          | Simple chat (non-streaming)                    |
| `NewChatSession(ctx, ChatRequest)`                                | Start streaming session                        |
| `ChatStream(ctx, ChatRequest, callback)`                          | Convenience streaming method                   |

---

## License

MIT License - see [LICENSE](LICENSE) for details.
