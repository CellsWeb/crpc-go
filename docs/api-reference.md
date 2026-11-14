---
layout: default
title: Go API Reference
nav_order: 10
---

# Go API Reference

This page contains the comprehensive API reference for all exported packages, structs, interfaces, and functions in `github.com/CellsWeb/crpc-go`.

---

## 📦 Package `pkg/rpc`

### Server Initialization

#### `rpc.New(config Config) (*CoreRPC, error)`
Creates and strictly validates a new `CoreRPC` server instance.

```go
type Config struct {
    RpcPath     string      // Path for RPC POST endpoint (e.g. "/rpc")
    Port        int         // HTTP port (e.g. 4001)
    Host        string      // Bind host (e.g. "0.0.0.0")
    DocsEnabled bool        // Enable server-rendered docs
    DocsPath    string      // Path for docs (e.g. "/docs")
    TrustProxy  bool        // Respect X-Forwarded-For headers
    CORS        *CorsConfig // Mandatory CORS configuration
}
```

#### `app.UseGlobalRegistry() *CoreRPC`
Imports all functions and middlewares registered via package `init()`.

#### `app.ListenAndServe() error`
Binds to the configured host/port and starts the fasthttp event loop.

---

### Function & Middleware Registration

#### `rpc.Register[TInput, TOutput](cfg FunctionConfig[TInput, TOutput]) *FunctionDefinition`
Registers a type-safe RPC function. Schemas are automatically inferred if omitted.

```go
type FunctionConfig[TInput any, TOutput any] struct {
    Namespace    string
    FunctionName string
    ContentType  string // "application/json" or "multipart/form-data"
    Metadata     FunctionMetadata
    InputSchema  *schema.SchemaRule
    OutputSchema *schema.SchemaRule
    Middlewares  []MiddlewareAttachment
    Handler      func(ctx *Context, input TInput) (*TOutput, error)
}
```

#### `rpc.RegisterMiddleware(name, description string, fn MiddlewareFn, optSchema, headerSchema *schema.SchemaRule)`
Registers a reusable middleware into the registry.

#### `rpc.DefineMiddlewares(attachments ...MiddlewareAttachment) []MiddlewareAttachment`
Helper to build ordered middleware chains.

#### `rpc.M(name string, options map[string]any) MiddlewareAttachment`
Attaches a middleware with specific runtime options.

---

### Context Container (`*rpc.Context`)

| Method | Signature | Description |
| :--- | :--- | :--- |
| `Set` | `(key string, value any)` | Stores a value in request context container |
| `Get` | `(key string) (any, bool)` | Retrieves a value from context |
| `MustGet` | `(key string) any` | Retrieves value or panics if not present |
| `Header` | `(name string) string` | Case-insensitive header retrieval |
| `Headers` | `() map[string]string` | Returns copy of all incoming headers |
| `ClientIP` | `() string` | Returns client IP address |
| `RequestID` | `() string` | Returns unique request correlation ID |
| `TrackFile` | `(f *UploadedFile)` | Registers file for auto-cleanup |
| `Cleanup` | `()` | Deletes tracked temporary upload files |

#### Generic Helper: `rpc.GetContext[T](ctx *Context, key string) (T, bool)`
Type-safe retrieval of stored context values.

---

### Errors & Protocol

#### `rpc.NewError(code, message string, details ...any) *AppError`
Creates a standardized RPC error that maps to the HTTP 200 error envelope.

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

### File Uploads (`*rpc.UploadedFile`)

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

Used for runtime schema validation and contract generation.

| Function | Signature | Description |
| :--- | :--- | :--- |
| `FromStruct` | `(v any) *SchemaRule` | Auto-infers TypeBox-compatible schema from Go struct |
| `Object` | `(props map[string]*SchemaRule) *SchemaRule` | Creates an object schema |
| `String` | `() *SchemaRule` | String schema rule |
| `Number` | `() *SchemaRule` | Float schema rule |
| `Integer`| `() *SchemaRule` | Integer schema rule |
| `Boolean`| `() *SchemaRule` | Boolean schema rule |
| `Array`  | `(items *SchemaRule) *SchemaRule` | Array schema rule |
| `Optional` | `(rule *SchemaRule) *SchemaRule` | Marks property as optional |
| `Binary` | `() *SchemaRule` | Marks property as binary file (`format: "binary"`) |
| `Validate` | `(rule *SchemaRule, data any, path string) []ValidationErrorDetail` | Validates data against schema |

---

## 📦 Package `pkg/nodebridge`

Handles IPC communication with Node.js and Next.js applications.

| Function | Signature | Description |
| :--- | :--- | :--- |
| `ServeUnixSocket` | `(app *CoreRPC, socketPath string) error` | Runs persistent Unix Domain Socket IPC server |
| `ServeStdio` | `(app *CoreRPC) error` | Runs JSON-lines Stdio IPC stream for child processes |
| `GenerateTypeScriptDeclarations` | `(functions map[string]*FunctionDefinition) string` | Auto-generates `crpc.d.ts` declaration file |
