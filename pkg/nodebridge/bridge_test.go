package nodebridge

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CellsWeb/crpc-go/pkg/rpc"
	"github.com/CellsWeb/crpc-go/pkg/schema"
)

// Generate an unterminated frame without allocating the adversarial input itself.
type generatedFrameConn struct {
	remaining int
	readBytes int
	response  bytes.Buffer
	closed    bool
}

func (c *generatedFrameConn) Read(p []byte) (int, error) {
	if c.remaining == 0 {
		return 0, io.EOF
	}
	n := min(len(p), c.remaining)
	for i := range p[:n] {
		p[i] = 'x'
	}
	if c.readBytes == 0 {
		p[0] = '{'
	}
	c.readBytes += n
	c.remaining -= n
	return n, nil
}
func (c *generatedFrameConn) Write(p []byte) (int, error)    { return c.response.Write(p) }
func (c *generatedFrameConn) Close() error                   { c.closed = true; return nil }
func (*generatedFrameConn) LocalAddr() net.Addr              { return &net.UnixAddr{Net: "unix"} }
func (*generatedFrameConn) RemoteAddr() net.Addr             { return &net.UnixAddr{Net: "unix"} }
func (*generatedFrameConn) SetDeadline(time.Time) error      { return nil }
func (*generatedFrameConn) SetReadDeadline(time.Time) error  { return nil }
func (*generatedFrameConn) SetWriteDeadline(time.Time) error { return nil }

func TestIPCUnterminatedFrameAllocationBound(t *testing.T) {
	app, err := rpc.New(rpc.Config{RpcPath: "/rpc", CORS: &rpc.CorsConfig{AllowOrigins: []string{"*"}}})
	if err != nil {
		t.Fatal(err)
	}
	conn := &generatedFrameConn{remaining: 3 * 1024 * 1024}
	// Warm the error serializer so one-time reflection caches do not obscure read allocation.
	_, _ = json.Marshal(BridgeResponse{Status: "error", Error: "warmup"})
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	handleSocketConn(app, conn)
	runtime.ReadMemStats(&after)
	allocated := after.TotalAlloc - before.TotalAlloc
	t.Logf("read=%d allocated=%d response=%q closed=%v", conn.readBytes, allocated, conn.response.String(), conn.closed)
	// Allow fixed bufio buffers, error encoding, and modest runtime overhead, not an input-sized copy.
	if allocated > MaxIPCFrameSize+128*1024 {
		t.Errorf("frame read allocated %d bytes; expected a bounded frame buffer plus fixed overhead", allocated)
	}
	if conn.readBytes > MaxIPCFrameSize+4096 {
		t.Errorf("consumed %d unterminated bytes instead of stopping at the limit plus fixed bufio read-ahead", conn.readBytes)
	}
	var response BridgeResponse
	if err := json.Unmarshal(conn.response.Bytes(), &response); err != nil || response.Status != "error" || !strings.Contains(response.Error, "exceeds limit") || !conn.closed {
		t.Errorf("expected framing error and closed connection, response=%+v err=%v", response, err)
	}
}

func TestIPCOversizedNoNewlineRejected(t *testing.T) {
	app, err := rpc.New(rpc.Config{RpcPath: "/rpc", CORS: &rpc.CorsConfig{AllowOrigins: []string{"*"}}})
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	app.RegisterFunction(&rpc.FunctionDefinition{Namespace: "frames", FunctionName: "echo", Handler: func(*rpc.Context, any) (any, error) {
		calls.Add(1)
		return nil, nil
	}})
	conn, reader := newPayloadSocket(t, app)
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	payload := bytes.Repeat([]byte{'x'}, 3*1024*1024)
	payload[0] = '{'
	writeDone := make(chan error, 1)
	go func() { _, err := conn.Write(payload); writeDone <- err }()
	t.Cleanup(func() {
		_ = conn.Close()
		select {
		case <-writeDone:
		case <-time.After(3 * time.Second):
			t.Error("unterminated-frame writer did not exit")
		}
	})
	line, err := reader.ReadBytes('\n')
	if err != nil {
		t.Fatalf("expected rejection before EOF/newline: %v", err)
	}
	var response BridgeResponse
	if err := json.Unmarshal(line, &response); err != nil || response.Status != "error" || !strings.Contains(response.Error, "exceeds limit") {
		t.Fatalf("expected framing error, got %q err=%v", line, err)
	}
	if calls.Load() != 0 {
		t.Fatal("oversized frame reached handler")
	}
	if _, err := reader.ReadByte(); err == nil {
		t.Fatal("expected closed connection after oversized frame")
	} else if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
		t.Fatal("connection remained open after framing error")
	}
}

