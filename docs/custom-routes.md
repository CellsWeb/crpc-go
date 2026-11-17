# Custom HTTP routes

Register custom handlers with `func(*rpc.HTTPContext) error`, not raw fasthttp
callbacks. Custom routes have their own statuses/auth responsibilities; the
uniform RPC application-error convention does not force them to return200.

<!-- compile-go: cmd/http/main.go -->
```go
package main

import (
    "log"
    _ "docsexamples/functions/hello"
    "github.com/CellsWeb/crpc-go/pkg/rpc"
)
func main() {
    app, err := rpc.New(rpc.Config{RpcPath:"/rpc", Host:"127.0.0.1", Port:4000,
        DocsEnabled:false, CORS:&rpc.CorsConfig{AllowOrigins:[]string{"http://localhost:3000"}}})
    if err != nil { log.Fatal(err) }
    app.UseGlobalRegistry()
    app.GET("/health", func(c *rpc.HTTPContext) error { return c.SendString("OK") })
    app.POST("/example", func(c *rpc.HTTPContext) error { return c.Status(202).JSON(map[string]any{"accepted":true}) })
    if err := app.ListenAndServe(); err != nil { log.Fatal(err) }
}
```

HTTPContext supplies Method, Path, Header, Query, Body, SetHeader, Status, JSON,
SendString and Raw. Its Call method invokes registered RPCs in-process; validate
custom-route credentials/signatures first and forward relevant credentials. A
custom webhook handler does not become authenticated simply because it calls RPC.
