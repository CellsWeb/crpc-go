package rpc_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/CellsWeb/crpc-go/pkg/rpc"
	"github.com/CellsWeb/crpc-go/pkg/schema"
)

func TestCallInMemoryJSONPayloadShapes(t *testing.T) {
	app, err := rpc.New(rpc.Config{RpcPath: "/rpc", CORS: &rpc.CorsConfig{AllowOrigins: []string{"*"}}})
	if err != nil {
		t.Fatal(err)
	}
	var middlewareCalls, handlerCalls int
	var received any
	app.RegisterMiddleware("shapeAuth", "Enforces auth", func(ctx *rpc.Context, headers map[string]string, options map[string]any) error {
		middlewareCalls++
		if headers["authorization"] != "Bearer valid" {
			return rpc.NewError(rpc.ErrUnauthorized, "Invalid token")
		}
		ctx.Set("actor", options["actor"])
		return nil
	}, nil, nil)
	app.RegisterFunction(&rpc.FunctionDefinition{
		Namespace: "shapes", FunctionName: "echo",
		Middlewares: rpc.DefineMiddlewares(rpc.M("shapeAuth", map[string]any{"actor": "reader"})),
		Handler: func(ctx *rpc.Context, input any) (any, error) {
			handlerCalls++
			if actor, _ := ctx.Get("actor"); actor != "reader" || ctx.ClientIP() != "127.0.0.7" {
				return nil, fmt.Errorf("middleware context or client IP was lost")
			}
			received = input
			// Preserve a null input in an explicit field without changing response omission rules.
			return map[string]any{"received": input}, nil
		},
	})
	opts := rpc.CallOptions{Headers: map[string]string{"authorization": "Bearer valid"}, ClientIP: "127.0.0.7"}
	for _, tc := range []struct{ name, data string }{
		{"object", `{"items":["item1",null],"enabled":false}`},
		{"empty_object", `{}`},
		{"array", `["item1","item2"]`},
		{"empty_array", `[]`},
		{"mixed_array", `["item1",2,false,null,[],{}]`},
		{"string", `"hello"`},
		{"empty_string", `""`},
		{"number", `12.5`},
		{"zero", `0`},
		{"true", `true`},
		{"false", `false`},
		{"null", `null`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var input any
			if err := json.Unmarshal([]byte(tc.data), &input); err != nil {
				t.Fatal(err)
			}
			beforeMiddleware, beforeHandler := middlewareCalls, handlerCalls
			var output struct {
				Received any `json:"received"`
			}
			if err := app.CallInMemory("shapes.echo", input, &output, opts); err != nil {
				t.Fatalf("data=%s: %v", tc.data, err)
			}
			if !reflect.DeepEqual(received, input) || !reflect.DeepEqual(output.Received, input) {
				t.Fatalf("data=%s handler=%#v (%T) output=%#v", tc.data, received, received, output.Received)
			}
			if middlewareCalls != beforeMiddleware+1 || handlerCalls != beforeHandler+1 {
				t.Fatal("expected one middleware and handler call")
			}
			t.Logf("data=%s handlerType=%T preserved", tc.data, received)
		})
	}
	for _, input := range []any{([]string)(nil), (map[string]any)(nil)} {
		t.Run(fmt.Sprintf("typed_null_%T", input), func(t *testing.T) {
			if err := app.CallInMemory("shapes.echo", input, nil, opts); err != nil {
				t.Fatal(err)
			}
			if received != nil {
				t.Fatalf("expected JSON null to reach handler as nil, got %#v (%T)", received, received)
			}
		})
	}
}

type callerShapeOutput[T any] struct {
	Received T `json:"received"`
}

func registerCallerShape[T any](app *rpc.CoreRPC, name string, rule *schema.SchemaRule, handlerCalls *int) {
	app.RegisterFunction(rpc.Register(rpc.FunctionConfig[T, callerShapeOutput[T]]{
		Namespace: "typedShapes", FunctionName: name,
		InputSchema: rule, OutputSchema: schema.Object(map[string]*schema.SchemaRule{"received": schema.Optional(rule)}),
		Middlewares: rpc.DefineMiddlewares(rpc.M("typedAuth", nil)),
		Handler: func(ctx *rpc.Context, input T) (*callerShapeOutput[T], error) {
			*handlerCalls++
			if actor, _ := ctx.Get("actor"); actor != "reader" {
				return nil, fmt.Errorf("typed handler lost middleware context")
			}
			return &callerShapeOutput[T]{Received: input}, nil
		},
	}))
}