const frameRequestPrefix = `{"id":"frame","function":"frames.echo","headers":{"x-token":"valid"},"data":"`

func frameRequest(size int) []byte {
	return []byte(frameRequestPrefix + strings.Repeat("x", size-len(frameRequestPrefix)-2) + `"}`)
}

func frameWire(payload []byte, framing string) []byte {
	if framing == "line" {
		return append(append([]byte(nil), payload...), '\n')
	}
	wire := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(wire, uint32(len(payload)))
	copy(wire[4:], payload)
	return wire
}

func frameApp(t *testing.T) (*rpc.CoreRPC, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	app, err := rpc.New(rpc.Config{RpcPath: "/rpc", CORS: &rpc.CorsConfig{AllowOrigins: []string{"*"}}})
	if err != nil {
		t.Fatal(err)
	}
	middlewareCalls, handlerCalls := new(atomic.Int32), new(atomic.Int32)
	app.RegisterMiddleware("frameAuth", "Enforces auth", func(ctx *rpc.Context, headers map[string]string, _ map[string]any) error {
		middlewareCalls.Add(1)
		if headers["x-token"] != "valid" {
			return rpc.NewError(rpc.ErrUnauthorized, "Invalid token")
		}
		ctx.Set("actor", "reader")
		return nil
	}, nil, nil)
	app.RegisterFunction(&rpc.FunctionDefinition{
		Namespace: "frames", FunctionName: "echo", InputSchema: schema.String(),
		Middlewares: rpc.DefineMiddlewares(rpc.M("frameAuth", nil)),
		Handler: func(ctx *rpc.Context, input any) (any, error) {
			handlerCalls.Add(1)
			if actor, _ := ctx.Get("actor"); actor != "reader" {
				return nil, fmt.Errorf("middleware context was lost")
			}
			return map[string]any{"bytes": len(input.(string))}, nil
		},
	})
	return app, middlewareCalls, handlerCalls
}

func asyncFrameWrite(t *testing.T, conn net.Conn, wire []byte) <-chan struct{} {
	t.Helper()
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _, _ = conn.Write(wire); close(done) }()
	t.Cleanup(func() {
		_ = conn.Close()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("frame writer did not exit")
		}
	})
	return done
}

