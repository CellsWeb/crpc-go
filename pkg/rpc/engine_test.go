package rpc_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/CellsWeb/crpc-go/pkg/rpc"
	"github.com/CellsWeb/crpc-go/pkg/schema"
	"github.com/valyala/fasthttp"
)

type ProfileInput struct {
	UserID string `json:"userId"`
}

type ProfileOutput struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func TestEngineExecution(t *testing.T) {
	app, err := rpc.New(rpc.Config{
		RpcPath: "/rpc",
		CORS: &rpc.CorsConfig{
			AllowOrigins: []string{"*"},
		},
	})
	if err != nil {
		t.Fatalf("Failed creating app: %v", err)
	}

	// Register Auth Middleware
	app.RegisterMiddleware("auth", "Requires valid token", func(ctx *rpc.Context, headers map[string]string, options map[string]any) error {
		auth := headers["authorization"]
		if auth != "Bearer valid_token" {
			return rpc.NewError(rpc.ErrUnauthorized, "Invalid auth token")
		}
		ctx.Set("user", "test_user_101")
		return nil
	}, nil, nil)

	// Register Function
	rpc.Register(rpc.FunctionConfig[ProfileInput, ProfileOutput]{
		Namespace:    "user",
		FunctionName: "getProfile",
		InputSchema:  schema.FromStruct(ProfileInput{}),
		OutputSchema: schema.FromStruct(ProfileOutput{}),
		Middlewares: rpc.DefineMiddlewares(
			rpc.M("auth", map[string]any{}),
		),
		Handler: func(ctx *rpc.Context, input ProfileInput) (*ProfileOutput, error) {
			user, _ := ctx.Get("user")
			if user != "test_user_101" {
				return nil, rpc.NewError(rpc.ErrForbidden, "User mismatch")
			}
			return &ProfileOutput{
				Name:  "Minhazur Rahman",
				Email: "minhaz@example.com",
			}, nil
		},
	})

	app.UseGlobalRegistry()

	caller := rpc.NewTestCaller(app, rpc.TestOptions{
		Headers: map[string]string{
			"authorization": "Bearer valid_token",
		},
	})

	// 1. Success Call
	var output ProfileOutput
	err = caller.Call("user.getProfile", ProfileInput{UserID: "usr_1"}, &output)
	if err != nil {
		t.Fatalf("Expected success, got %v", err)
	}
	if output.Name != "Minhazur Rahman" {
		t.Fatalf("Expected name 'Minhazur Rahman', got %q", output.Name)
	}

	// 2. Auth Failure (Fail-Closed)
	unauthCaller := rpc.NewTestCaller(app, rpc.TestOptions{
		Headers: map[string]string{
			"authorization": "Bearer bad_token",
		},
	})
	rawRes, err := unauthCaller.CallRaw("user.getProfile", ProfileInput{UserID: "usr_1"})
	if err != nil {
		t.Fatalf("Unexpected test error: %v", err)
	}
	if rawRes.Status != "error" || rawRes.Code != rpc.ErrUnauthorized {
		t.Fatalf("Expected error code 'unauthorized', got %q", rawRes.Code)
	}

	// 3. Validation Failure (Unknown fields)
	resWithExtra, err := caller.CallRaw("user.getProfile", map[string]any{
		"userId":  "usr_1",
		"unknown": "attack",
	})
	if err != nil {
		t.Fatalf("Unexpected test error: %v", err)
	}
	if resWithExtra.Status != "error" || resWithExtra.Code != rpc.ErrValidation {
		t.Fatalf("Expected validation error, got status=%s code=%s", resWithExtra.Status, resWithExtra.Code)
	}
}

type ItemsOutput struct {
	Items []string `json:"items"`
}

