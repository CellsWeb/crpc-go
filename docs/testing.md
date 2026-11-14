---
layout: default
title: In-Process Integration Testing
nav_order: 8
---

# In-Process Integration Testing

Testing microservices typically requires launching an HTTP server, binding to an ephemeral network port, and issuing slow network HTTP requests.

CellsWeb RPC Go eliminates network test overhead with **`rpc.NewTestCaller`**, executing RPC functions completely **in-process** through the full middleware, validation, and error pipeline.

---

## ⚡ Why In-Process Testing?

- **Zero Network Latency**: Tests run at CPU speed (over **50,000 test calls/sec**).
- **Zero Port Clashes**: No `EADDRINUSE` errors in parallel test runners (`go test -p 8`).
- **Complete Pipeline Coverage**: Executes exact middlewares, schemas, header checks, and error transformations.

---

## 🛠️ Basic Test Setup

```go
package user_test

import (
	"testing"
	"github.com/CellsWeb/crpc-go/pkg/rpc"
	_ "my-app/functions/user/getProfile"
	_ "my-app/middlewares/auth"
)

func setupTestApp(t *testing.T) *rpc.CoreRPC {
	app, err := rpc.New(rpc.Config{
		RpcPath: "/rpc",
		CORS:    &rpc.CorsConfig{AllowOrigins: []string{"*"}},
	})
	if err != nil {
		t.Fatalf("Failed creating test app: %v", err)
	}
	app.UseGlobalRegistry()
	return app
}
```

---

## 🧪 Testing with `caller.Call` (Unwrapped)

`caller.Call` decodes the response directly into your output struct or returns an error if `status === "error"`:

```go
func TestGetProfile_Success(t *testing.T) {
	app := setupTestApp(t)

	caller := rpc.NewTestCaller(app, rpc.TestOptions{
		Headers: map[string]string{
			"authorization": "Bearer valid_jwt_token",
		},
	})

	var profile struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}

	err := caller.Call("user.getProfile", map[string]any{"id": "usr_101"}, &profile)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if profile.ID != "usr_101" {
		t.Errorf("Expected user ID 'usr_101', got %s", profile.ID)
	}
}
```

---

## 🔍 Testing Error Boundaries with `caller.CallRaw`

When testing security restrictions, validation rules, or error codes, use `caller.CallRaw` to inspect the full response envelope without unwrapping:

```go
func TestGetProfile_Unauthorized(t *testing.T) {
	app := setupTestApp(t)

	// Caller without auth header
	caller := rpc.NewTestCaller(app, rpc.TestOptions{})

	raw := caller.CallRaw("user.getProfile", map[string]any{"id": "usr_101"})

	// 1. Verify envelope status
	if raw.Status != "error" {
		t.Fatalf("Expected status 'error', got %s", raw.Status)
	}

	// 2. Verify error code
	if raw.Code != rpc.ErrUnauthorized {
		t.Errorf("Expected code %s, got %s", rpc.ErrUnauthorized, raw.Code)
	}
}
```

---

## 🛡️ Testing Input Schema Validation

```go
func TestCreateUser_ValidationFailure(t *testing.T) {
	app := setupTestApp(t)
	caller := rpc.NewTestCaller(app, rpc.TestOptions{})

	// Send empty input that violates schema
	raw := caller.CallRaw("user.create", map[string]any{})

	if raw.Code != rpc.ErrValidation {
		t.Fatalf("Expected validation error, got %s", raw.Code)
	}

	if len(raw.Details) == 0 {
		t.Errorf("Expected field error details, got none")
	}
}
```
