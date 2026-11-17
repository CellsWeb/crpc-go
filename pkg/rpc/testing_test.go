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

type rawCallerState struct {
	middlewares, handlers int
	received              any
	requestID, clientIP   string
}

func rawCallerApp(t *testing.T) (*rpc.CoreRPC, *rawCallerState, rpc.TestOptions) {
	t.Helper()
	app, err := rpc.New(rpc.Config{RpcPath: "/rpc", CORS: &rpc.CorsConfig{AllowOrigins: []string{"*"}}})
	if err != nil {
		t.Fatal(err)
	}
	state := new(rawCallerState)
	app.RegisterMiddleware("rawAuth", "Authorization fixture", func(ctx *rpc.Context, headers map[string]string, options map[string]any) error {
		state.middlewares++
		if headers["authorization"] != "Bearer valid" {
			return &rpc.AppError{Code: rpc.ErrUnauthorized, Message: "Denied", Details: []rpc.ValidationErrorDetail{{Path: "/authorization", Message: "Invalid token"}}}
		}
		ctx.Set("actor", options["actor"])
		return nil
	}, nil, nil)
	return app, state, rpc.TestOptions{Headers: map[string]string{"authorization": "Bearer valid"}, ClientIP: "127.0.0.8"}
}

func (state *rawCallerState) capture(ctx *rpc.Context, input any) (any, error) {
	state.handlers++
	state.received, state.requestID, state.clientIP = input, ctx.RequestID(), ctx.ClientIP()
	if actor, _ := ctx.Get("actor"); actor != "reader" {
		return nil, fmt.Errorf("middleware context lost: %v", actor)
	}
	return map[string]any{"received": input}, nil
}

func rawAuth() []rpc.MiddlewareAttachment {
	return rpc.DefineMiddlewares(rpc.M("rawAuth", map[string]any{"actor": "reader"}))
}

func rawMeta(t *testing.T, response *rpc.ResponseEnvelope) {
	t.Helper()
	if response == nil || response.Meta.RequestID == "" || response.Meta.DurationMs < 0 {
		t.Fatalf("missing/invalid envelope metadata: %+v", response)
	}
}

func normalizedJSON(t *testing.T, value any) any {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var normalized any
	if err := json.Unmarshal(encoded, &normalized); err != nil {
		t.Fatal(err)
	}
	return normalized
}

type rawArrayMarshaler struct{}

func (rawArrayMarshaler) MarshalJSON() ([]byte, error) { return []byte(`[["custom"],true]`), nil }

