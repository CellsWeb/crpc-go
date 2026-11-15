---
layout: default
title: Go API Reference
nav_order: 10
---

# Go API Reference

Exhaustive, production-grade reference of all exported packages, structs, interfaces, and functions in `github.com/CellsWeb/crpc-go`.

---

## 📦 Package `pkg/rpc`

The core RPC server engine written on top of `valyala/fasthttp`.

### 1. Server Configuration (`rpc.Config`)

```go
type Config struct {
    // RpcPath is the required endpoint path (e.g. "/rpc"). Must begin with a leading slash.
    RpcPath string

    // Port is the TCP listening port (e.g. 4000).
    Port int

    // Host to bind to (e.g. "0.0.0.0" or "127.0.0.1").
    Host string

    // CORS is the required CORS policy. Must be explicitly provided.
    CORS *CorsConfig

    // DocsEnabled controls whether the interactive documentation UI (/docs) is active.
    DocsEnabled bool

    // DocsPath is the custom path for docs (defaults to "/docs").
    DocsPath string

    // MaxRequestBodySize in bytes (e.g. 10 * 1024 * 1024 for 10MB).
    MaxRequestBodySize int

    // MaxMultipartFiles is the maximum total number of files allowed in a single request (default: 20).
    MaxMultipartFiles int

    // UseHttpStatusCodes maps RPC error codes to standard HTTP status codes (400, 401, 403, 404, 429, 500)
    // for Layer 7 load balancers and APM probes. Defaults to false (uniform HTTP 200).
    UseHttpStatusCodes bool

    // TelemetryHooks are optional callbacks for external metrics, Sentry, or batch loggers.
    TelemetryHooks []TelemetryHook

    // TrustProxy determines whether X-Forwarded-For is trusted for client IP extraction.
    TrustProxy bool
}
```

#### `CorsConfig` Structure

```go
type CorsConfig struct {
    AllowedOrigins   []string // e.g. []string{"*"} or []string{"https://app.com"}
    AllowedMethods   []string // e.g. []string{"POST", "OPTIONS"}
    AllowedHeaders   []string // e.g. []string{"Content-Type", "Authorization", "X-Request-Id"}
    ExposedHeaders   []string
    AllowCredentials bool
    MaxAge           int      // Preflight cache duration in seconds (e.g. 86400)
}
```

---

### 2. `CoreRPC` Server Instance

#### Core Lifecycle & Routing Methods

| Method | Signature | Description |
| :--- | :--- | :--- |
| `rpc.New` | `(config Config) (*CoreRPC, error)` | Validates configuration and instantiates server. |
| `UseGlobalRegistry` | `() *CoreRPC` | Discovers and imports all functions and middlewares registered via `init()`. |
| `ListenAndServe` | `() error` | Binds to configured host/port and starts the fasthttp event loop. |
| `Shutdown` | `() error` | Gracefully drains active requests and terminates the listener. |
| `Handler` | `() fasthttp.RequestHandler` | Returns the raw fasthttp request handler function. |
| `NetHTTPHandler` | `() http.Handler` | Returns standard Go `net/http` adapter for external Go routers (Chi, Gin, Echo). |

#### Custom HTTP Route Registration

You can register custom REST/HTTP routes alongside the `/rpc` endpoint:

```go
app.GET("/health", func(ctx *fasthttp.RequestCtx) {
    ctx.SetStatusCode(200)
    ctx.SetBodyString(`{"status":"ok"}`)
})

app.POST("/webhook/stripe", func(ctx *fasthttp.RequestCtx) {
    // Process external webhook
})
```

| Method | Signature | Description |
| :--- | :--- | :--- |
| `GET` | `(path string, handler HTTPHandlerFunc) *CoreRPC` | Registers a raw HTTP `GET` handler. |
| `POST` | `(path string, handler HTTPHandlerFunc) *CoreRPC` | Registers a raw HTTP `POST` handler. |
| `PUT` | `(path string, handler HTTPHandlerFunc) *CoreRPC` | Registers a raw HTTP `PUT` handler. |
| `DELETE`| `(path string, handler HTTPHandlerFunc) *CoreRPC` | Registers a raw HTTP `DELETE` handler. |

