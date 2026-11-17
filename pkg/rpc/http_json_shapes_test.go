package rpc_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CellsWeb/crpc-go/pkg/rpc"
	"github.com/CellsWeb/crpc-go/pkg/schema"
	"github.com/valyala/fasthttp"
)

func TestHTTPJSONPayloadShapes(t *testing.T) {
	fixture, err := os.ReadFile("testdata/http-json-shapes.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name, Function, Namespace, Authorization, Code, Raw string
		OmitData                                            bool
		Data, Expected                                      json.RawMessage
		Headers                                             map[string]string
	}
	if err := json.Unmarshal(fixture, &cases); err != nil {
		t.Fatal(err)
	}
	for _, native := range []bool{true, false} {
		name := "fasthttp"
		if !native {
			name = "net-http"
		}
		t.Run(name, func(t *testing.T) {
			app, err := rpc.New(rpc.Config{RpcPath: "/rpc", CORS: &rpc.CorsConfig{AllowOrigins: []string{"*"}}})
			if err != nil {
				t.Fatal(err)
			}
			var middlewareCalls, handlerCalls atomic.Int32
			app.RegisterMiddleware("jsonShapeGuard", "Auth fixture", func(ctx *rpc.Context, headers map[string]string, options map[string]any) error {
				middlewareCalls.Add(1)
				if headers["authorization"] != "Bearer allowed" {
					return rpc.NewError(rpc.ErrUnauthorized, "Denied")
				}
				ctx.Set("actor", options["actor"])
				return nil
			}, nil, nil)
			middlewares := rpc.DefineMiddlewares(rpc.M("jsonShapeGuard", map[string]any{"actor": "fixture"}))
			echo := func(ctx *rpc.Context, input any) (any, error) {
				handlerCalls.Add(1)
				if actor, _ := ctx.Get("actor"); actor != "fixture" {
					return nil, fmt.Errorf("middleware context lost")
				}
				return map[string]any{"received": input}, nil
			}
			for _, fn := range []struct {
				name string
				rule *schema.SchemaRule
			}{
				{"echo", nil}, {"nullable", schema.Optional(schema.String())}, {"object", schema.Object(map[string]*schema.SchemaRule{})},
			} {
				app.RegisterFunction(&rpc.FunctionDefinition{Namespace: "jsonShapes", FunctionName: fn.name, InputSchema: fn.rule, Middlewares: middlewares, Handler: echo})
			}
			type typedOutput struct {
				Received *string `json:"received"`
			}
			app.RegisterFunction(rpc.Register(rpc.FunctionConfig[*string, typedOutput]{Namespace: "jsonShapes", FunctionName: "typedNullable",
				InputSchema: schema.Optional(schema.String()), OutputSchema: schema.Object(map[string]*schema.SchemaRule{"received": schema.Optional(schema.String())}), Middlewares: middlewares,
				Handler: func(ctx *rpc.Context, input *string) (*typedOutput, error) {
					if _, err := echo(ctx, input); err != nil {
						return nil, err
					}
					return &typedOutput{Received: input}, nil
				},
			}))
			var base string
			if native {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				server := &fasthttp.Server{Handler: app.Handler()}
				done := make(chan error, 1)
				go func() { done <- server.Serve(listener) }()
				t.Cleanup(func() {
					_ = server.Shutdown()
					if err := <-done; err != nil {
						t.Error(err)
					}
				})
				base = "http://" + listener.Addr().String()
			} else {
				server := httptest.NewServer(app.NetHTTPHandler())
				t.Cleanup(server.Close)
				base = server.URL
			}
			client := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
			t.Cleanup(client.CloseIdleConnections)
			for _, tc := range cases {
				t.Run(tc.Name, func(t *testing.T) {
					namespace, function := tc.Namespace, tc.Function
					if namespace == "" {
						namespace = "jsonShapes"
					}
					if function == "" {
						function = "echo"
					}
					payload := map[string]any{"namespace": namespace, "functionName": function}
					if !tc.OmitData {
						payload["data"] = tc.Data
					}
					body, err := json.Marshal(payload)
					if err != nil {
						t.Fatal(err)
					}
					if tc.Raw != "" {
						body = []byte(tc.Raw)
					}
					request, err := http.NewRequest("POST", base+"/rpc", bytes.NewReader(body))
					if err != nil {
						t.Fatal(err)
					}
					request.Header.Set("Content-Type", "application/json")
					auth := tc.Authorization
					if auth == "" {
						auth = "Bearer allowed"
					}
					request.Header.Set("Authorization", auth)
					for key, value := range tc.Headers {
						request.Header.Set(key, value)
					}
					beforeMiddleware, beforeHandler := middlewareCalls.Load(), handlerCalls.Load()
					response, err := client.Do(request)
					if err != nil {
						t.Fatal(err)
					}
					responseBody, err := io.ReadAll(response.Body)
					_ = response.Body.Close()
					if err != nil {
						t.Fatal(err)
					}
					var envelope struct {
						Status, Code string
						Data         struct{ Received any }
						Details      []rpc.ValidationErrorDetail
					}
					if err := json.Unmarshal(responseBody, &envelope); err != nil {
						t.Fatal(err)
					}
					if response.StatusCode != http.StatusOK {
						t.Fatalf("HTTP status changed: %d %s", response.StatusCode, responseBody)
					}
					if tc.Code != "" {
						if envelope.Status != "error" || envelope.Code != tc.Code {
							t.Fatalf("want %s: %s", tc.Code, responseBody)
						}
						if handlerCalls.Load() != beforeHandler {
							t.Error("failed call executed handler")
						}
						if tc.Code == "validation_error" && (len(envelope.Details) == 0 || envelope.Details[0].Path != "/") {
							t.Errorf("validation detail lost: %s", responseBody)
						}
					} else {
						var expected any
						if err := json.Unmarshal(tc.Expected, &expected); err != nil {
							t.Fatal(err)
						}
						if envelope.Status != "ok" || !reflect.DeepEqual(envelope.Data.Received, expected) {
							t.Errorf("received %#v, want %#v: %s", envelope.Data.Received, expected, responseBody)
						}
						if handlerCalls.Load() != beforeHandler+1 {
							t.Error("expected one handler call")
						}
					}
					wantMiddleware := int32(1)
					if tc.Code == "invalid_body" || tc.Code == "function_not_found" {
						wantMiddleware = 0
					}
					if middlewareCalls.Load() != beforeMiddleware+wantMiddleware {
						t.Error("middleware ordering/count changed")
					}
					if tc.Code != "" {
						t.Logf("HTTP 200 %s", responseBody)
					}
				})
			}
		})
	}
}
