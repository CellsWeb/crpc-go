package rpc_test

import (
	"testing"

	"github.com/CellsWeb/crpc-go/pkg/rpc"
	"github.com/CellsWeb/crpc-go/pkg/schema"
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