---

### 3. In-Process Testing (`CallInMemory`)

Executes an RPC function directly in-process without network overhead or port binding:

```go
err := app.CallInMemory("catalog.getProduct", Input{ID: "item-1"}, &output, rpc.CallOptions{
    Headers: map[string]string{"authorization": "Bearer admin-token"},
    ClientIP: "127.0.0.1",
})
```

---

### 4. Telemetry & Observability API

#### `TelemetryEvent` Structure

```go
type TelemetryEvent struct {
    Status       string // "ok" or "error"
    Namespace    string
    FunctionName string
    RequestID    string
    DurationMs   float64
    Code         string
    Message      string
    ClientIP     string
    Timestamp    time.Time
}

type TelemetryHook func(event TelemetryEvent)
```

#### Production Example: Sentry & Prometheus Integration

```go
app := rpc.New(rpc.Config{
    RpcPath: "/rpc",
    Port:    4000,
    CORS:    rpc.DefaultCORS(),
    TelemetryHooks: []rpc.TelemetryHook{
        func(evt rpc.TelemetryEvent) {
            if evt.Status == "error" {
                sentry.CaptureMessage(fmt.Sprintf("[%s.%s] %s: %s", 
                    evt.Namespace, evt.FunctionName, evt.Code, evt.Message))
            }
            metrics.RpcDuration.WithLabelValues(evt.Namespace, evt.FunctionName).Observe(evt.DurationMs)
        },
    },
})
```

---

### 5. Function & Middleware Registration

#### `rpc.Register[TInput, TOutput]`

Registers a strongly-typed RPC procedure:

```go
func init() {
    rpc.Register(rpc.FunctionConfig[GetProfileInput, GetProfileOutput]{
        Namespace:    "user",
        FunctionName: "getProfile",
        ContentType:  "application/json",
        Metadata: rpc.FunctionMetadata{
            Description: "Fetches user profile by ID",
            Tags:        []string{"user", "profile"},
        },
        Middlewares: rpc.DefineMiddlewares(
            rpc.M("auth", map[string]any{"role": "user"}),
        ),
        Handler: func(ctx *rpc.Context, input GetProfileInput) (*GetProfileOutput, error) {
            user, ok := rpc.GetContext[*User](ctx, "auth.user")
            return &GetProfileOutput{Name: user.Name}, nil
        },
    })
}
```

#### `rpc.RegisterMiddleware`

Registers a reusable middleware:

```go
rpc.RegisterMiddleware(
    "auth",
    "Bearer token authentication middleware",
    func(ctx *rpc.Context, options map[string]any) error {
        token := ctx.Header("Authorization")
        if token == "" {
            return rpc.NewError(rpc.ErrUnauthorized, "Missing Authorization header")
        }
        user, err := verifyToken(token)
        if err != nil {
            return rpc.NewError(rpc.ErrUnauthorized, "Invalid token")
        }
        ctx.Set("auth.user", user)
        return nil
    },
    schema.FromStruct(AuthOptions{}),
    schema.FromStruct(AuthHeaders{}),
)
```

---

### 6. Context Container (`*rpc.Context`)

| Method | Signature | Description |
| :--- | :--- | :--- |
| `Set` | `(key string, value any)` | Stores a value in the request context container. |
| `Get` | `(key string) (any, bool)` | Retrieves a value from context. |
| `MustGet` | `(key string) any` | Retrieves a value or panics if not present. |
| `Header` | `(name string) string` | Case-insensitive header retrieval. |
| `Headers` | `() map[string]string` | Returns a copy of all incoming HTTP headers. |
| `ClientIP` | `() string` | Returns the resolved client IP (respecting `TrustProxy`). |
| `RequestID`| `() string` | Returns unique request correlation ID. |
| `TrackFile`| `(f *UploadedFile)` | Registers a file for automatic cleanup at the end of the request. |
| `Cleanup` | `()` | Deletes all tracked temporary upload files. |

