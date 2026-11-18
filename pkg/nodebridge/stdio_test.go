package nodebridge

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

type stdioProcessResult struct {
	Error           string `json:"error"`
	TooLong         bool   `json:"tooLong"`
	MiddlewareCalls int32  `json:"middlewareCalls"`
	HandlerCalls    int32  `json:"handlerCalls"`
	Allocated       uint64 `json:"allocated"`
}

// Exercise the exported entry point over actual process stdin/stdout, without
// swapping global file descriptors in the parent test process.
func TestServeStdioProcess(t *testing.T) {
	if os.Getenv("CRPC_STDIO_TEST_CHILD") != "1" {
		return
	}
	app, middlewares, handlers := frameApp(t)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	err := ServeStdio(app)
	runtime.ReadMemStats(&after)
	result := stdioProcessResult{TooLong: errors.Is(err, bufio.ErrTooLong), MiddlewareCalls: middlewares.Load(), HandlerCalls: handlers.Load(), Allocated: after.TotalAlloc - before.TotalAlloc}
	if err != nil {
		result.Error = err.Error()
	}
	_ = json.NewEncoder(os.Stderr).Encode(result)
	if err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func stdioCommand(t *testing.T) *exec.Cmd {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestServeStdioProcess$")
	cmd.Env = append(os.Environ(), "CRPC_STDIO_TEST_CHILD=1", "GORACE=atexit_sleep_ms=0")
	cmd.WaitDelay = time.Second
	return cmd
}

func stdioResult(t *testing.T, stderr []byte) stdioProcessResult {
	t.Helper()
	var result stdioProcessResult
	if err := json.Unmarshal(stderr, &result); err != nil {
		t.Fatalf("child did not return lifecycle result: %v: %s", err, stderr)
	}
	return result
}

func runStdio(t *testing.T, wire []byte) ([]BridgeResponse, stdioProcessResult, error) {
	t.Helper()
	cmd := stdioCommand(t)
	var stdout, stderr bytes.Buffer
	cmd.Stdin = bytes.NewReader(wire)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := stdioResult(t, stderr.Bytes())
	decoder := json.NewDecoder(&stdout)
	var responses []BridgeResponse
	for {
		var response BridgeResponse
		if decodeErr := decoder.Decode(&response); decodeErr == io.EOF {
			break
		} else if decodeErr != nil {
			t.Fatalf("invalid stdout: %v", decodeErr)
		}
		responses = append(responses, response)
	}
	return responses, result, err
}

func stdioWire(size int, delimiter string) []byte {
	return append(frameRequest(size-len(delimiter)), []byte(delimiter)...)
}

func TestServeStdioFrameBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name      string
		size      int
		delimiter string
		valid     bool
	}{
		{"small_lf", 128, "\n", true},
		{"below_old_limit", 64*1024 - 1, "\n", true},
		{"at_old_limit", 64 * 1024, "\n", true},
		{"above_old_limit", 64*1024 + 1, "\n", true},
		{"128_kib", 128 * 1024, "\n", true},
		{"below_max_lf", MaxIPCFrameSize - 1, "\n", true},
		{"exact_max_lf", MaxIPCFrameSize, "\n", true},
		{"above_max_lf", MaxIPCFrameSize + 1, "\n", false},
		{"below_max_crlf", MaxIPCFrameSize - 1, "\r\n", true},
		{"exact_max_crlf", MaxIPCFrameSize, "\r\n", true},
		{"above_max_crlf", MaxIPCFrameSize + 1, "\r\n", false},
		{"below_max_eof", MaxIPCFrameSize - 1, "", true},
		{"exact_max_eof", MaxIPCFrameSize, "", true},
		{"above_max_eof", MaxIPCFrameSize + 1, "", false},
		{"exact_max_final_cr", MaxIPCFrameSize, "\r", true},
		{"above_max_final_cr", MaxIPCFrameSize + 1, "\r", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			responses, result, err := runStdio(t, stdioWire(tc.size, tc.delimiter))
			if !tc.valid {
				if err == nil || !result.TooLong || result.Error != bufio.ErrTooLong.Error() || len(responses) != 0 || result.MiddlewareCalls != 0 || result.HandlerCalls != 0 {
					t.Fatalf("oversize frame dispatched or error changed: err=%v result=%+v replies=%+v", err, result, responses)
				}
				return
			}
			if err != nil || result.Error != "" || len(responses) != 1 {
				t.Fatalf("supported %d-byte frame failed: err=%v result=%+v replies=%+v", tc.size, err, result, responses)
			}
			response := responses[0]
			data, _ := response.Data.(map[string]any)
			expected := tc.size - len(tc.delimiter) - len(frameRequestPrefix) - 2
			if response.Status != "ok" || response.ID != "frame" || data["bytes"] != float64(expected) || result.MiddlewareCalls != 1 || result.HandlerCalls != 1 {
				t.Fatalf("content/auth/context not preserved: result=%+v reply=%+v wantBytes=%d", result, response, expected)
			}
		})
	}
}

