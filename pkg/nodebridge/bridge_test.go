package nodebridge

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CellsWeb/crpc-go/pkg/rpc"
	"github.com/CellsWeb/crpc-go/pkg/schema"
)

// Use a real Unix socket and the production connection handler, with bounded test shutdown.
// ServeUnixSocket's public listener/permissions are covered by the existing test below.
func newPayloadSocket(t *testing.T, app *rpc.CoreRPC) (net.Conn, *bufio.Reader) {
	t.Helper()
	dir, err := os.MkdirTemp("", "crpc003-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	listener, err := net.Listen("unix", filepath.Join(dir, "test.sock"))
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			handleSocketConn(app, conn)
		}
		done <- err
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		select {
		case err := <-done:
			if err != nil && !errors.Is(err, net.ErrClosed) {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("Unix IPC handler did not exit")
		}
	})
	conn, err := net.DialTimeout("unix", listener.Addr().String(), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn, bufio.NewReader(conn)
}

func payloadRoundTrip(t *testing.T, conn net.Conn, reader *bufio.Reader, framing, id, function string, input any, authorized bool) BridgeResponse {
	t.Helper()
	headers := map[string]string{}
	if authorized {
		headers["x-token"] = "valid"
	}
	// Encode raw JSON rather than BridgeRequest so baseline's map-only field cannot hide the failure.
	payload, err := json.Marshal(map[string]any{
		"id": id, "function": function, "data": input, "headers": headers,
		"skipMiddlewares": true, // Must never bypass auth, even for scalar/array requests.
	})
	if err != nil {
		t.Fatal(err)
	}
	if framing == "line" {
		payload = append(payload, '\n')
	} else {
		frame := make([]byte, 4+len(payload))
		binary.BigEndian.PutUint32(frame, uint32(len(payload)))
		copy(frame[4:], payload)
		payload = frame
	}
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write(payload); err != nil {
		t.Fatal(err)
	}
	line, err := reader.ReadBytes('\n')
	if err != nil {
		t.Fatal(err)
	}
	var response BridgeResponse
	if err := json.Unmarshal(line, &response); err != nil {
		t.Fatalf("invalid response %q: %v", line, err)
	}
	return response
}

func TestUnixIPCJSONPayloadShapes(t *testing.T) {
	app, err := rpc.New(rpc.Config{RpcPath: "/rpc", CORS: &rpc.CorsConfig{AllowOrigins: []string{"*"}}})
	if err != nil {
		t.Fatal(err)
	}
	var middlewareCalls, handlerCalls atomic.Int32
	received := make(chan any, 1)
	app.RegisterMiddleware("payloadAuth", "Enforces auth", func(ctx *rpc.Context, headers map[string]string, _ map[string]any) error {
		middlewareCalls.Add(1)
		if headers["x-token"] != "valid" {
			return rpc.NewError(rpc.ErrUnauthorized, "Invalid token")
		}
		ctx.Set("actor", "reader")
		return nil
	}, nil, nil)
	app.RegisterFunction(&rpc.FunctionDefinition{
		Namespace: "shapes", FunctionName: "echo", Middlewares: rpc.DefineMiddlewares(rpc.M("payloadAuth", nil)),
		Handler: func(ctx *rpc.Context, input any) (any, error) {
			handlerCalls.Add(1)
			if actor, _ := ctx.Get("actor"); actor != "reader" {
				return nil, fmt.Errorf("middleware context was lost")
			}
			received <- input
			return map[string]any{"received": input}, nil
		},
	})
	for _, framing := range []string{"line", "length"} {
		t.Run(framing, func(t *testing.T) {
			conn, reader := newPayloadSocket(t, app)
			for _, tc := range []struct{ name, data string }{
				{"object", `{"items":["item1",null],"enabled":false}`}, {"empty_object", `{}`},
				{"array", `["item1","item2"]`}, {"empty_array", `[]`}, {"mixed_array", `["item1",2,false,null,[],{}]`},
				{"string", `"hello"`}, {"empty_string", `""`}, {"number", `12.5`}, {"zero", `0`},
				{"true", `true`}, {"false", `false`}, {"null", `null`},
			} {
				t.Run(tc.name, func(t *testing.T) {
					var input any
					if err := json.Unmarshal([]byte(tc.data), &input); err != nil {
						t.Fatal(err)
					}
					beforeMW, beforeHandler := middlewareCalls.Load(), handlerCalls.Load()
					response := payloadRoundTrip(t, conn, reader, framing, tc.name, "shapes.echo", input, true)
					if response.Status != "ok" || response.ID != tc.name {
						t.Fatalf("data=%s response=%+v", tc.data, response)
					}
					got := <-received
					output, ok := response.Data.(map[string]any)
					value, exists := output["received"]
					if !ok || !exists || !reflect.DeepEqual(got, input) || !reflect.DeepEqual(value, input) {
						t.Fatalf("data=%s handler=%#v (%T) output=%#v", tc.data, got, got, response.Data)
					}
					if middlewareCalls.Load() != beforeMW+1 || handlerCalls.Load() != beforeHandler+1 {
						t.Fatal("expected one middleware and handler invocation")
					}
					t.Logf("data=%s status=ok id=%s handlerType=%T preserved", tc.data, response.ID, got)
				})
			}
			for _, input := range []any{map[string]any{}, []string{"item1"}, "hello", 12.5, false, nil} {
				t.Run(fmt.Sprintf("unauthorized_%T", input), func(t *testing.T) {
					before := handlerCalls.Load()
					response := payloadRoundTrip(t, conn, reader, framing, "denied", "shapes.echo", input, false)
					if response.Status != "error" || response.ID != "denied" || !strings.Contains(response.Error, "[unauthorized]") || handlerCalls.Load() != before {
						t.Fatalf("auth must reject without handler invocation: %+v", response)
					}
				})
			}
		})
	}
}

