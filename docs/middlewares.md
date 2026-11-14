---
layout: default
title: Middlewares & Context Lifecycle
nav_order: 5
---

# Middlewares & Context Lifecycle

Middlewares in CellsWeb RPC Go intercept incoming requests before function business handlers run. They perform authentication, role verification, rate limiting, logging, and dependency injection.

---

## 🔑 Crucial Architectural Rule: Context Namespacing

Every middleware that produces output data **MUST namespace its output under its own registered middleware name**:

```go
ctx.Set("auth", AuthOutput{
    User: authenticatedUser,
})
```

Inside any downstream function or middleware:

```go
authVal, ok := ctx.Get("auth")
// or typed:
authData, ok := rpc.GetContext[AuthOutput](ctx, "auth")
```

### Why Namespacing Matters
When invoking Go RPC functions from **Node.js** or **Next.js Server Actions** via IPC with `skipMiddlewares: true`, Node passes:

```json
{
  "skipMiddlewares": true,
  "context": {
    "auth": {
      "user": { "id": "usr_101", "role": "admin" }
    }
  }
}
```

Go automatically unpacks this into `ctx.Set("auth", ...)`. Consequently, your Go functions run **identically** whether the context was established by the Go HTTP middleware or injected by a Node.js session!

---

## 📁 Middleware Directory Structure

Middlewares live in `middlewares/{name}/`:

```text
middlewares/auth/
├── constants.go # Options & Headers schemas
└── run.go       # Middleware handler & init() registration
```

---

## 1. Defining Middleware Schemas (`constants.go`)

`constants.go` defines:
- **Options Schema**: Configurable per function attachment.
- **Headers Schema**: The HTTP headers inspected by this middleware.

```go
package auth

import "github.com/CellsWeb/crpc-go/pkg/schema"

type Options struct {
	Roles    []string `json:"roles,omitempty"`
	Optional bool     `json:"optional,omitempty"`
}

var OptionsSchema = schema.Object(map[string]*schema.SchemaRule{
	"roles":    schema.Optional(schema.Array(schema.String())).Describe("Allowed roles"),
	"optional": schema.Optional(schema.Boolean()).Describe("Allow unauthenticated calls"),
})

var HeadersSchema = schema.Object(map[string]*schema.SchemaRule{
	"authorization": schema.Optional(schema.String()).Describe("Bearer JWT token"),
	"x-api-key":     schema.Optional(schema.String()).Describe("Alternative API Key"),
})
```

---

## 2. Implementing Middleware Logic (`run.go`)

```go
package auth

import (
	"strings"
	"github.com/CellsWeb/crpc-go/pkg/rpc"
)

type UserSession struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

func init() {
	rpc.RegisterMiddleware(
		"auth",
		"Validates bearer tokens and injects authenticated user into context",
		Run,
		OptionsSchema,
		HeadersSchema,
	)
}

func Run(ctx *rpc.Context, headers map[string]string, options map[string]any) error {
	token := headers["authorization"]

	if token == "" {
		// Check if function configured optional authentication
		if opt, ok := options["optional"].(bool); ok && opt {
			return nil
		}
		return rpc.NewError(rpc.ErrUnauthorized, "Missing Authorization header")
	}

	cleanToken := strings.TrimPrefix(token, "Bearer ")
	if cleanToken != "secret_token_123" {
		return rpc.NewError(rpc.ErrUnauthorized, "Invalid or expired token")
	}

	// 🔒 Store validated user strictly under middleware name "auth"
	ctx.Set("auth", map[string]any{
		"user": UserSession{
			ID:    "usr_42",
			Email: "alex@cellsweb.com",
			Role:  "admin",
		},
	})

	return nil
}
```

---

## 🛡️ Fail-Closed Security

CellsWeb RPC implements **fail-closed** security:
- If a function specifies `rpc.M("missingMiddleware", ...)`, the server **aborts immediately** with `internal_error`. It will **never** silently execute an unauthenticated or unprotected handler.
- Middleware errors immediately abort the request and return standardized error envelopes (HTTP 200).