func TestCallRawJSONShapeParity(t *testing.T) {
	app, state, opts := rawCallerApp(t)
	app.RegisterFunction(&rpc.FunctionDefinition{Namespace: "rawShapes", FunctionName: "echo", Middlewares: rawAuth(), Handler: state.capture})
	caller := rpc.NewTestCaller(app, opts)
	cases := []struct {
		name  string
		input any
	}{}
	for _, fixture := range []struct{ name, json string }{
		{"object", `{"name":"item","enabled":false}`}, {"empty_object", `{}`},
		{"array", `["a","b"]`}, {"empty_array", `[]`}, {"nested_arrays", `[[1,2],[],[null,{"x":true}]]`},
		{"mixed_array", `["a",2,false,null,[],{}]`}, {"string", `"hello"`}, {"empty_string", `""`},
		{"number", `12.5`}, {"zero", `0`}, {"true", `true`}, {"false", `false`}, {"null", `null`},
	} {
		var input any
		if err := json.Unmarshal([]byte(fixture.json), &input); err != nil {
			t.Fatal(err)
		}
		cases = append(cases, struct {
			name  string
			input any
		}{fixture.name, input})
	}
	cases = append(cases, []struct {
		name  string
		input any
	}{{"nil_map", (map[string]any)(nil)}, {"nil_slice", ([]string)(nil)}, {"nil_pointer", (*string)(nil)}, {"raw_json_null", json.RawMessage(`null`)}, {"custom_marshaler_array", rawArrayMarshaler{}}}...)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := normalizedJSON(t, tc.input)
			beforeMiddleware, beforeHandler := state.middlewares, state.handlers
			response, err := caller.CallRaw("rawShapes.echo", tc.input)
			if err != nil {
				t.Fatalf("CallRaw rejected valid JSON shape: %v", err)
			}
			rawMeta(t, response)
			if response.Status != "ok" || !reflect.DeepEqual(state.received, want) || response.Meta.RequestID != state.requestID || state.clientIP != opts.ClientIP {
				t.Fatalf("CallRaw shape/context mismatch: response=%+v input=%#v (%T), want=%#v", response, state.received, state.received, want)
			}
			var output any
			if err := app.CallInMemory("rawShapes.echo", tc.input, &output, rpc.CallOptions{Headers: opts.Headers, ClientIP: opts.ClientIP}); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(state.received, want) || !reflect.DeepEqual(normalizedJSON(t, response.Data), output) {
				t.Fatalf("CallRaw/CallInMemory disagree: raw=%#v memory=%#v handler=%#v", response.Data, output, state.received)
			}
			if state.middlewares != beforeMiddleware+2 || state.handlers != beforeHandler+2 {
				t.Fatal("expected exactly one middleware and handler invocation per caller")
			}
		})
	}
	t.Run("native_object_fast_path", func(t *testing.T) {
		marker := &struct{ Name string }{"native"}
		input := map[string]any{"count": int32(7), "marker": marker}
		app.RegisterFunction(&rpc.FunctionDefinition{Namespace: "rawShapes", FunctionName: "native", Middlewares: rawAuth(), Handler: func(ctx *rpc.Context, data any) (any, error) {
			object := data.(map[string]any)
			if object["marker"] != marker || reflect.TypeOf(object["count"]).Kind() != reflect.Int32 {
				return nil, fmt.Errorf("native map values were serialized")
			}
			object["observed"] = true
			return state.capture(ctx, data)
		}})
		response, err := caller.CallRaw("rawShapes.native", input)
		if err != nil || response.Status != "ok" || input["observed"] != true {
			t.Fatalf("CallRaw lost object fast path: %+v %v", response, err)
		}
		delete(input, "observed")
		if err := app.CallInMemory("rawShapes.native", input, nil, rpc.CallOptions{Headers: opts.Headers}); err != nil || input["observed"] != true {
			t.Fatalf("CallInMemory fast path changed: %v", err)
		}
	})
}

type rawTypedInput struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}
type rawTypedResult[T any] struct {
	Received T `json:"received"`
}

func registerRawTyped[T any](app *rpc.CoreRPC, state *rawCallerState, name string, rule *schema.SchemaRule) {
	app.RegisterFunction(rpc.Register(rpc.FunctionConfig[T, rawTypedResult[T]]{
		Namespace: "rawTyped", FunctionName: name, InputSchema: rule,
		OutputSchema: schema.Object(map[string]*schema.SchemaRule{"received": schema.Optional(rule)}), Middlewares: rawAuth(),
		Handler: func(ctx *rpc.Context, input T) (*rawTypedResult[T], error) {
			if _, err := state.capture(ctx, input); err != nil {
				return nil, err
			}
			return &rawTypedResult[T]{Received: input}, nil
		},
	}))
}

func TestCallRawTypedInputParity(t *testing.T) {
	app, state, opts := rawCallerApp(t)
	registerRawTyped[rawTypedInput](app, state, "object", schema.FromStruct(rawTypedInput{}))
	registerRawTyped[[]string](app, state, "array", schema.Array(schema.String()))
	registerRawTyped[[][]int](app, state, "nested", schema.Array(schema.Array(schema.Integer())))
	registerRawTyped[string](app, state, "string", schema.String())
	registerRawTyped[int](app, state, "integer", schema.Integer())
	registerRawTyped[float64](app, state, "number", schema.Number())
	registerRawTyped[bool](app, state, "boolean", schema.Boolean())
	registerRawTyped[*string](app, state, "nullable", schema.Optional(schema.String()))
	object := rawTypedInput{Name: "item", Count: 2}
	caller := rpc.NewTestCaller(app, opts)
	for _, tc := range []struct {
		name, function string
		input, want    any
	}{
		{"struct", "object", object, object}, {"struct_pointer", "object", &object, object},
		{"array", "array", []string{"a", "b"}, []string{"a", "b"}}, {"empty_array", "array", []string{}, []string{}},
		{"nested_arrays", "nested", [][]int{{1, 2}, {}, {3}}, [][]int{{1, 2}, {}, {3}}},
		{"string", "string", "hello", "hello"}, {"empty_string", "string", "", ""},
		{"integer", "integer", 7, 7}, {"zero", "integer", 0, 0}, {"number", "number", 12.5, 12.5},
		{"true", "boolean", true, true}, {"false", "boolean", false, false},
		{"null", "nullable", nil, (*string)(nil)}, {"typed_null", "nullable", (*string)(nil), (*string)(nil)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := state.handlers
			response, err := caller.CallRaw("rawTyped."+tc.function, tc.input)
			if err != nil {
				t.Fatalf("CallRaw rejected typed input: %v", err)
			}
			rawMeta(t, response)
			if response.Status != "ok" || !reflect.DeepEqual(state.received, tc.want) || reflect.TypeOf(response.Data).Kind() != reflect.Pointer {
				t.Fatalf("typed input/direct handler result changed: %+v received=%#v", response, state.received)
			}
			var output any
			if err := app.CallInMemory("rawTyped."+tc.function, tc.input, &output, rpc.CallOptions{Headers: opts.Headers, ClientIP: opts.ClientIP}); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(state.received, tc.want) || !reflect.DeepEqual(normalizedJSON(t, response.Data), output) || state.handlers != before+2 {
				t.Fatalf("typed caller disagreement: raw=%#v memory=%#v received=%#v", response.Data, output, state.received)
			}
		})
	}
}

