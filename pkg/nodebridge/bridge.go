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
	Data     any               `json:"data"`
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
	// Keep ScanLines' CRLF and final-EOF behavior, but count framing bytes too.
	// One lookahead byte distinguishes an exact-limit EOF line from overflow.
	scanner.Buffer(nil, MaxIPCFrameSize+1)
	scanner.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		advance, token, err := bufio.ScanLines(data, atEOF)
		if advance > MaxIPCFrameSize || (advance == 0 && len(data) > MaxIPCFrameSize) {
			return 0, nil, bufio.ErrTooLong
		}
		return advance, token, err
	})
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

// MaxIPCFrameSize limits IPC frames to 2 MiB. Line delimiters count toward the
// limit; the Unix length prefix is separate from its payload limit.
const MaxIPCFrameSize = 2 * 1024 * 1024

var errIPCFrameTooLarge = fmt.Errorf("Frame size exceeds limit of %d bytes", MaxIPCFrameSize)

// readLineFrame includes the newline in the existing size limit. Small frames borrow
// the reader buffer; long frames allocate one fixed, capped buffer without growing it.
func readLineFrame(reader *bufio.Reader) ([]byte, error) {
	fragment, err := reader.ReadSlice('\n')
	if len(fragment) > MaxIPCFrameSize {
		return nil, errIPCFrameTooLarge
	}
	if err != bufio.ErrBufferFull {
		return fragment, err
	}

	frame := make([]byte, MaxIPCFrameSize+1)
	n := copy(frame, fragment)
	for {
		// Near the limit, do not wait for ReadSlice to fill another whole fragment:
		// the very next byte may already prove that an unterminated frame is too large.
		if MaxIPCFrameSize-n < reader.Size() {
			b, readErr := reader.ReadByte()
			if readErr != nil {
				return frame[:n], readErr
			}
			frame[n] = b
			n++
			if n > MaxIPCFrameSize {
				return nil, errIPCFrameTooLarge
			}
			if b == '\n' {
				return frame[:n], nil
			}
			continue
		}
		fragment, err = reader.ReadSlice('\n')
		n += copy(frame[n:], fragment)
		if n > MaxIPCFrameSize {
			return nil, errIPCFrameTooLarge
		}
		if err != bufio.ErrBufferFull {
			return frame[:n], err
		}
	}
}

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
			line, err := readLineFrame(reader)
			if err == errIPCFrameTooLarge {
				_ = conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
				sendBridgeResponse(writer, BridgeResponse{
					Status: "error",
					Error:  err.Error(),
				})
				return
			}
			if err != nil {
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
				_ = conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
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
			_ = conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
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