func readFrameResponse(t *testing.T, reader *bufio.Reader) BridgeResponse {
	t.Helper()
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

func TestIPCFrameBoundaries(t *testing.T) {
	for _, framing := range []string{"line", "length"} {
		for _, below := range []int{1, 0} {
			t.Run(fmt.Sprintf("%s_below_limit_%d", framing, below), func(t *testing.T) {
				app, middlewareCalls, handlerCalls := frameApp(t)
				conn, reader := newPayloadSocket(t, app)
				size := MaxIPCFrameSize - below
				if framing == "line" {
					size-- // The existing line-frame limit includes the newline delimiter.
				}
				asyncFrameWrite(t, conn, frameWire(frameRequest(size), framing))
				response := readFrameResponse(t, reader)
				data, _ := response.Data.(map[string]any)
				if response.Status != "ok" || response.ID != "frame" || data["bytes"] != float64(size-len(frameRequestPrefix)-2) || middlewareCalls.Load() != 1 || handlerCalls.Load() != 1 {
					t.Fatalf("exact/below-limit request failed: %+v", response)
				}
			})
		}
	}
}

func TestIPCOversizedFrameVariants(t *testing.T) {
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, MaxIPCFrameSize+1)
	for _, tc := range []struct {
		name string
		wire []byte
	}{
		{"line_one_byte_over", frameWire(frameRequest(MaxIPCFrameSize), "line")},
		{"line_three_mib", frameWire(frameRequest(3*1024*1024), "line")},
		{"no_newline_one_byte_over", append([]byte{'{'}, bytes.Repeat([]byte{'x'}, MaxIPCFrameSize)...)},
		{"length_header_only_over_limit", header},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, middlewareCalls, handlerCalls := frameApp(t)
			conn, reader := newPayloadSocket(t, app)
			asyncFrameWrite(t, conn, tc.wire)
			response := readFrameResponse(t, reader)
			if response.Status != "error" || !strings.Contains(response.Error, "exceeds limit") || middlewareCalls.Load() != 0 || handlerCalls.Load() != 0 {
				t.Fatalf("expected framing rejection without dispatch, got %+v", response)
			}
			if _, err := reader.ReadByte(); err == nil {
				t.Fatal("expected connection closure")
			} else if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
				t.Fatal("oversized connection was not closed")
			}
		})
	}
}

func TestIPCFrameRecoveryAndCoalescing(t *testing.T) {
	for _, framing := range []string{"line", "length"} {
		t.Run(framing, func(t *testing.T) {
			app, middlewareCalls, handlerCalls := frameApp(t)
			conn, reader := newPayloadSocket(t, app)
			size := MaxIPCFrameSize
			if framing == "line" {
				size--
			}
			wire := frameWire([]byte(`{"id":}`), framing)
			wire = append(wire, frameWire(frameRequest(size), framing)...)
			// Alternate framing modes on the same connection after the large frame.
			wire = append(wire, frameWire(frameRequest(128), "length")...)
			wire = append(wire, frameWire(frameRequest(129), "line")...)
			asyncFrameWrite(t, conn, wire)
			if response := readFrameResponse(t, reader); response.Status != "error" || !strings.HasPrefix(response.Error, "Invalid JSON:") {
				t.Fatalf("expected malformed JSON error, got %+v", response)
			}
			for _, expectedSize := range []int{size, 128, 129} {
				response := readFrameResponse(t, reader)
				data, _ := response.Data.(map[string]any)
				if response.Status != "ok" || response.ID != "frame" || data["bytes"] != float64(expectedSize-len(frameRequestPrefix)-2) {
					t.Fatalf("framing lost synchronization after malformed/large request: %+v", response)
				}
			}
			if middlewareCalls.Load() != 3 || handlerCalls.Load() != 3 {
				t.Fatal("malformed request dispatched or valid frames were lost")
			}
		})
	}
}

func TestIPCPartialFramesCleanup(t *testing.T) {
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, 100)
	for _, tc := range []struct {
		name string
		wire []byte
	}{
		{"empty", nil}, {"line", []byte(`{"id":`)}, {"length_header", header[:2]},
		{"length_body", append(append([]byte(nil), header...), '{')},
		{"exact_limit_without_newline", frameRequest(MaxIPCFrameSize)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, middlewareCalls, handlerCalls := frameApp(t)
			conn, reader := newPayloadSocket(t, app)
			done := asyncFrameWrite(t, conn, tc.wire)
			<-done
			if err := conn.(*net.UnixConn).CloseWrite(); err != nil {
				t.Fatal(err)
			}
			if _, err := reader.ReadByte(); err == nil {
				t.Fatal("partial frames must close without dispatch or a complete reply")
			} else if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
				t.Fatal("partial frame was not released after EOF")
			}
			if middlewareCalls.Load() != 0 || handlerCalls.Load() != 0 {
				t.Fatal("partial frame dispatched")
			}
		})
	}
}