func TestCallRawValidationAuthorizationAndOptions(t *testing.T) {
	app, state, opts := rawCallerApp(t)
	caller := rpc.NewTestCaller(app, opts)
	for _, tc := range []struct {
		name, path string
		input      any
		rule       *schema.SchemaRule
	}{
		{"wrong_array_shape", "/", "item", schema.Array(schema.String())},
		{"wrong_array_element", "/0", []int{1}, schema.Array(schema.String())},
		{"required_null", "/", nil, schema.Array(schema.String())},
		{"fractional_integer", "/", 1.5, schema.Integer()},
		{"boolean_string_not_coerced", "/", "false", schema.Boolean()},
		{"unknown_object_field", "/extra", map[string]any{"name": "item", "extra": true}, schema.Object(map[string]*schema.SchemaRule{"name": schema.String()})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app.RegisterFunction(&rpc.FunctionDefinition{Namespace: "rawInvalid", FunctionName: tc.name, InputSchema: tc.rule, Middlewares: rawAuth(), Handler: state.capture})
			beforeHandlers, beforeMiddlewares := state.handlers, state.middlewares
			response, err := caller.CallRaw("rawInvalid."+tc.name, tc.input)
			if err != nil {
				t.Fatalf("expected validation envelope, got Go decoding error: %v", err)
			}
			rawMeta(t, response)
			wantDetails := schema.Validate(tc.rule, normalizedJSON(t, tc.input), "")
			if response.Status != "error" || response.Code != rpc.ErrValidation || !reflect.DeepEqual(response.Details, wantDetails) || len(response.Details) != 1 || response.Details[0].Path != tc.path {
				t.Fatalf("incorrect validation envelope: %+v, want details=%+v", response, wantDetails)
			}
			var appErr *rpc.AppError
			err = app.CallInMemory("rawInvalid."+tc.name, tc.input, nil, rpc.CallOptions{Headers: opts.Headers})
			if !errors.As(err, &appErr) || appErr.Code != rpc.ErrValidation || state.handlers != beforeHandlers || state.middlewares != beforeMiddlewares+2 {
				t.Fatalf("validation must follow auth and precede handlers: err=%v handlers=%d middlewares=%d", err, state.handlers-beforeHandlers, state.middlewares-beforeMiddlewares)
			}
		})
	}
	app.RegisterFunction(&rpc.FunctionDefinition{Namespace: "rawAuth", FunctionName: "echo", Middlewares: rawAuth(), Handler: state.capture})
	for _, input := range []any{[]string{"a"}, make(chan int)} {
		t.Run(fmt.Sprintf("authorization_before_decode_%T", input), func(t *testing.T) {
			before := state.handlers
			response, err := rpc.NewTestCaller(app).CallRaw("rawAuth.echo", input)
			if err != nil || response.Status != "error" || response.Code != rpc.ErrUnauthorized || len(response.Details) != 1 {
				t.Fatalf("authorization envelope changed: %+v %v", response, err)
			}
			rawMeta(t, response)
			var appErr *rpc.AppError
			err = app.CallInMemory("rawAuth.echo", input, nil)
			if !errors.As(err, &appErr) || appErr.Code != rpc.ErrUnauthorized || state.handlers != before {
				t.Fatalf("authorization bypass/order changed: %v", err)
			}
		})
	}
	t.Run("custom_options_preserve_defaults_and_context", func(t *testing.T) {
		defaults := rpc.TestOptions{Headers: map[string]string{"authorization": "bad"}, ClientIP: "127.0.0.1"}
		customCaller := rpc.NewTestCaller(app, defaults)
		response, err := customCaller.CallRaw("rawAuth.echo", []string{"a"}, rpc.TestOptions{Headers: opts.Headers, ClientIP: "127.0.0.9"})
		if err != nil || response.Status != "ok" || state.clientIP != "127.0.0.9" || state.requestID != response.Meta.RequestID {
			t.Fatalf("custom options lost: %+v %v ip=%s", response, err, state.clientIP)
		}
		response, err = customCaller.CallRaw("rawAuth.echo", []string{"a"})
		if err != nil || response.Code != rpc.ErrUnauthorized || defaults.Headers["authorization"] != "bad" {
			t.Fatalf("per-call override mutated defaults: %+v %v", response, err)
		}
		response, err = caller.CallRaw("rawAuth.echo", []string{"a"}, rpc.TestOptions{ClientIP: "127.0.0.10"})
		if err != nil || response.Status != "ok" || state.clientIP != "127.0.0.10" {
			t.Fatalf("IP override lost default headers: %+v %v", response, err)
		}
	})
}