#### Generic Helper: `rpc.GetContext[T](ctx *Context, key string) (T, bool)`
Type-safe retrieval of stored context values without manual type assertion boilerplate.

---

### 7. Errors & Protocol

#### `rpc.NewError(code, message string, details ...any) *AppError`
Creates a standardized RPC error that serializes into the standard RPC error envelope:

```json
{
  "status": "error",
  "code": "validation_error",
  "message": "Field 'email' is invalid",
  "meta": {
    "requestId": "req_123",
    "durationMs": 1.2
  }
}
```

#### Standard Error Codes:
- `rpc.ErrInvalidBody` (`"invalid_body"`)
- `rpc.ErrNotFound` (`"not_found"`)
- `rpc.ErrValidation` (`"validation_error"`)
- `rpc.ErrUnauthorized` (`"unauthorized"`)
- `rpc.ErrForbidden` (`"forbidden"`)
- `rpc.ErrInternal` (`"internal_error"`)
- `rpc.ErrTooManyRequests` (`"rate_limit_exceeded"`)
- `rpc.ErrPayloadTooLarge` (`"payload_too_large"`)

---

### 8. File Uploads (`*rpc.UploadedFile`)

```go
type UploadedFile struct {
    Filename string
    Size     int64
    MimeType string
    TempPath string
}

func (f *UploadedFile) ReadBytes() ([]byte, error)
func (f *UploadedFile) SaveTo(destPath string) error
func (f *UploadedFile) Cleanup() error
```

---

## 📦 Package `pkg/schema`

Schema validation and TypeBox-compatible JSON schema contract generator.

| Function | Signature | Description |
| :--- | :--- | :--- |
| `FromStruct` | `(v any) *SchemaRule` | Auto-infers TypeBox-compatible schema from Go struct types and tags. |
| `Object` | `(props map[string]*SchemaRule) *SchemaRule` | Creates an object schema definition. |
| `String` | `() *SchemaRule` | String schema rule. |
| `Number` | `() *SchemaRule` | Float schema rule. |
| `Integer`| `() *SchemaRule` | Integer schema rule. |
| `Boolean`| `() *SchemaRule` | Boolean schema rule. |
| `Array`  | `(items *SchemaRule) *SchemaRule` | Array schema rule. |
| `Optional`| `(rule *SchemaRule) *SchemaRule` | Marks a property as optional. |
| `Binary` | `() *SchemaRule` | Marks a property as binary file (`format: "binary"`). |
| `Validate`| `(rule *SchemaRule, data any, path string) []ValidationErrorDetail` | Validates arbitrary data against a schema rule. |

---

## 📦 Package `pkg/utils/storage`

Unified file storage abstraction supporting physical disk storage and Amazon S3.

```go
type StorageProvider interface {
    Save(file *rpc.UploadedFile, relativePath string) (string, error)
    Delete(relativePath string) error
    GetURL(relativePath string) string
}

// Disk Storage:
diskStorage := storage.NewDiskStorage("./uploads", "/uploads")

// S3 Storage:
s3Storage := storage.NewS3Storage(storage.S3Config{
    Bucket: "my-bucket",
    Region: "us-east-1",
})
```

---

## 📦 Package `pkg/nodebridge`

Handles high-performance inter-process communication (IPC) with Node.js and Next.js runtimes.

| Function | Signature | Description |
| :--- | :--- | :--- |
| `ServeUnixSocket` | `(app *CoreRPC, socketPath string) error` | Runs a Unix Domain Socket IPC server for local process calling. |
| `ServeStdio` | `(app *CoreRPC) error` | Runs a JSON-lines Stdio IPC stream for child-process orchestration. |
| `GenerateTypeScriptDeclarations` | `(functions map[string]*FunctionDefinition) string` | Generates a strictly typed `crpc.d.ts` declaration file. |
