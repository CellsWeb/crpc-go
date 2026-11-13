package nodebridge

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/CellsWeb/crpc-go/pkg/rpc"
)

// BridgeRequest represents an IPC call from Node.js
type BridgeRequest struct {
	ID              string            `json:"id"`
	Function        string            `json:"function"`
	Data            map[string]any    `json:"data"`
	Headers         map[string]string `json:"headers,omitempty"`
	SkipMiddlewares bool              `json:"skipMiddlewares,omitempty"`
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
			Headers:         req.Headers,
			SkipMiddlewares: req.SkipMiddlewares,
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

// ServeUnixSocket runs a Unix Domain Socket listener for high-speed local IPC
func ServeUnixSocket(app *rpc.CoreRPC, socketPath string) error {
	_ = os.Remove(socketPath)
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return err
	}
	defer ln.Close()
	defer os.Remove(socketPath)

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
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				_ = err
			}
			return
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
		callErr := app.CallInMemory(req.Function, req.Data, &output, rpc.CallOptions{
			Headers:         req.Headers,
			SkipMiddlewares: req.SkipMiddlewares,
		})

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