func TestHttpStatusCodesAndJsonV2(t *testing.T) {
	app, err := rpc.New(rpc.Config{
		RpcPath:            "/rpc",
		UseHttpStatusCodes: true, // Configurable HTTP Status Codes
		CORS: &rpc.CorsConfig{
			AllowOrigins: []string{"*"},
		},
	})
	if err != nil {
		t.Fatalf("Failed creating app: %v", err)
	}

	rpc.Register(rpc.FunctionConfig[ProfileInput, ItemsOutput]{
		Namespace:    "items",
		FunctionName: "list",
		InputSchema:  schema.FromStruct(ProfileInput{}),
		Handler: func(ctx *rpc.Context, input ProfileInput) (*ItemsOutput, error) {
			// Deliberately return nil slice to verify JSON v2 serializes as []
			return &ItemsOutput{
				Items: nil,
			}, nil
		},
	})
	app.UseGlobalRegistry()

	handler := app.Handler()

	// 1. Success with nil slice -> verify serialized as [] (JSON v2)
	var ctx fasthttp.RequestCtx
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetRequestURI("/rpc")
	ctx.Request.Header.SetContentType("application/json")
	ctx.Request.SetBody([]byte(`{"namespace":"items","functionName":"list","data":{"userId":"u1"}}`))
	handler(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", ctx.Response.StatusCode())
	}
	bodyStr := string(ctx.Response.Body())
	if !strings.Contains(bodyStr, `"items":[]`) {
		t.Fatalf("Expected JSON v2 to serialize nil slice as '[]', got: %s", bodyStr)
	}

	// 2. Not Found function with UseHttpStatusCodes: true -> HTTP 404
	var ctxNotFound fasthttp.RequestCtx
	ctxNotFound.Request.Header.SetMethod("POST")
	ctxNotFound.Request.SetRequestURI("/rpc")
	ctxNotFound.Request.Header.SetContentType("application/json")
	ctxNotFound.Request.SetBody([]byte(`{"namespace":"unknown","functionName":"none","data":{}}`))
	handler(&ctxNotFound)

	if ctxNotFound.Response.StatusCode() != fasthttp.StatusNotFound {
		t.Fatalf("Expected 404 Not Found, got %d", ctxNotFound.Response.StatusCode())
	}

	// 3. Duplicate key payload -> rejected by JSON v2
	var ctxDup fasthttp.RequestCtx
	ctxDup.Request.Header.SetMethod("POST")
	ctxDup.Request.SetRequestURI("/rpc")
	ctxDup.Request.Header.SetContentType("application/json")
	ctxDup.Request.SetBody([]byte(`{"namespace":"items","namespace":"attack","functionName":"list","data":{}}`))
	handler(&ctxDup)

	if ctxDup.Response.StatusCode() != fasthttp.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request for duplicate keys, got %d", ctxDup.Response.StatusCode())
	}
}

func TestCorsWildcardWithCredentialsRejection(t *testing.T) {
	// 1. Standalone "*" with credentials -> error
	cfg1 := rpc.Config{
		RpcPath: "/rpc",
		CORS: &rpc.CorsConfig{
			AllowOrigins:     []string{"*"},
			AllowCredentials: true,
		},
	}
	if err := cfg1.Validate(); err == nil {
		t.Fatalf("Expected error rejecting wildcard origin with credentials")
	}

	// 2. Mixed wildcard with credentials -> error
	cfg2 := rpc.Config{
		RpcPath: "/rpc",
		CORS: &rpc.CorsConfig{
			AllowOrigins:     []string{"*", "https://cellsweb.com"},
			AllowCredentials: true,
		},
	}
	if err := cfg2.Validate(); err == nil {
		t.Fatalf("Expected error rejecting mixed wildcard origin with credentials")
	}

	// 3. Explicit origin with credentials -> valid
	cfg3 := rpc.Config{
		RpcPath: "/rpc",
		CORS: &rpc.CorsConfig{
			AllowOrigins:     []string{"https://cellsweb.com"},
			AllowCredentials: true,
		},
	}
	if err := cfg3.Validate(); err != nil {
		t.Fatalf("Expected explicit origin with credentials to pass, got: %v", err)
	}
}