func TestUnixIPCTypedArrayAndValidation(t *testing.T) {
	app, err := rpc.New(rpc.Config{RpcPath: "/rpc", CORS: &rpc.CorsConfig{AllowOrigins: []string{"*"}}})
	if err != nil {
		t.Fatal(err)
	}
	var handlerCalls atomic.Int32
	app.RegisterMiddleware("arrayAuth", "Enforces auth", func(ctx *rpc.Context, headers map[string]string, _ map[string]any) error {
		if headers["x-token"] != "valid" {
			return rpc.NewError(rpc.ErrUnauthorized, "Invalid token")
		}
		ctx.Set("actor", "reader")
		return nil
	}, nil, nil)
	type output struct {
		Items []string `json:"items"`
	}
	app.RegisterFunction(rpc.Register(rpc.FunctionConfig[[]string, output]{
		Namespace: "typedShapes", FunctionName: "array", InputSchema: schema.Array(schema.String()),
		Middlewares: rpc.DefineMiddlewares(rpc.M("arrayAuth", nil)),
		Handler: func(ctx *rpc.Context, input []string) (*output, error) {
			handlerCalls.Add(1)
			if actor, _ := ctx.Get("actor"); actor != "reader" {
				return nil, fmt.Errorf("middleware context was lost")
			}
			return &output{Items: input}, nil
		},
	}))
	for _, framing := range []string{"line", "length"} {
		t.Run(framing, func(t *testing.T) {
			conn, reader := newPayloadSocket(t, app)
			for _, tc := range []struct {
				name  string
				input any
				code  string
			}{
				{"array", []string{"item1", "item2"}, ""}, {"empty_array", []string{}, ""},
				{"wrong_shape", "item1", "[validation_error]"}, {"wrong_element", []int{1}, "[validation_error]"},
				{"null", nil, "[validation_error]"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					before := handlerCalls.Load()
					response := payloadRoundTrip(t, conn, reader, framing, tc.name, "typedShapes.array", tc.input, true)
					if tc.code != "" {
						if response.Status != "error" || response.ID != tc.name || !strings.Contains(response.Error, tc.code) || handlerCalls.Load() != before {
							t.Fatalf("expected schema rejection before handler: %+v", response)
						}
						return
					}
					want, _ := json.Marshal(map[string]any{"items": tc.input})
					got, _ := json.Marshal(response.Data)
					if response.Status != "ok" || response.ID != tc.name || string(got) != string(want) || handlerCalls.Load() != before+1 {
						t.Fatalf("want=%s response=%+v got=%s", want, response, got)
					}
				})
			}
		})
	}
}

type BridgeTestInput struct {
	Msg string `json:"msg"`
}