func TestCallInMemoryTypedPayloadsAndValidation(t *testing.T) {
	app, err := rpc.New(rpc.Config{RpcPath: "/rpc", CORS: &rpc.CorsConfig{AllowOrigins: []string{"*"}}})
	if err != nil {
		t.Fatal(err)
	}
	var handlerCalls int
	app.RegisterMiddleware("typedAuth", "Enforces auth", func(ctx *rpc.Context, headers map[string]string, _ map[string]any) error {
		if headers["authorization"] != "Bearer valid" {
			return rpc.NewError(rpc.ErrUnauthorized, "Invalid token")
		}
		ctx.Set("actor", "reader")
		return nil
	}, nil, nil)
	type objectInput struct {
		Msg string `json:"msg"`
	}
	registerCallerShape[objectInput](app, "object", schema.FromStruct(objectInput{}), &handlerCalls)
	registerCallerShape[[]string](app, "array", schema.Array(schema.String()), &handlerCalls)
	registerCallerShape[string](app, "string", schema.String(), &handlerCalls)
	registerCallerShape[int](app, "integer", schema.Integer(), &handlerCalls)
	registerCallerShape[float64](app, "number", schema.Number(), &handlerCalls)
	registerCallerShape[bool](app, "boolean", schema.Boolean(), &handlerCalls)
	registerCallerShape[*string](app, "null", schema.Optional(schema.String()), &handlerCalls)
	opts := rpc.CallOptions{Headers: map[string]string{"authorization": "Bearer valid"}}
	for _, tc := range []struct {
		name  string
		input any
	}{
		{"object", objectInput{Msg: "hello"}}, {"array", []string{"item1", "item2"}},
		{"array", []string{}}, {"string", "hello"}, {"string", ""},
		{"integer", 7}, {"number", 12.5}, {"integer", 0},
		{"boolean", true}, {"boolean", false}, {"null", nil},
	} {
		t.Run(fmt.Sprintf("%s_%v", tc.name, tc.input), func(t *testing.T) {
			before := handlerCalls
			var output struct {
				Received json.RawMessage `json:"received"`
			}
			// NewTestCaller.Call delegates to the same production CallInMemory path.
			caller := rpc.NewTestCaller(app, rpc.TestOptions{Headers: opts.Headers})
			if err := caller.Call("typedShapes."+tc.name, tc.input, &output); err != nil {
				t.Fatal(err)
			}
			want, err := json.Marshal(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			if string(output.Received) != string(want) || handlerCalls != before+1 {
				t.Fatalf("want=%s got=%s handlerCalls=%d", want, output.Received, handlerCalls-before)
			}
		})
	}
	for _, tc := range []struct {
		name    string
		input   any
		options rpc.CallOptions
		code    string
	}{
		{"unauthorized", []string{"item1"}, rpc.CallOptions{}, rpc.ErrUnauthorized},
		{"wrong_shape", "item1", opts, rpc.ErrValidation},
		{"wrong_element", []int{1}, opts, rpc.ErrValidation},
		{"required_null", nil, opts, rpc.ErrValidation},
		{"non_json", make(chan int), opts, rpc.ErrInvalidBody},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := handlerCalls
			err := app.CallInMemory("typedShapes.array", tc.input, nil, tc.options)
			var appErr *rpc.AppError
			if !errors.As(err, &appErr) || appErr.Code != tc.code || handlerCalls != before {
				t.Fatalf("want=%s got=%v handlerCalls=%d", tc.code, err, handlerCalls-before)
			}
		})
	}
	t.Run("output_validation", func(t *testing.T) {
		app.RegisterFunction(&rpc.FunctionDefinition{
			Namespace: "typedShapes", FunctionName: "badOutput", InputSchema: schema.Array(schema.String()), OutputSchema: schema.String(),
			Handler: func(*rpc.Context, any) (any, error) { return 42, nil },
		})
		err := app.CallInMemory("typedShapes.badOutput", []string{"item1"}, nil)
		var appErr *rpc.AppError
		if !errors.As(err, &appErr) || appErr.Code != rpc.ErrInternal {
			t.Fatalf("expected existing output validation error, got %v", err)
		}
	})
}
