---
layout: default
title: Custom HTTP Routes & In-Process Calling
nav_order: 7
---

# Custom HTTP Routes & In-Process Calling

While CellsWeb RPC routes main API operations through `POST /rpc`, real-world applications frequently require custom HTTP endpoints for:
- Static media serving (`GET /uploads/*`)
- Third-party webhooks (`POST /webhooks/stripe`, `POST /webhooks/github`)
- Health probes (`GET /health`, `GET /ready`)
- OAuth redirection callbacks (`GET /auth/callback`)

CellsWeb RPC Go provides first-class support for registering custom HTTP routes directly on `CoreRPC`.

---

## 🛣️ Registering Custom HTTP Endpoints

You can register standard HTTP methods using `app.GET`, `app.POST`, `app.PUT`, and `app.DELETE`:

```go
package main

import (
	"github.com/CellsWeb/crpc-go/pkg/rpc"
)

func main() {
	app, _ := rpc.New(rpc.Config{
		RpcPath: "/rpc",
		Port:    4001,
		CORS:    &rpc.CorsConfig{AllowOrigins: []string{"*"}},
	})

	// 1. Simple Health Probe
	app.GET("/health", func(c *rpc.HTTPContext) error {
		return c.SendString("OK")
	})

	// 2. JSON Status Endpoint
	app.GET("/status", func(c *rpc.HTTPContext) error {
		return c.JSON(map[string]any{
			"status": "healthy",
			"uptime": 1284,
		})
	})
}
```

---

## 📥 Working with `rpc.HTTPContext`

The `HTTPContext` object provides clean, fasthttp-accelerated helper methods:

| Method | Description |
| :--- | :--- |
| `c.Path()` | Returns request URL path string |
| `c.Method()` | Returns HTTP method (`GET`, `POST`, etc.) |
| `c.Header(name)` | Retrieves request header (case-insensitive) |
| `c.SetHeader(key, val)` | Sets response header |
| `c.Query(key)` | Retrieves URL query parameter value |
| `c.Status(code)` | Sets HTTP status code (e.g. 200, 404, 500) |
| `c.JSON(data)` | Marshals data to JSON with `application/json` header |
| `c.SendString(str)` | Sends plaintext response |
| `c.Raw()` | Access to the raw `*fasthttp.RequestCtx` |

---

## ⚡ In-Process RPC Calling (`c.Call`)

A common requirement in webhooks or REST endpoints is to invoke existing RPC functions **without making an HTTP network request**.

You can invoke any registered RPC function directly via `c.Call()`:

```go
app.POST("/webhooks/stripe", func(c *rpc.HTTPContext) error {
	// Parse Stripe signature
	sig := c.Header("Stripe-Signature")
	if sig == "" {
		c.Status(400)
		return c.SendString("Missing signature")
	}

	// In-process call to 'billing.handleWebhook' RPC function
	var rpcResult map[string]any
	err := c.Call("billing.handleWebhook", map[string]any{
		"signature": sig,
		"payload":   string(c.Raw().PostBody()),
	}, &rpcResult)

	if err != nil {
		c.Status(500)
		return c.SendString(err.Error())
	}

	return c.JSON(rpcResult)
})
```

### Benefits of `c.Call`:
1. **Zero TCP/Loopback Overhead**: Dispatches directly in memory ($<5\mu s$).
2. **Context Inheritance**: In-flight request ID, IP, and headers are seamlessly preserved.
3. **Strict Validation**: Executes the same input schema validation and error reporting as external callers.

---

## 🖼️ Static Media Server Example

Serve uploaded files from `/uploads/*`:

```go
app.GET("/uploads/*", func(c *rpc.HTTPContext) error {
	path := c.Path()
	data, mimeType, err := storage.DefaultEngine.Serve(path)
	if err != nil {
		c.Status(404)
		return c.SendString("File not found")
	}
	c.SetHeader("Content-Type", mimeType)
	c.Raw().SetBody(data)
	return nil
})
```