type BridgeTestOutput struct {
	User any    `json:"user"`
	Echo string `json:"echo"`
}

func TestServeUnixSocket_WithContextAndSkipMiddlewares(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "crpc-bridge-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	sockPath := filepath.Join(tmpDir, "test.sock")

	app, err := rpc.New(rpc.Config{
		RpcPath: "/rpc",
		CORS: &rpc.CorsConfig{
			AllowOrigins: []string{"*"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to create rpc app: %v", err)
	}

	// Register a middleware that sets user under its middleware name "bridgeAuth"
	app.RegisterMiddleware("bridgeAuth", "Requires valid token", func(c *rpc.Context, headers map[string]string, options map[string]any) error {
		if headers["x-token"] != "valid" {
			return rpc.NewError("unauthorized", "Invalid token")
		}
		c.Set("bridgeAuth", map[string]any{
			"user": map[string]any{"id": "usr_mw", "role": "admin"},
		})
		return nil
	}, nil, nil)

	// Register function that reads from context under middleware name
	rpc.Register(rpc.FunctionConfig[BridgeTestInput, BridgeTestOutput]{
		Namespace:    "bridgeTest",
		FunctionName: "getProfile",
		InputSchema:  schema.FromStruct(BridgeTestInput{}),
		OutputSchema: schema.FromStruct(BridgeTestOutput{}),
		Middlewares: rpc.DefineMiddlewares(
			rpc.M("bridgeAuth", map[string]any{}),
		),
		Handler: func(c *rpc.Context, input BridgeTestInput) (*BridgeTestOutput, error) {
			var user any
			if authVal, ok := c.Get("bridgeAuth"); ok {
				if authMap, ok := authVal.(map[string]any); ok {
					user = authMap["user"]
				}
			}
			return &BridgeTestOutput{
				User: user,
				Echo: input.Msg,
			}, nil
		},
	})

	app.UseGlobalRegistry()

	go func() {
		_ = ServeUnixSocket(app, sockPath)
	}()

	// Wait for socket to be ready
	var conn net.Conn
	for i := 0; i < 20; i++ {
		c, dialErr := net.Dial("unix", sockPath)
		if dialErr == nil {
			conn = c
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if conn == nil {
		t.Fatalf("Could not connect to unix socket %s", sockPath)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Test 1: Standard call through middleware with headers
	req1 := BridgeRequest{
		ID:       "req-1",
		Function: "bridgeTest.getProfile",
		Data:     map[string]any{"msg": "hello from node"},
		Headers:  map[string]string{"x-token": "valid"},
	}
	reqBytes1, _ := json.Marshal(req1)
	writer.Write(append(reqBytes1, '\n'))
	writer.Flush()

	line1, err := reader.ReadBytes('\n')
	if err != nil {
		t.Fatalf("Failed to read response 1: %v", err)
	}
	var resp1 BridgeResponse
	if err := json.Unmarshal(line1, &resp1); err != nil {
		t.Fatalf("Failed to parse response 1: %v", err)
	}
	if resp1.Status != "ok" {
		t.Fatalf("Expected ok, got: %s (err: %s)", resp1.Status, resp1.Error)
	}

	// Test 2: Call without required auth header (MUST be rejected with unauthorized)
	req2 := BridgeRequest{
		ID:       "req-2",
		Function: "bridgeTest.getProfile",
		Data:     map[string]any{"msg": "unauthenticated call"},
	}
	reqBytes2, _ := json.Marshal(req2)
	writer.Write(append(reqBytes2, '\n'))
	writer.Flush()

	line2, err := reader.ReadBytes('\n')
	if err != nil {
		t.Fatalf("Failed to read response 2: %v", err)
	}
	var resp2 BridgeResponse
	if err := json.Unmarshal(line2, &resp2); err != nil {
		t.Fatalf("Failed to parse response 2: %v", err)
	}
	if resp2.Status != "error" {
		t.Fatalf("Expected error status for unauthenticated call, got: %s", resp2.Status)
	}

	// Test 3: Verify socket permissions are 0600
	fi, err := os.Stat(sockPath)
	if err != nil {
		t.Fatalf("Failed to stat socket: %v", err)
	}
	perm := fi.Mode().Perm()
	if perm != 0600 {
		t.Fatalf("Expected socket permissions 0600, got %04o", perm)
	}
}