func TestServeStdioRecoveryAndLifecycle(t *testing.T) {
	t.Run("blank_malformed_crlf_and_final_eof", func(t *testing.T) {
		wire := []byte("\n\r\n{\"id\":}\n \t\n")
		wire = append(wire, stdioWire(128, "\r\n")...)
		wire = append(wire, stdioWire(129, "")...)
		responses, result, err := runStdio(t, wire)
		if err != nil || len(responses) != 4 || result.HandlerCalls != 2 || result.MiddlewareCalls != 2 {
			t.Fatalf("recovery failed: %v %+v %+v", err, result, responses)
		}
		for _, response := range responses[:2] {
			if response.ID != "" || response.Status != "error" || !strings.HasPrefix(response.Error, "Invalid JSON:") {
				t.Fatalf("malformed JSON error changed: %+v", response)
			}
		}
		for _, response := range responses[2:] {
			if response.ID != "frame" || response.Status != "ok" {
				t.Fatalf("next valid line lost: %+v", response)
			}
		}
	})
	t.Run("multiple_exact_limit_lines", func(t *testing.T) {
		wire := bytes.Repeat(stdioWire(MaxIPCFrameSize, "\n"), 3)
		responses, result, err := runStdio(t, wire)
		if err != nil || len(responses) != 3 || result.HandlerCalls != 3 || result.MiddlewareCalls != 3 {
			t.Fatalf("repeated large frames failed: %v %+v", err, result)
		}
		for _, response := range responses {
			if response.Status != "ok" || response.ID != "frame" {
				t.Fatalf("large-frame correlation lost: %+v", response)
			}
		}
	})
	t.Run("application_errors_continue", func(t *testing.T) {
		wire := bytes.ReplaceAll(stdioWire(128, "\n"), []byte(`"x-token":"valid"`), []byte(`"x-token":"denied"`))
		wire = append(wire, []byte(`{"id":"schema","function":"frames.echo","headers":{"x-token":"valid"},"data":12}`+"\n")...)
		wire = append(wire, []byte(`{"id":"missing","function":"frames.missing","data":null}`+"\n")...)
		wire = append(wire, stdioWire(128, "\n")...)
		responses, result, err := runStdio(t, wire)
		if err != nil || len(responses) != 4 || result.MiddlewareCalls != 3 || result.HandlerCalls != 1 {
			t.Fatalf("application errors changed: %v %+v %+v", err, result, responses)
		}
		for i, code := range []string{"unauthorized", "validation_error", "not_found"} {
			if responses[i].Status != "error" || !strings.Contains(responses[i].Error, "["+code+"]") {
				t.Fatalf("wrong application error: %+v", responses[i])
			}
		}
		if responses[3].Status != "ok" {
			t.Fatalf("server did not recover after application errors: %+v", responses[3])
		}
	})
	t.Run("partial_json_at_eof", func(t *testing.T) {
		responses, result, err := runStdio(t, []byte(`{"id":`))
		if err != nil || len(responses) != 1 || !strings.HasPrefix(responses[0].Error, "Invalid JSON:") || result.HandlerCalls != 0 {
			t.Fatalf("EOF JSON handling changed: %v %+v %+v", err, result, responses)
		}
	})
	t.Run("empty_and_cr_eof", func(t *testing.T) {
		for _, wire := range [][]byte{nil, []byte("\r")} {
			responses, result, err := runStdio(t, wire)
			if err != nil || len(responses) != 0 || result.HandlerCalls != 0 {
				t.Fatalf("empty EOF changed: %v %+v %+v", err, result, responses)
			}
		}
	})
	t.Run("oversize_retires_stream_without_next_dispatch", func(t *testing.T) {
		wire := append(stdioWire(MaxIPCFrameSize+1, "\n"), stdioWire(128, "\n")...)
		responses, result, err := runStdio(t, wire)
		if err == nil || !result.TooLong || len(responses) != 0 || result.HandlerCalls != 0 || result.MiddlewareCalls != 0 {
			t.Fatalf("oversize stream was reused: %v %+v %+v", err, result, responses)
		}
	})
}

