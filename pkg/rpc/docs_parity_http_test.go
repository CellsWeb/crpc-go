package rpc

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CellsWeb/crpc-go/pkg/schema"
	"github.com/valyala/fasthttp"
)

func TestDocsParityLiveHTTPAndExecution(t *testing.T) {
	for _, native := range []bool{false, true} {
		name := "net-http"
		if native {
			name = "fasthttp"
		}
		t.Run(name, func(t *testing.T) {
			app, err := New(Config{RpcPath: "/rpc", DocsPath: "/reference/api", DocsEnabled: true, CORS: &CorsConfig{AllowOrigins: []string{"*"}}})
			if err != nil {
				t.Fatal(err)
			}
			var calls atomic.Int64
			options := schema.Object(map[string]*schema.SchemaRule{"token": schema.String().WithDefault("SCHEMA_SECRET_MUST_NOT_APPEAR")})
			headers := schema.Object(map[string]*schema.SchemaRule{"authorization": schema.String().Describe("Required bearer marker")})
			app.RegisterMiddleware("guard", "Middleware description marker", func(ctx *Context, headers map[string]string, options map[string]any) error {
				calls.Add(1)
				if headers["authorization"] != "Bearer allowed" {
					return NewError(ErrUnauthorized, "Denied")
				}
				// These schemas are metadata in Go, not newly introduced runtime validation.
				if options["token"] != 123 {
					return NewError(ErrInternal, "options mutated")
				}
				return nil
			}, options, headers)
			for _, legacy := range []bool{false, true} {
				function := "legacyFalse"
				if legacy {
					function = "legacyTrue"
				}
				app.RegisterFunction(&FunctionDefinition{Namespace: "catalog", FunctionName: function, ContentType: "application/json", Metadata: FunctionMetadata{IsPublic: legacy},
					Middlewares: []MiddlewareAttachment{M("guard", map[string]any{"token": 123, "private": "CONFIGURED_SECRET_MUST_NOT_APPEAR"})},
					Handler:     func(_ *Context, _ any) (any, error) { calls.Add(1); return map[string]any{"ok": true}, nil },
				})
			}
			var base string
			if native {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				server := &fasthttp.Server{Handler: app.Handler()}
				go func() { _ = server.Serve(listener) }()
				t.Cleanup(func() { _ = server.Shutdown() })
				base = "http://" + listener.Addr().String()
			} else {
				server := httptest.NewServer(app.NetHTTPHandler())
				t.Cleanup(server.Close)
				base = server.URL
			}
			client := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
			for _, path := range []string{"", "/catalog/legacyFalse", "/catalog/legacyTrue", "/middlewares/guard"} {
				response, err := client.Get(base + "/reference/api" + path + "?tab=schema")
				if err != nil {
					t.Fatal(err)
				}
				body, err := io.ReadAll(response.Body)
				_ = response.Body.Close()
				if err != nil || response.StatusCode != 200 {
					t.Fatalf("docs HTTP: %v, %d", err, response.StatusCode)
				}
				for _, value := range []string{">PUBLIC</span>", ">PRIVATE</span>", "CONFIGURED_SECRET_MUST_NOT_APPEAR", "SCHEMA_SECRET_MUST_NOT_APPEAR"} {
					if strings.Contains(string(body), value) {
						t.Errorf("docs HTTP exposed %q", value)
					}
				}
			}
			if calls.Load() != 0 {
				t.Fatal("rendering docs executed middleware/handler")
			}
			for _, function := range []string{"legacyFalse", "legacyTrue"} {
				for _, token := range []string{"Bearer allowed", "denied"} {
					request, _ := http.NewRequest("POST", base+"/rpc", strings.NewReader(`{"namespace":"catalog","functionName":"`+function+`","data":{}}`))
					request.Header.Set("Content-Type", "application/json")
					request.Header.Set("authorization", token)
					before := calls.Load()
					response, err := client.Do(request)
					if err != nil {
						t.Fatal(err)
					}
					var envelope map[string]any
					err = json.NewDecoder(response.Body).Decode(&envelope)
					_ = response.Body.Close()
					if err != nil || response.StatusCode != 200 {
						t.Fatalf("RPC HTTP: %v, %d", err, response.StatusCode)
					}
					if token == "denied" {
						if envelope["code"] != "unauthorized" || calls.Load()-before != 1 {
							t.Fatalf("authorization changed: %v", envelope)
						}
					} else if envelope["status"] != "ok" || calls.Load()-before != 2 {
						t.Fatalf("execution changed: %v", envelope)
					}
				}
			}
		})
	}
}