func TestCallRawErrorReturnConventions(t *testing.T) {
	app, state, opts := rawCallerApp(t)
	caller := rpc.NewTestCaller(app, opts)
	app.RegisterFunction(&rpc.FunctionDefinition{Namespace: "rawErrors", FunctionName: "echo", Middlewares: rawAuth(), Handler: state.capture})
	for _, input := range []any{make(chan int), json.RawMessage(`{`)} {
		t.Run(fmt.Sprintf("non_json_%T", input), func(t *testing.T) {
			before := state.handlers
			response, err := caller.CallRaw("rawErrors.echo", input)
			if err == nil || response != nil {
				t.Fatalf("expected existing direct encoding error/nil envelope: %+v %v", response, err)
			}
			var appErr *rpc.AppError
			if err := app.CallInMemory("rawErrors.echo", input, nil, rpc.CallOptions{Headers: opts.Headers}); !errors.As(err, &appErr) || appErr.Code != rpc.ErrInvalidBody || state.handlers != before {
				t.Fatalf("CallInMemory encoding convention changed: %v", err)
			}
		})
	}
	t.Run("not_found", func(t *testing.T) {
		response, err := caller.CallRaw("rawErrors.missing", []string{"a"})
		if err != nil || response.Status != "error" || response.Code != rpc.ErrNotFound {
			t.Fatalf("not-found envelope changed: %+v %v", response, err)
		}
		rawMeta(t, response)
	})
	t.Run("wrapped_handler_error", func(t *testing.T) {
		details := []rpc.ValidationErrorDetail{{Path: "/business", Message: "Rejected"}}
		app.RegisterFunction(&rpc.FunctionDefinition{Namespace: "rawErrors", FunctionName: "wrapped", Middlewares: rawAuth(), Handler: func(ctx *rpc.Context, input any) (any, error) {
			_, _ = state.capture(ctx, input)
			return nil, fmt.Errorf("wrapped: %w", rpc.NewValidationError("Rejected", details))
		}})
		response, err := caller.CallRaw("rawErrors.wrapped", []string{"a"})
		if err != nil || response.Status != "error" || response.Code != rpc.ErrValidation || response.Message != "Rejected" || !reflect.DeepEqual(response.Details, details) {
			t.Fatalf("handler error envelope changed: %+v %v", response, err)
		}
		rawMeta(t, response)
		var appErr *rpc.AppError
		if err := app.CallInMemory("rawErrors.wrapped", []string{"a"}, nil, rpc.CallOptions{Headers: opts.Headers}); !errors.As(err, &appErr) || appErr.Code != rpc.ErrValidation || !reflect.DeepEqual(appErr.Details, details) {
			t.Fatalf("handler error parity changed: %v", err)
		}
	})
}