func TestServeStdioPartialAndSlowLine(t *testing.T) {
	cmd := stdioCommand(t)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stdin.Close(); _ = cmd.Process.Kill() })
	line := frameRequest(128)
	if _, err := stdin.Write(line[:64]); err != nil {
		t.Fatal(err)
	}
	reply := make(chan []byte, 1)
	readError := make(chan error, 1)
	go func() { b, err := bufio.NewReader(stdout).ReadBytes('\n'); reply <- b; readError <- err }()
	select {
	case b := <-reply:
		t.Fatalf("partial frame replied early: %q", b)
	case <-time.After(30 * time.Millisecond):
	}
	if _, err := stdin.Write(line[64:]); err != nil {
		t.Fatal(err)
	}
	select {
	case b := <-reply:
		t.Fatalf("unterminated frame replied before EOF/newline: %q", b)
	case <-time.After(30 * time.Millisecond):
	}
	if _, err := stdin.Write([]byte{'\n'}); err != nil {
		t.Fatal(err)
	}
	var response BridgeResponse
	select {
	case b := <-reply:
		if err := json.Unmarshal(b, &response); err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("complete line did not receive response")
	}
	if err := <-readError; err != nil {
		t.Fatal(err)
	}
	if response.Status != "ok" || response.ID != "frame" {
		t.Fatalf("bad slow response: %+v", response)
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	result := stdioResult(t, stderr.Bytes())
	if result.HandlerCalls != 1 {
		t.Fatalf("wrong dispatch count: %+v", result)
	}
}

// Supply an oversized stream lazily: neither parent nor child needs to allocate
// the advertised 128MiB input. Leave stdin open to prove rejection does not wait for EOF.
type repeatedStdioBytes struct{}

func (repeatedStdioBytes) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	return len(p), nil
}

func TestServeStdioUnterminatedAllocationBound(t *testing.T) {
	cmd := stdioCommand(t)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stdin.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() })
	written := make(chan int64, 1)
	go func() { n, _ := io.CopyN(stdin, repeatedStdioBytes{}, 64*MaxIPCFrameSize); written <- n }()
	err = cmd.Wait()
	_ = stdin.Close()
	var n int64
	select {
	case n = <-written:
	case <-time.After(3 * time.Second):
		t.Fatal("oversize stdin writer did not exit")
	}
	result := stdioResult(t, stderr.Bytes())
	t.Logf("accepted_pipe_bytes=%d allocated=%d error=%q", n, result.Allocated, result.Error)
	if err == nil || !result.TooLong || stdout.Len() != 0 || result.HandlerCalls != 0 || result.MiddlewareCalls != 0 {
		t.Fatalf("unterminated oversize stream was dispatched: %v %+v", err, result)
	}
	if n < int64(MaxIPCFrameSize) || n > int64(MaxIPCFrameSize+128*1024) {
		t.Errorf("consumed %d bytes instead of the frame limit plus bounded pipe read-ahead", n)
	}
	// Scanner's geometric growth and one-byte lookahead have bounded cumulative
	// allocations. This excludes the parent's virtual input and any request decoding.
	if result.Allocated > 4*MaxIPCFrameSize {
		t.Errorf("allocated %d bytes reading an oversized stream", result.Allocated)
	}
}
