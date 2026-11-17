# Middleware and authentication

Go middleware receives `(*rpc.Context, headers, options)` and returns an error
to stop processing. It runs before input validation and the handler. Store
verified application state under a middleware-specific context key.

<!-- compile-go: middlewares/auth/run.go -->
```go
package auth

import (
    "strings"
    "github.com/CellsWeb/crpc-go/pkg/rpc"
    "github.com/CellsWeb/crpc-go/pkg/schema"
)
type Session struct { UserID, Role string }
// Application integration point; this example denies until it is implemented.
var VerifyToken = func(_ string) (Session, error) {
    return Session{}, rpc.NewError(rpc.ErrUnauthorized, "Configure credential verification")
}
func Run(ctx *rpc.Context, headers map[string]string, options map[string]any) error {
    raw := headers["authorization"]
    if !strings.HasPrefix(raw, "Bearer ") { return rpc.NewError(rpc.ErrUnauthorized, "Bearer credentials required") }
    session, err := VerifyToken(strings.TrimPrefix(raw, "Bearer "))
    if err != nil { return err }
    if session.UserID == "" { return rpc.NewError(rpc.ErrUnauthorized, "Invalid credentials") }
    role, _ := options["role"].(string)
    if role != "" && session.Role != role { return rpc.NewError(rpc.ErrForbidden, "Insufficient role") }
    ctx.Set("auth", session)
    return nil
}
func init() {
    rpc.RegisterMiddleware("auth", "Verify a bearer session and role", Run,
        schema.Object(map[string]*schema.SchemaRule{"role": schema.Optional(schema.String())}),
        schema.Object(map[string]*schema.SchemaRule{"authorization": schema.String().Describe("Bearer credentials")}),
    )
}
```

These option/header schemas are native documentation metadata. Go does not gain
automatic middleware option/header validation from them; the application must
validate its configuration and credentials. Node's middleware schema checks are
different implementation semantics, while auth responsibility is shared.

Consume state with `rpc.GetContext[auth.Session](ctx,"auth")` after the matching
middleware establishes it. Middleware namespacing organizes server-owned state;
it is not a mechanism for accepting identity from caller JSON.

There is no supported remote middleware-skip/context-injection authentication
workflow. Legacy Node IPC CallOptions fields named skipMiddlewares/context are
retained/transmitted for compatibility, but the Go bridge ignores them and always
uses the middleware pipeline. Forward credentials through headers instead.

Generated middleware is an editable no-op stub, not production authentication.
Docs show structural options/header schemas, descriptions and required status;
they do not read live configured options/defaults. IsPublic is deprecated and
ignored; it cannot grant or deny execution.
