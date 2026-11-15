package nodebridge

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/CellsWeb/crpc-go/pkg/rpc"
)

// BridgeRequest represents an IPC call from Node.js
type BridgeRequest struct {
	ID       string            `json:"id"`
	Function string            `json:"function"`
	Data     map[string]any    `json:"data"`
	Headers  map[string]string `json:"headers,omitempty"`
}

// BridgeResponse represents the IPC response returned to Node.js
type BridgeResponse struct {
	ID      string `json:"id"`
	Status  string `json:"status"` // "ok" or "error"
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
	Details any    `json:"details,omitempty"`
}

// ServeStdio starts a JSON-line stdio server for parent Node.js child processes
func ServeStdio(app *rpc.CoreRPC) error {
	scanner := bufio.NewScanner(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req BridgeRequest
		if err := json.Unmarshal(line, &req); err != nil {
			sendBridgeResponse(writer, BridgeResponse{
				Status: "error",
				Error:  fmt.Sprintf("Invalid JSON: %v", err),
			})
			continue
		}

		var output any
		err := app.CallInMemory(req.Function, req.Data, &output, rpc.CallOptions{
			Headers: req.Headers,
		})

		if err != nil {
			sendBridgeResponse(writer, BridgeResponse{
				ID:     req.ID,
				Status: "error",
				Error:  err.Error(),
			})
		} else {
			sendBridgeResponse(writer, BridgeResponse{
				ID:     req.ID,
				Status: "ok",
				Data:   output,
			})
		}
	}

	return scanner.Err()
}

// MaxIPCFrameSize limits single frames over IPC to 2 MB to prevent memory exhaustion
const MaxIPCFrameSize = 2 * 1024 * 1024

// ServeUnixSocket runs a Unix Domain Socket listener with explicit 0600 permissions
func ServeUnixSocket(app *rpc.CoreRPC, socketPath string) error {
	dir := filepath.Dir(socketPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create socket directory %q: %w", dir, err)
	}

	// Remove old socket only if it is actually a socket file
	if fi, err := os.Lstat(socketPath); err == nil {
		if fi.Mode()&os.ModeSocket != 0 {
			_ = os.Remove(socketPath)
		} else {
			return fmt.Errorf("cannot bind socket: path %q exists and is not a socket", socketPath)
		}
	}

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return err
	}
	defer ln.Close()
	defer os.Remove(socketPath)

	// Explicitly lock down socket permissions to 0600 so only the owning process/user can connect
	if err := os.Chmod(socketPath, 0600); err != nil {
		_ = ln.Close()
		_ = os.Remove(socketPath)
		return fmt.Errorf("failed to set socket permissions to 0600: %w", err)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go handleSocketConn(app, conn)
	}
}

func handleSocketConn(app *rpc.CoreRPC, conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for {
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		peek, err := reader.Peek(1)
		if err != nil {
			return
		}

		var payloadBytes []byte

		if peek[0] == '{' {
			// Newline-delimited JSON frame (bounded to MaxIPCFrameSize)
			line, err := reader.ReadBytes('\n')
			if err != nil {
				return
			}
			if len(line) > MaxIPCFrameSize {
				sendBridgeResponse(writer, BridgeResponse{
					Status: "error",
					Error:  fmt.Sprintf("Frame size %d exceeds limit of %d bytes", len(line), MaxIPCFrameSize),
				})
				return
			}
			payloadBytes = line
		} else {
			// Length-prefixed frame: 4-byte big-endian uint32
			header := make([]byte, 4)
			if _, err := io.ReadFull(reader, header); err != nil {
				return
			}
			frameLen := binary.BigEndian.Uint32(header)
			if frameLen > MaxIPCFrameSize {
				sendBridgeResponse(writer, BridgeResponse{
					Status: "error",
					Error:  fmt.Sprintf("Frame length %d exceeds limit of %d bytes", frameLen, MaxIPCFrameSize),
				})
				return
			}
			buf := make([]byte, frameLen)
			if _, err := io.ReadFull(reader, buf); err != nil {
				return
			}
			payloadBytes = buf
		}

		var req BridgeRequest
		if err := json.Unmarshal(payloadBytes, &req); err != nil {
			sendBridgeResponse(writer, BridgeResponse{
				Status: "error",
				Error:  fmt.Sprintf("Invalid JSON: %v", err),
			})
			continue
		}

		var output any
		callErr := app.CallInMemory(req.Function, req.Data, &output, rpc.CallOptions{
			Headers: req.Headers,
		})

		_ = conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
		if callErr != nil {
			sendBridgeResponse(writer, BridgeResponse{
				ID:     req.ID,
				Status: "error",
				Error:  callErr.Error(),
			})
		} else {
			sendBridgeResponse(writer, BridgeResponse{
				ID:     req.ID,
				Status: "ok",
				Data:   output,
			})
		}
	}
}

func sendBridgeResponse(w *bufio.Writer, resp BridgeResponse) {
	bytes, _ := json.Marshal(resp)
	_, _ = w.Write(bytes)
	_, _ = w.Write([]byte("\n"))
	_ = w.Flush()
}
