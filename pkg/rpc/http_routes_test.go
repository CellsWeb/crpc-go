package rpc_test

import (
	"testing"

	"github.com/CellsWeb/crpc-go/pkg/rpc"
	"github.com/valyala/fasthttp"
)

func TestCustomHTTPRoutesAndInternalCall(t *testing.T) {
	app, err := rpc.New(rpc.Config{
		RpcPath: "/rpc",
		CORS:    &rpc.CorsConfig{AllowOrigins: []string{"*"}},
	})
	if err != nil {
		t.Fatalf("Failed creating app: %v", err)
	}

	rpc.Register(rpc.FunctionConfig[ProfileInput, ProfileOutput]{
		Namespace:    "user",
		FunctionName: "getInfo",
		Handler: func(ctx *rpc.Context, input ProfileInput) (*ProfileOutput, error) {
			return &ProfileOutput{
				Name:  "Internal " + input.UserID,
				Email: "internal@example.com",
			}, nil
		},
	})
	app.UseGlobalRegistry()

	// Register custom GET route
	app.GET("/health", func(c *rpc.HTTPContext) error {
		return c.SendString("OK")
	})

	// Register custom POST route that calls the RPC function in-memory
	app.POST("/webhook", func(c *rpc.HTTPContext) error {
		var output ProfileOutput
		err := c.Call("user.getInfo", ProfileInput{UserID: "webhook_user"}, &output)
		if err != nil {
			return err
		}
		return c.JSON(output)
	})

	handler := app.Handler()

	// 1. Test GET /health
	var ctx fasthttp.RequestCtx
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/health")
	handler(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("Expected status 200, got %d", ctx.Response.StatusCode())
	}
	if string(ctx.Response.Body()) != "OK" {
		t.Fatalf("Expected 'OK', got %q", string(ctx.Response.Body()))
	}

	// 2. Test POST /webhook calling RPC in-memory
	var ctxWebhook fasthttp.RequestCtx
	ctxWebhook.Request.Header.SetMethod("POST")
	ctxWebhook.Request.SetRequestURI("/webhook")
	handler(&ctxWebhook)

	if ctxWebhook.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("Expected status 200, got %d", ctxWebhook.Response.StatusCode())
	}
}
