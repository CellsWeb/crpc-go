# ⚡ CellsWeb RPC — Go (`crpc-go`)

[![Go Reference](https://pkg.go.dev/badge/github.com/CellsWeb/crpc-go.svg)](https://pkg.go.dev/github.com/CellsWeb/crpc-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Status: Production](https://img.shields.io/badge/Status-Production--Ready-emerald.svg)]()

**CellsWeb RPC Go** is a high-performance, production-ready RPC API framework for Go built directly on [fasthttp](https://github.com/valyala/fasthttp). It is **100.0% wire-compatible** with the [`@cellsweb/rpc-client`](https://www.npmjs.com/package/@cellsweb/rpc-client) frontend SDK and maintains identical protocol guarantees, response envelopes, and developer experience as the TypeScript `@cellsweb/rpc-core` runtime.

---

## 📑 Table of Contents

1. [Why CellsWeb RPC Go?](#-why-cellsweb-rpc-go)
2. [CLI Installation & Project Scaffolding](#-cli-installation--project-scaffolding)
3. [Server Entrypoint (`main.go`)](#-server-entrypoint-maingo)
4. [Project Architecture & File Co-Location](#-project-architecture--file-co-location)
5. [How to Write RPC Functions](#-how-to-write-rpc-functions)
   - [Standard JSON Function](#1-standard-json-rpc-function)
   - [Multipart Form-Data & File Uploads](#2-multipart-file-upload-function)
6. [Writing & Attaching Middlewares](#-writing--attaching-middlewares)
7. [In-Process Testing with Zero Network Sockets](#-in-process-testing-with-zero-network-sockets)
8. [Connecting from Frontend Clients (`@cellsweb/rpc-client`)](#-connecting-from-frontend-clients-cellswebrpc-client)
9. [Connecting to Node.js & Next.js Monorepos](#-connecting-to-nodejs--nextjs-monorepos)
10. [Built-in Server-Rendered Interactive Docs (`/docs`)](#-server-rendered-interactive-docs-docs)
11. [Uniform Protocol Specification & Error Codes](#-uniform-protocol-specification--error-codes)
12. [Production Deployment & Fly.io Autoscaling](#-production-deployment--flyio-autoscaling)
13. [Best Practices & What to Avoid](#-best-practices--what-to-avoid)
14. [Benchmarks](#-benchmarks)

---

## 🚀 Why CellsWeb RPC Go?

- **Extreme Throughput**: **68,900+ req/sec** on a single CPU core with **sub-millisecond (<1ms)** average latency.
- **Micro Memory Footprint**: Operates at **~14.8 MB RAM** under heavy load — over **16x lighter** than Node.js and Bun, making it ideal for 256MB microVMs (e.g. Fly.io at \$2.79/month).
- **Single-Endpoint Architecture**: All RPC requests dispatch through `POST /rpc` with uniform `{ namespace, functionName, data }` envelopes.
- **Uniform HTTP 200 Protocol**: Zero HTTP status code guessing. Validation failures, authorization rejections, and business errors return structured HTTP 200 payloads with unambiguous error codes.
- **Multi-Phase Streaming Multipart Engine**: Phase 1 early routing inspects keys and executes authentication middlewares *before* streaming large file uploads to disk.
- **Zero-Config Interactive Docs (`/docs`)**: Pure server-rendered dark theme documentation UI with 2-column schema view, recursive fields breakdown, copyable cURL commands, and central middleware catalog — with **zero third-party UI dependencies**.
- **First-Class Frontend & Monorepo Interop**: Direct drop-in compatibility with React/React Native hooks (`useRpcQuery`, `useRpcMutation`), Next.js server actions, and high-speed local IPC bridges (stdio & Unix Domain Sockets).

---

## 📦 CLI Installation & Project Scaffolding

### 1. Install the `crpc-go` CLI Tool

```bash
go install github.com/CellsWeb/crpc-go/cmd/crpc-go@latest
```

### 2. CLI Commands Reference

```bash
# Initialize a new production-ready project
crpc-go init my-service
cd my-service

# Create a standard JSON RPC Function
crpc-go create:function --ns user --name getProfile

# Create a Multipart File Upload Function
crpc-go create:function --ns user --name uploadAvatar --multipart

# Create an RPC Function with attached middlewares
crpc-go create:function --ns user --name updateSettings --middlewares "auth,rateLimit"

# Create a reusable middleware
crpc-go create:middleware --name rateLimit --desc "Per-function IP rate limiting"

# Run development server with ultra-fast file watcher (<200ms auto-restart)
crpc-go dev

# Compile standalone production binary
crpc-go build

# AI Agent Batch Scaffolding (non-interactive)
crpc-go create:function --batch '[
  {"namespace":"catalog","functionName":"listProducts"},
  {"namespace":"catalog","functionName":"uploadProductImage","multipart":true,"middlewares":["auth"]}
]'
```

---

## 🔌 Server Entrypoint (`main.go`)

Here is how a clean, production-grade `crpc-go` server is configured:

```go
package main

import (
	"log"
	"os"

	"github.com/CellsWeb/crpc-go/pkg/rpc"
	"my-service/utils/storage"

	// Self-register functions & middlewares via package init()
	_ "my-service/functions/user/getProfile"
	_ "my-service/functions/user/uploadAvatar"
	_ "my-service/middlewares/auth"
)

func main() {
	// 1. Initialize CoreRPC
	app, err := rpc.New(rpc.Config{
		RpcPath:     "/rpc",
		Port:        4000,
		Host:        "0.0.0.0",
		DocsEnabled: os.Getenv("NODE_ENV") != "production",
		DocsPath:    "/docs",
		CORS: &rpc.CorsConfig{
			AllowOrigins: []string{"*"},
			AllowMethods: []string{"POST", "GET", "OPTIONS"},
			AllowHeaders: []string{"Content-Type", "Authorization"},
		},
	})
	if err != nil {
		log.Fatalf("CoreRPC initialization error: %v", err)
	}

	// 2. Load self-registered functions & middlewares from global registry
	app.UseGlobalRegistry()

	// 3. Optional: Register custom HTTP routes (Health check, webhooks, redirects)
	app.GET("/health", func(c *rpc.HTTPContext) error {
		return c.SendString("OK")
	})

	// 4. Custom HTTP route: Static Media Server for /uploads/*
	app.GET("/uploads/*", func(c *rpc.HTTPContext) error {
		path := c.Path()
		data, mimeType, err := storage.DefaultEngine.Serve(path)
		if err != nil {
			c.Status(404)
			return c.SendString("File not found")
		}
		c.SetContentType(mimeType)
		return c.SendBytes(data)
	})

	// 5. Start high-performance fasthttp server
	log.Printf("⚡ CellsWeb RPC Go server running at http://0.0.0.0:4000/rpc")
	log.Printf("📖 Interactive Docs available at http://0.0.0.0:4000/docs")
	if err := app.ListenAndServe(); err != nil {
		log.Fatalf("Server stopped: %v", err)
	}
}
```

---

## 🏗️ Project Architecture & File Co-Location

Every RPC function in `crpc-go` is co-located inside `functions/{namespace}/{functionName}/` across three single-responsibility files:

```
my-service/
├── cmd/
│   └── server/main.go          # Application entrypoint & HTTP server
├── functions/
│   └── user/
│       └── getProfile/
│           ├── contracts.go    # Declarative contracts, metadata, inputs & outputs
│           ├── middlewares.go  # Declarative middleware attachments
│           └── run.go          # Pure business logic execution handler
├── middlewares/
│   └── auth/
│       ├── constants.go        # Options & headers schemas
│       └── run.go              # Middleware interceptor logic
├── utils/
│   ├── storage/                # Media upload & sandbox-safe path resolution
│   └── imagehandler/           # Automated image resizing & relative URLs
├── go.mod
└── .env
```

### The Three Function Files:
1. **`contracts.go`**: Declares `Namespace`, `FunctionName`, `ContentType`, `Metadata`, and the strongly-typed `Input` and `Output` Go structs. **Zero execution logic.**
2. **`middlewares.go`**: Declaratively attaches reusable middlewares with type-safe option parameters.
3. **`run.go`**: Contains the `init()` registration hook and the pure handler: `func Run(ctx *rpc.Context, input Input) (*Output, error)`.

---

## ✍️ How to Write RPC Functions

### 1. Standard JSON RPC Function

#### `functions/user/getProfile/contracts.go`
```go
package getprofile

import (
	"github.com/CellsWeb/crpc-go/pkg/rpc"
)

const (
	Namespace    = "user"
	FunctionName = "getProfile"
	ContentType  = "application/json"
)

var Metadata = rpc.FunctionMetadata{
	Description: "Retrieves user profile information by unique identifier",
	IsPublic:    true,
	Tags:        []string{"user", "profile"},
}

type Input struct {
	UserID string `json:"userId" doc:"Unique user identifier"`
}

type Output struct {
	ID    string `json:"id" doc:"User ID"`
	Name  string `json:"name" doc:"Full name"`
	Email string `json:"email" doc:"Verified email address"`
}
```

#### `functions/user/getProfile/middlewares.go`
```go
package getprofile

import "github.com/CellsWeb/crpc-go/pkg/rpc"

var Middlewares = rpc.DefineMiddlewares(
	rpc.M("auth", map[string]any{"roles": []string{"user", "admin"}}),
	rpc.M("rateLimit", map[string]any{"windowSeconds": 60, "maxRequests": 100}),
)
```

#### `functions/user/getProfile/run.go`
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
	if input.UserID == "" {
		return nil, rpc.NewError(rpc.ErrValidation, "userId is required")
	}

	// Read injected authentication context from middleware
	currentUserID, _ := ctx.Get("userId")
	_ = currentUserID

	return &Output{
		ID:    input.UserID,
		Name:  "Jane Doe",
		Email: "jane@example.com",
	}, nil
}
```

---

### 2. Multipart File Upload Function

`crpc-go` handles multipart file uploads natively without external form parsers. Files are parsed into `*rpc.UploadedFile` instances with automatic size validation and MIME type checking.

#### `functions/user/uploadAvatar/contracts.go`
```go
package uploadavatar

import "github.com/CellsWeb/crpc-go/pkg/rpc"

const (
	Namespace    = "user"
	FunctionName = "uploadAvatar"
	ContentType  = "multipart/form-data"
)

var Metadata = rpc.FunctionMetadata{
	Description: "Uploads and updates a user profile avatar image",
	IsPublic:    true,
	Tags:        []string{"user", "upload"},
}

type Input struct {
	Email  string            `json:"email" doc:"User email address"`
	Avatar *rpc.UploadedFile `json:"avatar" doc:"Uploaded image file"`
}

type Output struct {
	Success   bool   `json:"success" doc:"Operation status"`
	AvatarURL string `json:"avatarUrl" doc:"Relative URL to uploaded image"`
}
```

#### `functions/user/uploadAvatar/run.go`
```go
package uploadavatar

import (
	"my-service/utils/storage"
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
	if input.Avatar == nil {
		return nil, rpc.NewError(rpc.ErrValidation, "Avatar file is required")
	}

	// Validate file size (< 5MB)
	if input.Avatar.Size() > 5*1024*1024 {
		return nil, rpc.NewError(rpc.ErrPayloadTooLarge, "Avatar file exceeds 5MB limit")
	}

	// Save avatar using local storage engine (stores to public/uploads/avatars/...)
	result, err := storage.DefaultEngine.Upload(input.Avatar, "avatars", input.Email)
	if err != nil {
		return nil, rpc.NewError(rpc.ErrInternal, "Upload failed: "+err.Error())
	}

	return &Output{
		Success:   true,
		AvatarURL: result.RelativeURL, // e.g. "uploads/avatars/user.jpg"
	}, nil
}
```

---

## ⚡ Writing & Attaching Middlewares

Middlewares run before function execution. They validate headers, enforce rate limits, authenticate credentials, and inject dependencies into the request context.

### `middlewares/auth/constants.go`
```go
package auth

import "github.com/CellsWeb/crpc-go/pkg/schema"

type Options struct {
	Roles    []string `json:"roles,omitempty"`
	Optional bool     `json:"optional,omitempty"`
}

var OptionsSchema = schema.Object(map[string]*schema.SchemaRule{
	"roles":    schema.Optional(schema.Array(schema.String())).Describe("Allowed roles"),
	"optional": schema.Optional(schema.Boolean()).Describe("Allow unauthenticated callers"),
})

var HeadersSchema = schema.Object(map[string]*schema.SchemaRule{
	"authorization": schema.Optional(schema.String()).Describe("Bearer JWT token"),
})
```

### `middlewares/auth/run.go`
```go
package auth

import (
	"strings"
	"github.com/CellsWeb/crpc-go/pkg/rpc"
)

func init() {
	rpc.RegisterMiddleware("auth", "Validates bearer token and enforces roles", Run, OptionsSchema, HeadersSchema)
}

func Run(ctx *rpc.Context, headers map[string]string, options map[string]any) error {
	token := headers["authorization"]
	if token == "" {
		if opt, _ := options["optional"].(bool); opt {
			return nil
		}
		return rpc.NewError(rpc.ErrUnauthorized, "Missing Authorization header")
	}

	// Fail-closed authorization check
	if !strings.HasPrefix(token, "Bearer ") {
		return rpc.NewError(rpc.ErrUnauthorized, "Invalid Bearer token format")
	}

	// Verify token & inject user into context
	userID := "usr_1001"
	ctx.Set("userId", userID)
	return nil
}
```

---

## 🧪 In-Process Testing with Zero Network Sockets

`crpc-go` includes `TestCaller` for writing fast integration tests without binding to TCP ports:

```go
package test

import (
	"testing"
	"github.com/CellsWeb/crpc-go/pkg/rpc"
	_ "my-service/functions/user/getProfile"
	_ "my-service/middlewares/auth"
)

func TestUserProfileCall(t *testing.T) {
	app, _ := rpc.New(rpc.Config{
		RpcPath: "/rpc",
		CORS:    &rpc.CorsConfig{AllowOrigins: []string{"*"}},
	})
	app.UseGlobalRegistry()

	// Create in-process test caller with default headers
	caller := rpc.NewTestCaller(app, rpc.TestOptions{
		Headers: map[string]string{
			"authorization": "Bearer valid_token",
		},
	})

	// 1. Strongly typed Call()
	type ProfileInput struct {
		UserID string `json:"userId"`
	}
	type ProfileOutput struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	var output ProfileOutput
	err := caller.Call("user.getProfile", ProfileInput{UserID: "usr_101"}, &output)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if output.Name != "Jane Doe" {
		t.Errorf("Expected name 'Jane Doe', got %q", output.Name)
	}

	// 2. CallRaw() for status code & error code assertion
	unauthCaller := rpc.NewTestCaller(app, rpc.TestOptions{})
	rawRes, err := unauthCaller.CallRaw("user.getProfile", ProfileInput{UserID: "usr_101"})
	if err != nil {
		t.Fatalf("Unexpected network error: %v", err)
	}
	if rawRes.Code != "unauthorized" {
		t.Errorf("Expected unauthorized code, got %s", rawRes.Code)
	}
}
```

---

## 🌐 Connecting from Frontend Clients (`@cellsweb/rpc-client`)

`crpc-go` is 100% compatible with the official React and React Native client SDK:

```bash
npm install @cellsweb/rpc-client
```

### Initialize Client
```tsx
import { createRpcClient } from "@cellsweb/rpc-client";

export const rpc = createRpcClient({
  baseUrl: "http://localhost:4000",
  getHeaders: () => ({
    authorization: `Bearer ${localStorage.getItem("token")}`,
  }),
});
```

### React Query & Mutation Hooks
```tsx
import { useRpcQuery, useRpcMutation } from "@cellsweb/rpc-client";
import { rpc } from "./rpc";

// 1. Data Query
export function UserProfile({ id }: { id: string }) {
  const { data, loading, error } = useRpcQuery(rpc, "user", "getProfile", { userId: id });

  if (loading) return <div>Loading...</div>;
  if (error) return <div>Error: {error.message}</div>;

  return <h1>{data.name} ({data.email})</h1>;
}

// 2. Automatic Multipart File Upload Mutation
export function AvatarUpload() {
  const { mutate, loading } = useRpcMutation(rpc, "user", "uploadAvatar", { multipart: true });

  const onFileSelect = (file: File) => {
    // Standard JS object containing File — converted to FormData automatically!
    mutate({ email: "user@example.com", avatar: file });
  };

  return <input type="file" onChange={(e) => onFileSelect(e.target.files![0])} />;
}
```

---

## 🔗 Connecting to Node.js & Next.js Monorepos

If you are running Go as a high-performance microservice alongside Next.js or Node.js server actions, you have two integration modes:

### Mode 1: Direct HTTP Request (`rawRpcRequest`)
```ts
import { rawRpcRequest } from "@cellsweb/rpc-client";

export async function getUser(userId: string) {
  const user = await rawRpcRequest({
    baseUrl: process.env.GO_RPC_URL ?? "http://localhost:4000",
    namespace: "user",
    functionName: "getProfile",
    data: { userId },
  });
  return user;
}
```

### Mode 2: IPC Unix Domain Socket / Stdio Bridge (`pkg/nodebridge`)
For local monorepos where Node and Go run on the same machine, `crpc-go` includes a built-in IPC bridge:
- Runs via Unix Domain Socket (`/tmp/crpc.sock`) with zero TCP network stack overhead.
- Includes TypeScript generator: `nodebridge.GenerateTypeScriptDeclarations(app)` exports `.d.ts` declaration maps from Go struct definitions automatically!

---

## 📖 Server-Rendered Interactive Docs (`/docs`)

`crpc-go` includes a built-in, zero-dependency interactive documentation UI:

- **2-Column Split Schema View**:
  - **Left**: Recursive hierarchical bullet list displaying field names, types, optional/required states, and human-readable descriptions.
  - **Right**: Formatted Draft 7 JSON Schema code block with a one-click `Copy` button.
- **Dynamic Middleware Cards**: Displays configured option values, schema defaults, and required HTTP headers.
- **cURL Command Generator**: Auto-generates ready-to-run terminal commands with appropriate `-H` headers and `-d` or `-F` multipart payloads.
- **Live Search**: Client-side regex and fuzzy search across namespaces, functions, and middlewares.

Access the docs in your browser at `http://localhost:4000/docs`.

---

## 📋 Uniform Protocol Specification & Error Codes

Every RPC request returns **HTTP Status 200** with a uniform JSON envelope.

### Success Envelope
```json
{
  "status": "ok",
  "data": { ... },
  "meta": {
    "requestId": "clx123abc456",
    "durationMs": 0.84
  }
}
```

### Error Envelope
```json
{
  "status": "error",
  "code": "validation_error",
  "message": "Input validation failed: userId is required",
  "meta": {
    "requestId": "clx123abc456",
    "durationMs": 1.20
  }
}
```

### Standard Error Codes:
| Error Code | HTTP Equivalent | When it Fires |
| :--- | :--- | :--- |
| `invalid_content_type` | 415 | Content-Type is not `application/json` or `multipart/form-data` |
| `invalid_body` | 400 | Request body is empty, null, or malformed JSON |
| `function_not_found` | 404 | Requested `{ namespace, functionName }` is not registered |
| `validation_error` | 422 | Input payload fails required types or constraints |
| `unauthorized` | 401 | Auth middleware rejects caller credentials |
| `forbidden` | 403 | Caller lacks required role or permissions |
| `rate_limited` | 429 | Caller exceeded request quota limits |
| `schema_mismatch` | 500 | Function output violates defined output contract |
| `payload_too_large` | 413 | Request payload or file upload exceeds size limits |
| `timeout` | 504 | Execution duration exceeds configured limit |
| `internal_error` | 500 | Unhandled panic or database crash |

---

## 🚀 Production Deployment & Fly.io Autoscaling

Because `crpc-go` compiles to a static binary and consumes only **~14.8 MB of RAM**, it is the perfect fit for **Fly.io's \$2.79/month microVM tier (1 CPU, 256MB RAM)**.

### Production `Dockerfile` (Multi-Stage Build ~14MB Image)
```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/server cmd/server/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/
COPY --from=builder /app/server .
EXPOSE 4000
CMD ["./server"]
```

### Fly.io Autoscaling Configuration (`fly.toml`)
```toml
app = "my-crpc-service"
primary_region = "syd"

[http_service]
  internal_port = 4000
  force_https = true
  auto_stop_machines = true
  auto_start_machines = true
  min_machines_running = 1

  [http_service.concurrency]
    type = "requests"
    soft_limit = 2500  # Proxy spawns a new Firecracker VM when active requests hit 2,500
    hard_limit = 5000  # Hard ceiling per machine before proxy queues traffic

[[vm]]
  cpu_kind = "shared"
  cpus = 1
  memory_mb = 256
```
*(Cold start on Fly Firecracker microVMs is **<150ms** because Go has zero runtime warmup or JIT compilation).*

---

## 🛡️ Best Practices & What to Avoid

### Best Practices:
1. **Keep `contracts.go` Declarative**: Never put database queries or network logic in `contracts.go`. It should solely declare `Namespace`, `FunctionName`, `ContentType`, `Metadata`, and `Input`/`Output` structs.
2. **Use Struct `doc` Tags**: Always add `doc:"..."` tags to your struct fields so descriptions populate in `/docs`.
3. **Fail-Closed Middlewares**: Always return `rpc.NewError(rpc.ErrUnauthorized, "...")` or `rpc.NewError(rpc.ErrForbidden, "...")` to halt execution before function handlers run.
4. **Use Domain-Agnostic Storage Paths**: Store files as relative paths (`uploads/avatars/image.jpg`) in your database. Prepend domains dynamically using `storage.DefaultEngine.ResolveImageUrl(url)`.

### What to Avoid:
- ❌ **Do NOT Return Non-200 HTTP Statuses**: CellsWeb RPC uses HTTP 200 for validation and business errors so frontend SDKs receive structured error codes. Use `rpc.NewError(rpc.ErrValidation, "...")`.
- ❌ **Do NOT Parse Upload Streams Before Middlewares**: Always declare middlewares in `middlewares.go`. The engine guarantees middlewares execute in Phase 1 before streaming heavy files to disk.
- ❌ **Do NOT Hardcode Server Domains in the DB**: Never store `http://api.myapp.com/uploads/...` in database fields. Store relative `uploads/...` paths to ensure smooth domain migrations and CDN transitions.

---

## 📈 Benchmarks

Single-core performance tested on `GOMAXPROCS=1` against `public.hello` (`autocannon -c 100 -d 10`):

| Metric | Node.js (v24) | Bun (v1.3) | Go (`crpc-go`) | Go Improvement |
| :--- | :--- | :--- | :--- | :--- |
| **Throughput (Avg)** | 4,364 req/sec | 4,362 req/sec | **68,909 req/sec** | **⚡ 15.8x Faster** |
| **Average Latency** | 22.40 ms | 22.42 ms | **0.99 ms** | **⚡ 22.6x Lower Latency** |
| **p99 Tail Latency** | 28.00 ms | 45.00 ms | **2.00 ms** | **⚡ 14x – 22x Lower** |
| **Worst-Case Spikes** | 604.00 ms | 105.00 ms | **22.00 ms** | **⚡ Zero Stutter** |
| **RAM Footprint (RSS)** | 233.0 MB | 207.8 MB | **14.8 MB** | **⚡ 1/16th Memory Usage** |
| **Docker 256MB Tier** | Crashed (OOM) | Crashed (OOM) | **18,981 req/sec** | **Rock Solid** |

---

## 📄 License

MIT © [CellsWeb](https://cellsweb.com)
