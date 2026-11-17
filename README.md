<p align="center">
  <a href="https://cellsweb.com" target="_blank" rel="noopener noreferrer">
    <img src="https://cellsweb.com/logo.svg" alt="CellsWeb Logo" width="340" style="background: rgba(255, 255, 255, 0.96); padding: 16px 24px; border-radius: 16px; box-shadow: 0 8px 32px rgba(0, 0, 0, 0.08); display: inline-block; max-width: 100%;" />
  </a>
</p>

<p align="center">
  <strong>High-Performance RPC API Framework for Go</strong><br>
  <a href="https://cellsweb.com">🌐 Official Website</a> •
  <a href="https://cellsweb.github.io/crpc-go/">📖 Go Engine Docs</a> •
  <a href="https://cellsweb.github.io/crpc-node/">📖 Node Engine Docs</a> •
  <a href="https://github.com/CellsWeb">🏛️ GitHub Organization</a>
</p>

# ⚡ CellsWeb RPC — Go (`crpc-go`)

[![Go Reference](https://pkg.go.dev/badge/github.com/CellsWeb/crpc-go.svg)](https://pkg.go.dev/github.com/CellsWeb/crpc-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Status: Production](https://img.shields.io/badge/Status-Production--Ready-emerald.svg)]()

**CellsWeb RPC Go** is a high-performance, production-ready RPC API framework for Go built directly on [fasthttp](https://github.com/valyala/fasthttp). It is **100.0% wire-compatible** with the [`@cellsweb/rpc-client`](https://www.npmjs.com/package/@cellsweb/rpc-client) frontend SDK and maintains identical protocol guarantees, response envelopes, and developer experience as the TypeScript `@cellsweb/rpc-core` runtime.

---

## 📑 Table of Contents

1. [Why CellsWeb RPC Go?](#-why-cellsweb-rpc-go)
2. [Why CellsWeb RPC Go?](#-why-cellsweb-rpc-go)
3. [CellsWeb RPC vs Connect-RPC vs tRPC vs Twirp](#-cellsweb-rpc-vs-connect-rpc-vs-trpc-vs-twirp)
4. [CLI Installation & Project Scaffolding](#-cli-installation--project-scaffolding)
5. [Server Entrypoint (`main.go`)](#-server-entrypoint-maingo)
6. [Project Architecture & File Co-Location](#-project-architecture--file-co-location)
7. [How to Write RPC Functions](#-how-to-write-rpc-functions)
   - [Standard JSON Function](#1-standard-json-rpc-function)
   - [Multipart Form-Data & File Uploads](#2-multipart-file-upload-function)
8. [Writing & Attaching Middlewares](#-writing--attaching-middlewares)
9. [In-Process Testing with Zero Network Sockets](#-in-process-testing-with-zero-network-sockets)
10. [Configurable HTTP Status Codes & APM Mode](#-configurable-http-status-codes--apm-mode)
11. [Connecting from Frontend Clients (`@cellsweb/rpc-client`)](#-connecting-from-frontend-clients-cellswebrpc-client)
12. [Connecting to Node.js & Next.js Monorepos](#-connecting-to-nodejs--nextjs-monorepos)
13. [Built-in Server-Rendered Interactive Docs (`/docs`)](#-server-rendered-interactive-docs-docs)
14. [Uniform Protocol Specification & Error Codes](#-uniform-protocol-specification--error-codes)
15. [Production Deployment & Fly.io Autoscaling](#-production-deployment--flyio-autoscaling)
16. [Best Practices & What to Avoid](#-best-practices--what-to-avoid)
17. [Battle-Tested Benchmarks & Attack Survival](#-battle-tested-benchmarks--attack-survival)

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

## ⚖️ CellsWeb RPC vs Connect-RPC vs tRPC vs Twirp

Why choose CellsWeb RPC over existing RPC technologies?

| Capability / Feature | CellsWeb RPC (`crpc`) | Connect-RPC | tRPC | Twirp |
| :--- | :--- | :--- | :--- | :--- |
| **Go & TypeScript Parity** | **Yes** (100% interoperable wire format) | Yes (via Protobuf) | No (TypeScript/JS only) | Yes (via Protobuf) |
| **Zero-Codegen Client** | **Yes** (Dynamic client SDK, no build step) | No (Requires `buf generate`) | Yes (TypeScript types only) | No (Requires `protoc`) |
| **Single Endpoint (`/rpc`)** | **Yes** (Unified routing & proxy configuration) | No (Endpoint per RPC method) | No (Endpoint per procedure) | No (Endpoint per method) |
| **Zero-Config Live Docs UI** | **Yes** (Server-rendered dark theme `/docs`) | No (Requires third-party tools) | No (Third-party OpenAPI) | No |
| **Multipart File Streaming** | **Yes** (Streaming intake, Web & RN objects) | Complex (bytes in proto payload) | Complex (Custom FormData) | Complex (bytes in proto) |
| **In-Process Testing Engine** | **Yes** (`NewTestCaller` / zero socket) | No (Requires network listener) | Yes (caller) | No (Network listener) |
| **Dual HTTP Status Mode** | **Yes** (Uniform 200 or standard APM codes) | No (gRPC status codes) | No (HTTP error codes) | No (HTTP error codes) |
| **Go 1.27 JSON v2 Standard**| **Yes** (Enforced, zero-allocation serialization) | No | N/A | No |

### The Core Advantage:
- **No Protobuf Compilation Ceremony**: Connect-RPC and Twirp require maintaining `.proto` definition files, installing compilers, and regenerating code whenever a field changes. CellsWeb RPC uses pure Go structs on the backend and native TypeScript contracts on the frontend.
- **True Multi-Language Monorepos**: Unlike tRPC (which locks you into TypeScript), CellsWeb RPC lets you write high-performance Go microservices while seamlessly consuming them from React, Next.js, or React Native with identical client semantics.

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
go mod tidy

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

## 🚦 Configurable HTTP Status Codes & APM Mode

By default, CellsWeb RPC maintains a uniform **HTTP 200** envelope for all responses (including validation and business errors). However, for enterprise infrastructure requiring native HTTP status codes for SRE metrics, APM tracing (Datadog, OpenTelemetry), and reverse proxy routing:

Enable `UseHttpStatusCodes: true` in your server configuration:

```go
app := rpc.New(rpc.Config{
    UseHttpStatusCodes: true, // Emits 400, 401, 403, 404, 429, 500
})
```

### Error Code to HTTP Status Mapping:
| Protocol Error Code | HTTP Status | APM & Reverse Proxy Behavior |
| :--- | :--- | :--- |
| `validation_error`, `schema_mismatch`, `invalid_body` | **400 Bad Request** | Counted as client error (4xx) |
| `unauthorized` | **401 Unauthorized** | Triggers frontend re-auth / login redirect |
| `forbidden` | **403 Forbidden** | Access denied |
| `not_found`, `unknown_function` | **404 Not Found** | Unregistered endpoint |
| `payload_too_large` | **413 Payload Too Large** | Upload size exceeded |
| `rate_limit_exceeded` | **429 Too Many Requests** | Upstream rate limiting |
| `internal_error` | **500 Internal Server Error** | Alerts on-call engineering |

The JSON response envelope remains 100% identical and compatible with `@cellsweb/rpc-client`:
```json
{
  "status": "error",
  "code": "validation_error",
  "message": "Field 'email' is required",
  "meta": { "requestId": "req-102", "durationMs": 0.12 }
}
```

---

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

### Mode 2: High-Speed IPC Bridge & Automated TypeScript `.d.ts` Generation (`pkg/nodebridge`)

When running Go and Node.js/Next.js side-by-side on the same machine, container, or monorepo, you can bypass the TCP network stack entirely. `crpc-go` provides a built-in IPC bridge running over **Unix Domain Sockets** (`/tmp/crpc.sock`) with **<0.1ms (100µs)** latency and automated TypeScript typing.

#### 1. What Go Gives

##### A. Exporting TypeScript Contracts (`.d.ts`)
Run this snippet or add a CLI task in your Go server to generate your frontend/Node types automatically from your Go structs:

```go
package main

import (
	"os"
	"github.com/CellsWeb/crpc-go/pkg/nodebridge"
)

func exportTypes(app *rpc.CoreRPC) {
	// Generates complete TypeScript declaration map
	dts := nodebridge.GenerateTypeScriptDeclarations(app.Functions())
	_ = os.WriteFile("../frontend/src/types/crpc.d.ts", []byte(dts), 0644)
}
```

Generated `crpc.d.ts`:
```ts
export type UserGetProfileInput = {
  userId: string;
};

export type UserGetProfileOutput = {
  id: string;
  name: string;
  email: string;
};

export interface RpcContractMap {
  "user.getProfile": {
    input: UserGetProfileInput;
    output: UserGetProfileOutput;
  };
}
```

##### B. Starting the Unix Domain Socket Listener in Go
In your Go server `main.go`:
```go
go func() {
	socketPath := "/tmp/crpc.sock"
	log.Printf("🔌 IPC Socket running at %s", socketPath)
	if err := nodebridge.ServeUnixSocket(app, socketPath); err != nil {
		log.Printf("IPC Socket error: %v", err)
	}
}()
```

---

#### 2. What to Do in Node.js / Next.js

Create a lightweight IPC client in your Node.js or Next.js app (`src/lib/go-rpc.ts`):

```ts
import net from "node:net";
import readline from "node:readline";
import type { RpcContractMap } from "@/types/crpc";

export interface CallOptions {
  headers?: Record<string, string>;
  skipMiddlewares?: boolean;
}

export class GoRpcIpcClient {
  private socketPath: string;

  constructor(socketPath = "/tmp/crpc.sock") {
    this.socketPath = socketPath;
  }

  async call<K extends keyof RpcContractMap>(
    fn: K,
    data: RpcContractMap[K]["input"],
    options: CallOptions = {}
  ): Promise<RpcContractMap[K]["output"]> {
    return new Promise((resolve, reject) => {
      const client = net.createConnection({ path: this.socketPath });
      const rl = readline.createInterface({ input: client });

      client.on("connect", () => {
        // Send single-line JSON request with optional headers and middleware bypass flag
        const payload = JSON.stringify({
          id: Math.random().toString(36).slice(2),
          function: fn,
          data,
          headers: options.headers ?? {},
          skipMiddlewares: options.skipMiddlewares ?? false,
        });
        client.write(payload + "\n");
      });

      rl.on("line", (line) => {
        try {
          const res = JSON.parse(line);
          client.end();
          if (res.status === "ok") {
            resolve(res.data);
          } else {
            reject(new Error(res.error || "RPC error"));
          }
        } catch (err) {
          client.end();
          reject(err);
        }
      });

      client.on("error", (err) => {
        reject(new Error(`Failed connecting to Go IPC socket: ${err.message}`));
      });
    });
  }
}

export const goRpc = new GoRpcIpcClient();
```

---

#### 3. How to Use in Next.js Server Actions or Node Services

Now call your Go microservice with **zero HTTP latency** and **100% compile-time TypeScript type safety**:

```ts
// app/actions/user.ts
"use server";

import { headers } from "next/headers";
import { goRpc } from "@/lib/go-rpc";

export async function getUserProfile(userId: string) {
  const reqHeaders = await headers();

  // 1. Standard call with forwarded auth headers (executes Go middlewares)
  const user = await goRpc.call(
    "user.getProfile",
    { userId },
    {
      headers: {
        authorization: reqHeaders.get("authorization") ?? "",
      },
    }
  );

  return user;
}

export async function getAdminInternalData(userId: string) {
  // 2. Internal trusted call: Bypass Go middlewares directly
  const data = await goRpc.call(
    "user.getProfile",
    { userId },
    { skipMiddlewares: true }
  );

  return data;
}
```

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

## 📈 Battle-Tested Benchmarks & Attack Survival

A comprehensive benchmark harness and adversarial attack suite is available in the [`benchmarks/`](./benchmarks) directory.

### 1. Multi-Engine Throughput Comparison (50,000 Requests, 100 Concurrency)

Tested in **triplicate** on Apple Silicon M-series (macOS Darwin arm64 / Linux Docker arm64) using `oha`:

| Framework / Engine | Runtime | Environment / Hardware | Ping Baseline (RPS) | Ping Avg (ms) | Ping p50 (ms) | Ping p99 (ms) | RSS Memory (MB) |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **`crpc-go` (fasthttp)** | **Go 1.27.4** | **Host Native (Unconstrained)** | **86,946.9** | **1.14** | **1.02** | **4.36** | **23.8 MB** |
| **`crpc-go` (fasthttp)** | **Go 1.27.4** | **Docker 4 CPUs / 4GB RAM** | **62,784.4** | **1.60** | **1.41** | **4.45** | **16.1 MB** |
| **`crpc-go` (fasthttp)** | **Go 1.27.4** | **Docker 2 CPUs / 1GB RAM** | **63,119.2** | **1.58** | **1.39** | **4.08** | **12.5 MB** |
| **`crpc-go` (fasthttp)** | **Go 1.27.4** | **Docker 1 CPU / 256MB RAM** | **62,677.2** | **1.59** | **1.33** | **9.10** | **12.6 MB** |
| **`crpc-node` (`bun:serve`)** | **Bun v1.3.1** | **Host Native** | **8,741.3** | **11.40** | **11.17** | **14.92** | **94.2 MB** |
| **`crpc-node` (`bun:elysia`)** | **Bun v1.3.1** | **Host Native** | **8,703.6** | **11.45** | **11.37** | **14.30** | **98.6 MB** |
| **`crpc-node` (`bun:hono`)** | **Bun v1.3.1** | **Host Native** | **8,609.4** | **11.58** | **11.40** | **14.23** | **96.4 MB** |
| **`crpc-node` (`node:fastify`)** | **Node.js v24.2** | **Host Native** | **6,428.7** | **15.51** | **15.15** | **18.65** | **142.1 MB** |
| **`crpc-node` (`node:http`)** | **Node.js v24.2** | **Host Native** | **6,450.0** | **15.46** | **15.10** | **19.66** | **148.8 MB** |
| **`crpc-node` (`node:ultimate-express`)** | **Node.js v24.2** | **Host Native (uWS)** | **6,295.1** | **15.84** | **15.49** | **19.62** | **138.4 MB** |
| **`crpc-node` (`node:express`)** | **Node.js v24.2** | **Host Native** | **6,297.1** | **15.84** | **15.54** | **18.88** | **153.4 MB** |

### 2. Real-World Database Operations (Live PostgreSQL 16)

| Framework / Engine | Environment | DB Read (`items.list` 20 items) | Read Avg Latency | DB Write (`items.create` INSERT) | Write Avg Latency |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **`crpc-go` (Host Native)** | **Host Unconstrained** | **14,327.5 req/s** | **3.44 ms** | **45,883.8 req/s** | **1.08 ms** |
| **`crpc-go` (Docker 4C / 4GB)** | **Docker 4 CPUs / 4GB** | **10,204.3 req/s** | **4.84 ms** | **30,244.3 req/s** | **1.63 ms** |
| **`crpc-go` (Docker 2C / 1GB)** | **Docker 2 CPUs / 1GB** | **10,231.3 req/s** | **4.83 ms** | **33,451.1 req/s** | **1.46 ms** |
| **`crpc-go` (Docker 1C / 256MB)** | **Docker 1 CPU / 256MB** | **9,263.6 req/s** | **5.35 ms** | **30,076.0 req/s** | **1.63 ms** |
| **`crpc-node` (`bun:serve`)** | **Host Native** | **4,517.6 req/s** | **11.02 ms** | **7,882.0 req/s** | **6.28 ms** |
| **`crpc-node` (`bun:elysia`)** | **Host Native** | **4,465.4 req/s** | **11.15 ms** | **7,966.3 req/s** | **6.21 ms** |
| **`crpc-node` (`bun:hono`)** | **Host Native** | **4,342.7 req/s** | **11.47 ms** | **7,687.9 req/s** | **6.44 ms** |
| **`crpc-node` (`node:fastify`)** | **Host Native** | **3,935.4 req/s** | **12.65 ms** | **5,853.9 req/s** | **8.46 ms** |
| **`crpc-node` (`node:http`)** | **Host Native** | **3,894.1 req/s** | **12.79 ms** | **5,945.4 req/s** | **8.33 ms** |
| **`crpc-node` (`node:ultimate-express`)** | **Host Native (uWS)** | **3,812.9 req/s** | **13.06 ms** | **5,829.9 req/s** | **8.50 ms** |
| **`crpc-node` (`node:express`)** | **Host Native** | **3,846.3 req/s** | **12.95 ms** | **5,857.4 req/s** | **8.45 ms** |

### 3. Real Multipart File Uploads (Disk Storage & DB Persistence)

Tested with real production files: `Screenshot.png` (191 KB) & `RideRough.pdf` (29 KB) saved to disk with metadata inserted into PostgreSQL:

| Framework / Engine | PNG Image Upload (191 KB) | PNG Avg Latency | PDF Document Upload (29 KB) | PDF Avg Latency | Verification |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **`crpc-go` (Host Native)** | **2,654.8 uploads/s** | **3.70 ms** | **3,103.6 uploads/s** | **3.32 ms** | **Verified (HTTP 200)** |
| **`crpc-go` (Docker 4C / 4GB)** | **1,878.5 uploads/s** | **5.28 ms** | **2,547.0 uploads/s** | **3.84 ms** | **Verified (HTTP 200)** |
| **`crpc-go` (Docker 2C / 1GB)** | **1,951.9 uploads/s** | **5.09 ms** | **2,804.1 uploads/s** | **3.49 ms** | **Verified (HTTP 200)** |
| **`crpc-go` (Docker 1C / 256MB)** | **1,891.4 uploads/s** | **5.66 ms** | **1,957.3 uploads/s** | **9.22 ms** | **Verified (HTTP 200)** |
| **`crpc-node` (`bun:hono`)** | **2,059.4 uploads/s** | **4.82 ms** | **2,249.8 uploads/s** | **4.38 ms** | **Verified (HTTP 200)** |
| **`crpc-node` (`bun:elysia`)** | **1,901.2 uploads/s** | **5.21 ms** | **2,504.4 uploads/s** | **3.91 ms** | **Verified (HTTP 200)** |
| **`crpc-node` (`bun:serve`)** | **1,828.9 uploads/s** | **5.54 ms** | **2,448.9 uploads/s** | **4.00 ms** | **Verified (HTTP 200)** |
| **`crpc-node` (`node:http`)** | **1,009.1 uploads/s** | **10.02 ms** | **1,540.9 uploads/s** | **6.54 ms** | **Verified (HTTP 200)** |
| **`crpc-node` (`node:express`)** | **1,087.4 uploads/s** | **9.13 ms** | **1,318.3 uploads/s** | **7.55 ms** | **Verified (HTTP 200)** |

### 🛡️ Attack Survival & Fuzzing Suite

- **Concurrency Saturation**: 2,000 active concurrent connections sustained with 0 socket resets.
- **Rate Limit Flooding**: Flooded with 2,000 requests in <0.04s. Exactly 100 allowed (HTTP 200), 1,900 repelled (HTTP 429), 0 server crashes.
- **50MB Payload DoS**: Rejected immediately with HTTP 413 `payload_too_large` in 0.12 ms.
- **Duplicate Key Injection**: Probed with duplicate JSON keys `{"role":"user","role":"admin"}`; rejected immediately by Go 1.27 `encoding/json/v2` engine.
- **Path Traversal Probing**: Probed with `namespace: "../../../etc/passwd"`; rejected in 0.02 ms via strict routing regex.
- **Unauthenticated Mutation**: `items.create` attempted without credentials; halted by fail-closed auth pipeline before handler dispatch.

For full scripts and reproduction instructions, see [benchmarks/README.md](./benchmarks/README.md) and [benchmarks/RESULTS.md](./benchmarks/RESULTS.md).

---

## 📄 License

MIT © [CellsWeb](https://cellsweb.com)


### Deprecated documentation metadata

`rpc.FunctionMetadata.IsPublic` is deprecated and ignored. The existing `bool`
field and JSON tag remain for source compatibility until a future major release.
It has no documentation visibility or authentication/authorization meaning.
PUBLIC/PRIVATE badges are removed; new generators omit this field. Middleware
handles access control, and `rpc.Config.DocsEnabled` controls docs globally.
Middleware documentation shows schema structure, descriptions and required/optional
fields, never configured values or schema defaults. See
[Interactive Documentation](docs/interactive-docs.md).
