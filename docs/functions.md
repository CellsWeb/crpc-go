---
layout: default
title: Writing Functions & Contracts
nav_order: 4
---

# Writing Functions & Contracts

In CellsWeb RPC Go, every function is organized as an isolated, self-contained package inside `functions/{namespace}/{functionName}/`.

---

## 📁 The Co-Located Triad Structure

Each function directory contains three canonical files:

```text
functions/user/getProfile/
├── contracts.go   # Types, Schemas & API Metadata (Single source of truth)
├── middlewares.go # Attached middleware chain & options
└── run.go         # Registration hook (init) & execution logic (Run)
```

---

## 1. Defining Contracts (`contracts.go`)

`contracts.go` defines the public contract: the target namespace, function name, content type, documentation metadata, and the typed `Input` and `Output` structs.

```go
package getprofile

import (
	"github.com/CellsWeb/crpc-go/pkg/rpc"
	"github.com/CellsWeb/crpc-go/pkg/schema"
)

const (
	Namespace    = "user"
	FunctionName = "getProfile"
	ContentType  = "application/json"
)

var Metadata = rpc.FunctionMetadata{
	Description: "Retrieves complete user profile and preferences",
	IsPublic:    false,
	Tags:        []string{"user", "profile"},
}

// Fluent TypeBox/Zod-like schema definition:
var InputSchema = schema.Object(map[string]*schema.SchemaRule{
	"id": schema.String().Min(1).Describe("Target user identifier"),
})

var OutputSchema = schema.Object(map[string]*schema.SchemaRule{
	"id":        schema.String(),
	"email":     schema.String().Email(),
	"name":      schema.String(),
	"role":      schema.String(),
	"createdAt": schema.Integer(),
})

// Go structs for handler type safety:
type Input struct {
	ID string `json:"id"`
}

type Output struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	CreatedAt int64  `json:"createdAt"`
}

// NOTE: You can also auto-infer schemas directly from plain Go structs:
// var InputSchema = schema.FromStruct(Input{})
```

### Struct Tags for Automatic Documentation
- `json:"id"`: Maps JSON payload field name. Append `,omitempty` to mark the field as **optional**.
- `doc:"Description"`: Supplies the human-readable description rendered on the left column of `/docs`.

---

## 2. Attaching Middlewares (`middlewares.go`)

`middlewares.go` configures the middleware pipeline for this specific function.

```go
package getprofile

import "github.com/CellsWeb/crpc-go/pkg/rpc"

var Middlewares = rpc.DefineMiddlewares(
	rpc.M("auth", map[string]any{
		"roles": []string{"admin", "user"},
	}),
	rpc.M("rateLimit", map[string]any{
		"windowSeconds": 60,
		"maxRequests":   100,
	}),
)
```

---

## 3. Implementing Business Logic (`run.go`)

`run.go` contains the registration hook (`init()`) and the typed `Run` handler function.

```go
package getprofile

import (
	"github.com/CellsWeb/crpc-go/pkg/rpc"
)

func init() {
	rpc.Register(rpc.FunctionConfig[Input, Output]{
		Namespace:    Namespace,
		FunctionName: FunctionName,
		ContentType:  ContentType,
		Metadata:     Metadata,
		Middlewares:  Middlewares,
		Handler:      Run,
	})
}

func Run(ctx *rpc.Context, input Input) (*Output, error) {
	// 1. Validate inputs or read from dependency injection context
	if input.ID == "" {
		return nil, rpc.NewError(rpc.ErrValidation, "User ID is required")
	}

	// 2. Fetch authenticated context injected by 'auth' middleware
	authVal, ok := ctx.Get("auth")
	if !ok {
		return nil, rpc.NewError(rpc.ErrUnauthorized, "Authentication context missing")
	}

	// 3. Return strictly-typed output
	return &Output{
		ID:        input.ID,
		Email:     "user@example.com",
		Name:      "Jane Doe",
		Role:      "user",
		CreatedAt: 1715690000,
	}, nil
}
```

---

## 📤 Multipart File Uploads

For file uploads (images, PDFs, documents), set `ContentType = "multipart/form-data"` and use `*rpc.UploadedFile` in your `Input` struct:

### `contracts.go`
```go
package uploadavatar

import "github.com/CellsWeb/crpc-go/pkg/rpc"

const (
	Namespace    = "user"
	FunctionName = "uploadAvatar"
	ContentType  = "multipart/form-data"
)

type Input struct {
	UserID string            `json:"userId" doc:"User identifier"`
	Avatar *rpc.UploadedFile `json:"avatar" doc:"Binary avatar image file"`
}

type Output struct {
	AvatarURL string `json:"avatarUrl"`
}
```

### `run.go`
```go
package uploadavatar

import (
	"fmt"
	"path/filepath"
	"github.com/CellsWeb/crpc-go/pkg/rpc"
)

func Run(ctx *rpc.Context, input Input) (*Output, error) {
	if input.Avatar == nil {
		return nil, rpc.NewError(rpc.ErrValidation, "Avatar file is required")
	}

	// 1. Validate file size and mime type
	if input.Avatar.Size > 5*1024*1024 { // 5MB limit
		return nil, rpc.NewError(rpc.ErrPayloadTooLarge, "Avatar exceeds 5MB limit")
	}

	// 2. Save to persistent storage
	destPath := filepath.Join("./public/uploads", input.Avatar.Filename)
	if err := input.Avatar.SaveTo(destPath); err != nil {
		return nil, rpc.NewError(rpc.ErrInternal, fmt.Sprintf("Failed to save avatar: %v", err))
	}

	return &Output{
		AvatarURL: fmt.Sprintf("/uploads/%s", input.Avatar.Filename),
	}, nil
}
```

> **Automatic Temporary File Cleanup**: The framework tracks uploaded files inside `ctx`. When the HTTP response finishes, any unsaved temporary files are automatically purged from disk.
