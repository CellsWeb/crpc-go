package nodebridge

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/CellsWeb/crpc-go/pkg/rpc"
	"github.com/CellsWeb/crpc-go/pkg/schema"
)

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

	// Test 2: Call with SkipMiddlewares and Context injection
	req2 := BridgeRequest{
		ID:              "req-2",
		Function:        "bridgeTest.getProfile",
		Data:            map[string]any{"msg": "direct call"},
		SkipMiddlewares: true,
		Context: map[string]any{
			"bridgeAuth": map[string]any{
				"user": map[string]any{"id": "usr_injected", "role": "superadmin"},
			},
		},
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
	if resp2.Status != "ok" {
		t.Fatalf("Expected ok, got: %s (err: %s)", resp2.Status, resp2.Error)
	}

	dataMap, ok := resp2.Data.(map[string]any)
	if !ok {
		t.Fatalf("Expected map in response data, got %T", resp2.Data)
	}
	userMap, ok := dataMap["user"].(map[string]any)
	if !ok || userMap["id"] != "usr_injected" {
		t.Fatalf("Expected injected user 'usr_injected', got: %v", dataMap["user"])
	}
}