func TestHeaderBasedMultipartPreAuthRejection(t *testing.T) {
	app, err := rpc.New(rpc.Config{
		RpcPath: "/rpc",
		CORS:    &rpc.CorsConfig{AllowOrigins: []string{"*"}},
	})
	if err != nil {
		t.Fatalf("Failed creating app: %v", err)
	}

	app.RegisterMiddleware("secureUploadAuth", "Checks token", func(ctx *rpc.Context, headers map[string]string, options map[string]any) error {
		if headers["x-upload-secret"] != "supersecret" {
			return rpc.NewError(rpc.ErrUnauthorized, "Invalid upload secret")
		}
		return nil
	}, nil, nil)

	rpc.Register(rpc.FunctionConfig[ProfileInput, ProfileOutput]{
		Namespace:    "docUpload",
		FunctionName: "submit",
		InputSchema:  schema.FromStruct(ProfileInput{}),
		Middlewares:  rpc.DefineMiddlewares(rpc.M("secureUploadAuth", nil)),
		Handler: func(ctx *rpc.Context, input ProfileInput) (*ProfileOutput, error) {
			return &ProfileOutput{Name: "done", Email: "done@test.com"}, nil
		},
	})
	app.UseGlobalRegistry()

	handler := app.Handler()

	// Send request with X-CRPC-Namespace and X-CRPC-Function headers, but INVALID secret
	var ctx fasthttp.RequestCtx
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetRequestURI("/rpc")
	ctx.Request.Header.SetContentType("multipart/form-data; boundary=----WebKitFormBoundary7MA4YWxkTrZu0gW")
	ctx.Request.Header.Set("X-CRPC-Namespace", "docUpload")
	ctx.Request.Header.Set("X-CRPC-Function", "submit")
	ctx.Request.Header.Set("x-upload-secret", "wrong-secret")
	// Dummy unparseable body — proves the server NEVER touches the body when headers auth fails!
	ctx.Request.SetBody([]byte("THIS IS NOT VALID MULTIPART DATA"))

	handler(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("Expected uniform 200 OK status, got %d", ctx.Response.StatusCode())
	}

	bodyStr := string(ctx.Response.Body())
	if !strings.Contains(bodyStr, `"code":"unauthorized"`) {
		t.Fatalf("Expected unauthorized error code in response envelope, got: %s", bodyStr)
	}
}

func TestWrappedErrorUnwrapping(t *testing.T) {
	app, err := rpc.New(rpc.Config{
		RpcPath: "/rpc",
		CORS:    &rpc.CorsConfig{AllowOrigins: []string{"*"}},
	})
	if err != nil {
		t.Fatalf("Failed creating app: %v", err)
	}

	rpc.Register(rpc.FunctionConfig[ProfileInput, ProfileOutput]{
		Namespace:    "errorsTest",
		FunctionName: "wrapped",
		InputSchema:  schema.FromStruct(ProfileInput{}),
		Handler: func(ctx *rpc.Context, input ProfileInput) (*ProfileOutput, error) {
			domainErr := rpc.NewError(rpc.ErrConflict, "Resource conflict occurred")
			// Return wrapped error via %w
			return nil, fmt.Errorf("outer handler failed: %w", domainErr)
		},
	})
	app.UseGlobalRegistry()

	caller := rpc.NewTestCaller(app)
	res, err := caller.CallRaw("errorsTest.wrapped", ProfileInput{UserID: "u1"})
	if err != nil {
		t.Fatalf("Unexpected test error: %v", err)
	}
	if res.Status != "error" || res.Code != rpc.ErrConflict {
		t.Fatalf("Expected error code 'conflict' from wrapped error, got %q", res.Code)
	}
}

func TestTelemetryPanicIsolation(t *testing.T) {
	panicHookCalled := false

	app, err := rpc.New(rpc.Config{
		RpcPath: "/rpc",
		CORS:    &rpc.CorsConfig{AllowOrigins: []string{"*"}},
		TelemetryHooks: []rpc.TelemetryHook{
			func(event rpc.TelemetryEvent) {
				panicHookCalled = true
				panic("deliberate third-party telemetry crash")
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed creating app: %v", err)
	}

	rpc.Register(rpc.FunctionConfig[ProfileInput, ProfileOutput]{
		Namespace:    "telemetryTest",
		FunctionName: "ping",
		InputSchema:  schema.FromStruct(ProfileInput{}),
		Handler: func(ctx *rpc.Context, input ProfileInput) (*ProfileOutput, error) {
			return &ProfileOutput{Name: "pong", Email: "pong@test.com"}, nil
		},
	})
	app.UseGlobalRegistry()

	handler := app.Handler()
	var ctx fasthttp.RequestCtx
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetRequestURI("/rpc")
	ctx.Request.Header.SetContentType("application/json")
	ctx.Request.SetBody([]byte(`{"namespace":"telemetryTest","functionName":"ping","data":{"userId":"u1"}}`))
	handler(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("Expected 200 OK despite telemetry panic, got %d", ctx.Response.StatusCode())
	}
	if !panicHookCalled {
		t.Fatalf("Expected telemetry hook to be called")
	}
}