type frameDeadlineConn struct {
	net.Conn
	readBudget, writeBudget time.Duration
	closed                  atomic.Bool
}

func (c *frameDeadlineConn) SetReadDeadline(deadline time.Time) error {
	c.readBudget = time.Until(deadline)
	return c.Conn.SetReadDeadline(time.Now().Add(40 * time.Millisecond))
}
func (c *frameDeadlineConn) SetWriteDeadline(deadline time.Time) error {
	c.writeBudget = time.Until(deadline)
	return c.Conn.SetWriteDeadline(time.Now().Add(40 * time.Millisecond))
}
func (c *frameDeadlineConn) Close() error { c.closed.Store(true); return c.Conn.Close() }

func deadlineFrameSocket(t *testing.T, app *rpc.CoreRPC) (net.Conn, *frameDeadlineConn, <-chan struct{}) {
	t.Helper()
	server, peer := net.Pipe()
	conn := &frameDeadlineConn{Conn: server}
	done := make(chan struct{})
	go func() { handleSocketConn(app, conn); close(done) }()
	t.Cleanup(func() {
		_ = peer.Close()
		_ = server.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("deadline handler did not exit")
		}
	})
	return peer, conn, done
}

func TestIPCReadDeadlineCleanup(t *testing.T) {
	for _, partial := range [][]byte{nil, []byte(`{"id":`), {0, 0}, {0, 0, 0, 100, '{'}} {
		t.Run(fmt.Sprintf("partial_%x", partial), func(t *testing.T) {
			app, middlewareCalls, handlerCalls := frameApp(t)
			peer, conn, done := deadlineFrameSocket(t, app)
			if len(partial) > 0 {
				if _, err := peer.Write(partial); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("stalled reader did not honor deadline")
			}
			if !conn.closed.Load() || conn.readBudget < 59*time.Second || conn.readBudget > 60*time.Second || middlewareCalls.Load() != 0 || handlerCalls.Load() != 0 {
				t.Fatalf("read deadline or cleanup changed: budget=%v closed=%v", conn.readBudget, conn.closed.Load())
			}
		})
	}
}

func TestIPCWriteDeadlineCleanup(t *testing.T) {
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, MaxIPCFrameSize+1)
	for _, tc := range []struct {
		name string
		wire []byte
	}{
		{"oversized_length", header},
		{"oversized_line", frameWire(frameRequest(MaxIPCFrameSize), "line")},
		{"malformed_json", []byte("{bad}\n")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, middlewareCalls, handlerCalls := frameApp(t)
			peer, conn, done := deadlineFrameSocket(t, app)
			if _, err := peer.Write(tc.wire); err != nil {
				t.Fatal(err)
			}
			// Deliberately do not read the response: error writes must also be bounded.
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("error response blocked without a write deadline")
			}
			if !conn.closed.Load() || conn.writeBudget < 29*time.Second || conn.writeBudget > 30*time.Second || middlewareCalls.Load() != 0 || handlerCalls.Load() != 0 {
				t.Fatalf("write deadline or cleanup missing: budget=%v closed=%v", conn.writeBudget, conn.closed.Load())
			}
		})
	}
}

func TestIPCSlowFragmentedFrame(t *testing.T) {
	app, middlewareCalls, handlerCalls := frameApp(t)
	conn, reader := newPayloadSocket(t, app)
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	wire := frameWire(frameRequest(128), "line")
	if _, err := conn.Write(wire[:32]); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	if middlewareCalls.Load() != 0 || handlerCalls.Load() != 0 {
		t.Fatal("unfinished slow frame dispatched")
	}
	if _, err := conn.Write(wire[32:]); err != nil {
		t.Fatal(err)
	}
	if response := readFrameResponse(t, reader); response.Status != "ok" || handlerCalls.Load() != 1 {
		t.Fatalf("slow complete frame failed: %+v", response)
	}
}

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
