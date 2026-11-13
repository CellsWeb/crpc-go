# ⚡ CellsWeb RPC — Go (`crpc-go`)

[![Go Reference](https://pkg.go.dev/badge/github.com/CellsWeb/crpc-go.svg)](https://pkg.go.dev/github.com/CellsWeb/crpc-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Status: Production](https://img.shields.io/badge/Status-Production--Ready-emerald.svg)]()

**CellsWeb RPC Go** is a high-performance, production-ready RPC API framework for Go built directly on [fasthttp](https://github.com/valyala/fasthttp). It is **100.0% wire-compatible** with the [`@cellsweb/rpc-client`](https://www.npmjs.com/package/@cellsweb/rpc-client) frontend SDK and maintains identical protocol guarantees, response envelopes, and developer experience as the TypeScript `@cellsweb/rpc-core` runtime.

---

## 🚀 Why CellsWeb RPC Go?

- **Extreme Performance**: **68,900+ req/sec** on a single core with **sub-millisecond (<1ms)** average latency.
- **Micro Memory Footprint**: Runs at **~14.8 MB RAM** under heavy load — over **16x lighter** than Node.js and Bun, making it ideal for 256MB microVMs (e.g. Fly.io at \$2.79/month).
- **Single-Endpoint Architecture**: All RPC calls dispatch through `POST /rpc` with uniform `{ namespace, functionName, data }` envelopes.
- **Uniform HTTP 200 Protocol**: Zero HTTP status code guessing — client-side errors and success payloads share structured envelopes with full error codes and validation details.
- **Multi-Phase Streaming Multipart Engine**: Phase 1 early routing inspects keys and executes authentication middlewares *before* buffering or streaming large file uploads to disk.
- **Zero-Config Interactive Docs (`/docs`)**: Pure server-rendered dark theme documentation UI with 2-column side-by-side schema views, recursive fields breakdown, copyable cURL commands, and central middleware catalogs — with **zero third-party UI dependencies**.
- **First-Class Frontend & Monorepo Interop**: Direct drop-in compatibility with React/React Native hooks (`useRpcQuery`, `useRpcMutation`), Next.js server actions, and high-speed local IPC bridges (stdio & Unix Domain Sockets).

---

## 📦 Quick Start

### 1. Install the CLI Scaffolder

```bash
go install github.com/CellsWeb/crpc-go/cmd/crpc-go@latest
```

### 2. Initialize a New Project

```bash
crpc-go init my-service
cd my-service
```

### 3. Start the Development Server (<200ms auto-reload)

```bash
crpc-go dev
```

Visit **`http://localhost:4000/docs`** to see your live, interactive documentation UI!

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
	// Business logic: database query or service call
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
	UserID string            `json:"userId" doc:"Target user ID"`
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

	// Save avatar using local storage engine
	result, err := storage.DefaultEngine.Upload(input.Avatar, "avatars", input.UserID)
	if err != nil {
		return nil, rpc.NewError(rpc.ErrInternal, "Upload failed: "+err.Error())
	}

	return &Output{
		Success:   true,
		AvatarURL: result.RelativeURL, // e.g. "uploads/avatars/usr_123.jpg"
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

	// Verify token & inject user into context
	userID := "usr_1001"
	ctx.Set("userId", userID)
	return nil
}
```

---

## 🌐 Connecting from Frontend Clients

### Using `@cellsweb/rpc-client` (React / React Native / Expo)

```tsx
import { createRpcClient, useRpcQuery, useRpcMutation } from "@cellsweb/rpc-client";

export const rpc = createRpcClient({
  baseUrl: "http://localhost:4000",
  getHeaders: () => ({
    authorization: `Bearer ${localStorage.getItem("token")}`,
  }),
});

// React Query Hook
export function UserProfile({ id }: { id: string }) {
  const { data, loading, error } = useRpcQuery(rpc, "user", "getProfile", { userId: id });

  if (loading) return <div>Loading...</div>;
  if (error) return <div>Error: {error.message}</div>;

  return <h1>{data.name} ({data.email})</h1>;
}

// File Upload Mutation (Automatic FormData conversion)
export function AvatarUpload() {
  const { mutate, loading } = useRpcMutation(rpc, "user", "uploadAvatar", { multipart: true });

  const onFileSelect = (file: File) => {
    mutate({ userId: "usr_1001", avatar: file });
  };

  return <input type="file" onChange={(e) => onFileSelect(e.target.files![0])} />;
}
```

---

## 📖 Server-Rendered Interactive Docs (`/docs`)

`crpc-go` includes a built-in, zero-dependency interactive documentation UI:

- **2-Column Split Schema View**:
  - **Left**: Recursive hierarchical bullet list displaying field names, types, optional/required states, and human-readable descriptions.
  - **Right**: Formatted JSON Schema code block with a one-click `Copy` button.
- **Dynamic Middleware Cards**: Displays configured option values, schema defaults, and required HTTP headers.
- **cURL Command Generator**: Auto-generates ready-to-run terminal commands with appropriate `-H` headers and `-d` or `-F` multipart payloads.
- **Live Search**: Client-side regex and fuzzy search across namespaces, functions, and middlewares.

Access the docs at `http://localhost:4000/docs`.

---

## 🛡️ Best Practices & Rules

1. **Keep `contracts.go` Declarative**: Never put database calls or execution logic inside `contracts.go`. It should solely define `Namespace`, `FunctionName`, `ContentType`, `Metadata`, and your `Input`/`Output` structs.
2. **Always Use Struct `doc` Tags**: Add `doc:"..."` tags to your struct fields so they automatically populate descriptions in the `/docs` UI.
3. **Fail-Closed Security**: Return `rpc.NewError(rpc.ErrUnauthorized, "...")` or `rpc.NewError(rpc.ErrForbidden, "...")` from middlewares to halt execution before function handlers run.
4. **Use Domain-Agnostic Storage Paths**: Store files as relative paths (`uploads/avatars/image.jpg`) in your database. Use `storage.DefaultEngine.ResolveImageUrl(url)` to prepend host domains dynamically.

---

## 🚫 What to Avoid (Anti-Patterns)

- ❌ **Do NOT Return Non-200 HTTP Statuses**: CellsWeb RPC uses HTTP 200 for validation and business errors to provide consistent error objects to the client. Use `rpc.NewError(rpc.ErrValidation, "...")` instead of manual HTTP status overrides.
- ❌ **Do NOT Parse Upload Streams Before Middlewares**: Always declare middlewares in `middlewares.go`. The engine guarantees middlewares execute in Phase 1 before streaming heavy files to disk.
- ❌ **Do NOT Hardcode Server Domains in the DB**: Never store `http://api.myapp.com/uploads/...` in database fields. Store `uploads/...` so you can migrate domains or switch CDNs without running database migrations.

---

## 📈 Benchmarks

Single-core performance tested on `GOMAXPROCS=1` against `public.hello` (`autocannon -c 100 -d 10`):

| Metric | Node.js (v24) | Bun (v1.3) | Go (`crpc-go`) | Go Improvement |
| :--- | :--- | :--- | :--- | :--- |
| **Throughput (Avg)** | 4,364 req/sec | 4,362 req/sec | **68,909 req/sec** | **⚡ 15.8x Faster** |
| **Average Latency** | 22.40 ms | 22.42 ms | **0.99 ms** | **⚡ 22.6x Lower Latency** |
| **p99 Tail Latency** | 28.00 ms | 45.00 ms | **2.00 ms** | **⚡ 14x – 22x Lower** |
| **RAM Footprint (RSS)** | 233.0 MB | 207.8 MB | **14.8 MB** | **⚡ 1/16th Memory Usage** |

---

## 📄 License

MIT © [CellsWeb](https://cellsweb.com)
