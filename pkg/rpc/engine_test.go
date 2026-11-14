package rpc_test

import (
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
