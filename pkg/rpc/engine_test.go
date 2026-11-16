package rpc_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CellsWeb/crpc-go/pkg/rpc"
	"github.com/CellsWeb/crpc-go/pkg/schema"
	"github.com/valyala/fasthttp"
)

func TestDuplicateRoutingHeadersRejection(t *testing.T) {
	app, err := rpc.New(rpc.Config{
		RpcPath: "/rpc",
		CORS:    &rpc.CorsConfig{AllowOrigins: []string{"*"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var middlewareCalls, handlerCalls atomic.Int32
	app.RegisterMiddleware("routingGuard", "Counts dispatches", func(*rpc.Context, map[string]string, map[string]any) error {
		middlewareCalls.Add(1)
		return nil
	}, nil, nil)
	app.RegisterFunction(&rpc.FunctionDefinition{
		Namespace:    "admin",
		FunctionName: "upload",
		ContentType:  "multipart/form-data",
		Middlewares:  rpc.DefineMiddlewares(rpc.M("routingGuard", nil)),
		Handler: func(_ *rpc.Context, input any) (any, error) {
			handlerCalls.Add(1)
			file, ok := input.(map[string]any)["file"].(*rpc.UploadedFile)
			if !ok || file.TempPath() == "" {
				return nil, fmt.Errorf("expected a disk-backed upload")
			}
			if _, err := os.Stat(file.TempPath()); err != nil {
				return nil, err
			}
			return map[string]any{"accepted": true}, nil
		},
	})

	// Exercise native fasthttp HTTP parsing; an adaptor or Header.Set would hide duplicates.
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &fasthttp.Server{Handler: app.Handler(), ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second}
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()
	t.Cleanup(func() {
		if err := server.Shutdown(); err != nil {
			t.Error(err)
		}
		if err := <-serveDone; err != nil {
			t.Error(err)
		}
	})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, field := range [][2]string{{"namespace", "admin"}, {"functionName", "upload"}} {
		if err := writer.WriteField(field[0], field[1]); err != nil {
			t.Fatal(err)
		}
	}
	file, err := writer.CreateFormFile("file", "proof.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(file, "CRPC-001 upload proof"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	const ns = "X-CRPC-Namespace: admin\r\n"
	const fn = "X-CRPC-Function: upload\r\n"
	for _, tc := range []struct {
		name    string
		headers string
		valid   bool
	}{
		{"namespace_conflicting", ns + "X-CRPC-Namespace: public\r\n" + fn, false},
		{"namespace_identical", ns + ns + fn, false},
		{"function_conflicting", ns + fn + "X-CRPC-Function: other\r\n", false},
		{"function_identical", ns + fn + fn, false},
		{"namespace_mixed_case", ns + "x-cRpC-nAmEsPaCe: public\r\n" + fn, false},
		{"function_mixed_case", ns + fn + "x-cRpC-fUnCtIoN: other\r\n", false},
		{"namespace_empty_first", "X-CRPC-Namespace:\r\n" + ns + fn, false},
		{"function_empty_second", ns + fn + "X-CRPC-Function:\r\n", false},
		{"both_duplicated", ns + ns + fn + fn, false},
		{"single_headers", ns + fn, true},
		{"headerless_fallback", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// A fresh, absent upload directory proves rejection happened before file creation,
			// rather than merely observing an empty directory after cleanup.
			tmpDir := t.TempDir()
			t.Setenv("TMPDIR", tmpDir)
			middlewareCalls.Store(0)
			handlerCalls.Store(0)
			conn, err := net.DialTimeout("tcp4", listener.Addr().String(), 5*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			_, err = fmt.Fprintf(conn, "POST /rpc HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\nContent-Type: %s\r\nContent-Length: %d\r\n%s\r\n%s",
				writer.FormDataContentType(), body.Len(), tc.headers, body.String())
			if err != nil {
				t.Fatal(err)
			}
			response, err := http.ReadResponse(bufio.NewReader(conn), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			payload, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			var envelope struct {
				Status  string `json:"status"`
				Code    string `json:"code"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(payload, &envelope); err != nil {
				t.Fatalf("Invalid JSON response %q: %v", payload, err)
			}
			t.Logf("HTTP %d %s; middleware=%d handler=%d", response.StatusCode, payload, middlewareCalls.Load(), handlerCalls.Load())
			if response.StatusCode != http.StatusOK {
				t.Errorf("Expected HTTP 200, got %d", response.StatusCode)
			}
			uploadDir := filepath.Join(tmpDir, "crpc-uploads")
			if tc.valid {
				if envelope.Status != "ok" || middlewareCalls.Load() != 1 || handlerCalls.Load() != 1 {
					t.Errorf("Expected successful upload and one middleware/handler call")
				}
				entries, err := os.ReadDir(uploadDir)
				if err != nil || len(entries) != 0 {
					t.Errorf("Expected upload cleanup, entries=%v err=%v", entries, err)
				}
				return
			}
			if envelope.Status != "error" || envelope.Code != rpc.ErrInvalidBody ||
				envelope.Message != "Duplicate routing headers ('X-CRPC-Namespace' or 'X-CRPC-Function') in multipart request" {
				t.Errorf("Expected duplicate routing header invalid_body error, got %s", payload)
			}
			if middlewareCalls.Load() != 0 || handlerCalls.Load() != 0 {
				t.Errorf("Expected zero middleware/handler calls")
			}
			if _, err := os.Stat(uploadDir); !os.IsNotExist(err) {
				t.Errorf("Expected no application upload directory to be created, got %v", err)
			}
			if entries, err := os.ReadDir(tmpDir); err != nil || len(entries) != 0 {
				t.Errorf("Expected zero temporary files, entries=%v err=%v", entries, err)
			}
		})
	}
}

type ProfileInput struct {
	UserID string `json:"userId"`
}

type ProfileOutput struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func TestEngineExecution(t *testing.T) {
	app, err := rpc.New(rpc.Config{
		RpcPath: "/rpc",
		CORS: &rpc.CorsConfig{
			AllowOrigins: []string{"*"},
		},
	})
	if err != nil {
		t.Fatalf("Failed creating app: %v", err)
	}

	// Register Auth Middleware
	app.RegisterMiddleware("auth", "Requires valid token", func(ctx *rpc.Context, headers map[string]string, options map[string]any) error {
		auth := headers["authorization"]
		if auth != "Bearer valid_token" {
			return rpc.NewError(rpc.ErrUnauthorized, "Invalid auth token")
		}
		ctx.Set("user", "test_user_101")
		return nil
	}, nil, nil)

	// Register Function
	rpc.Register(rpc.FunctionConfig[ProfileInput, ProfileOutput]{
		Namespace:    "user",
		FunctionName: "getProfile",
		InputSchema:  schema.FromStruct(ProfileInput{}),
		OutputSchema: schema.FromStruct(ProfileOutput{}),
		Middlewares: rpc.DefineMiddlewares(
			rpc.M("auth", map[string]any{}),
		),
		Handler: func(ctx *rpc.Context, input ProfileInput) (*ProfileOutput, error) {
			user, _ := ctx.Get("user")
			if user != "test_user_101" {
				return nil, rpc.NewError(rpc.ErrForbidden, "User mismatch")
			}
			return &ProfileOutput{
				Name:  "Minhazur Rahman",
				Email: "minhaz@example.com",
			}, nil
		},
	})

	app.UseGlobalRegistry()

	caller := rpc.NewTestCaller(app, rpc.TestOptions{
		Headers: map[string]string{
			"authorization": "Bearer valid_token",
		},
	})

	// 1. Success Call
	var output ProfileOutput
	err = caller.Call("user.getProfile", ProfileInput{UserID: "usr_1"}, &output)
	if err != nil {
		t.Fatalf("Expected success, got %v", err)
	}
	if output.Name != "Minhazur Rahman" {
		t.Fatalf("Expected name 'Minhazur Rahman', got %q", output.Name)
	}

	// 2. Auth Failure (Fail-Closed)
	unauthCaller := rpc.NewTestCaller(app, rpc.TestOptions{
		Headers: map[string]string{
			"authorization": "Bearer bad_token",
		},
	})
	rawRes, err := unauthCaller.CallRaw("user.getProfile", ProfileInput{UserID: "usr_1"})
	if err != nil {
		t.Fatalf("Unexpected test error: %v", err)
	}
	if rawRes.Status != "error" || rawRes.Code != rpc.ErrUnauthorized {
		t.Fatalf("Expected error code 'unauthorized', got %q", rawRes.Code)
	}

	// 3. Validation Failure (Unknown fields)
	resWithExtra, err := caller.CallRaw("user.getProfile", map[string]any{
		"userId":  "usr_1",
		"unknown": "attack",
	})
	if err != nil {
		t.Fatalf("Unexpected test error: %v", err)
	}
	if resWithExtra.Status != "error" || resWithExtra.Code != rpc.ErrValidation {
		t.Fatalf("Expected validation error, got status=%s code=%s", resWithExtra.Status, resWithExtra.Code)
	}
}

type ItemsOutput struct {
	Items []string `json:"items"`
}

func TestHttpStatusCodesAndJsonV2(t *testing.T) {
	app, err := rpc.New(rpc.Config{
		RpcPath:            "/rpc",
		UseHttpStatusCodes: true, // Configurable HTTP Status Codes
		CORS: &rpc.CorsConfig{
			AllowOrigins: []string{"*"},
		},
	})
	if err != nil {
		t.Fatalf("Failed creating app: %v", err)
	}

	rpc.Register(rpc.FunctionConfig[ProfileInput, ItemsOutput]{
		Namespace:    "items",
		FunctionName: "list",
		InputSchema:  schema.FromStruct(ProfileInput{}),
		Handler: func(ctx *rpc.Context, input ProfileInput) (*ItemsOutput, error) {
			// Deliberately return nil slice to verify JSON v2 serializes as []
			return &ItemsOutput{
				Items: nil,
			}, nil
		},
	})
	app.UseGlobalRegistry()

	handler := app.Handler()

	// 1. Success with nil slice -> verify serialized as [] (JSON v2)
	var ctx fasthttp.RequestCtx
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetRequestURI("/rpc")
	ctx.Request.Header.SetContentType("application/json")
	ctx.Request.SetBody([]byte(`{"namespace":"items","functionName":"list","data":{"userId":"u1"}}`))
	handler(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", ctx.Response.StatusCode())
	}
	bodyStr := string(ctx.Response.Body())
	if !strings.Contains(bodyStr, `"items":[]`) {
		t.Fatalf("Expected JSON v2 to serialize nil slice as '[]', got: %s", bodyStr)
	}

	// 2. Not Found function with UseHttpStatusCodes: true -> HTTP 404
	var ctxNotFound fasthttp.RequestCtx
	ctxNotFound.Request.Header.SetMethod("POST")
	ctxNotFound.Request.SetRequestURI("/rpc")
	ctxNotFound.Request.Header.SetContentType("application/json")
	ctxNotFound.Request.SetBody([]byte(`{"namespace":"unknown","functionName":"none","data":{}}`))
	handler(&ctxNotFound)

	if ctxNotFound.Response.StatusCode() != fasthttp.StatusNotFound {
		t.Fatalf("Expected 404 Not Found, got %d", ctxNotFound.Response.StatusCode())
	}

	// 3. Duplicate key payload -> rejected by JSON v2
	var ctxDup fasthttp.RequestCtx
	ctxDup.Request.Header.SetMethod("POST")
	ctxDup.Request.SetRequestURI("/rpc")
	ctxDup.Request.Header.SetContentType("application/json")
	ctxDup.Request.SetBody([]byte(`{"namespace":"items","namespace":"attack","functionName":"list","data":{}}`))
	handler(&ctxDup)

	if ctxDup.Response.StatusCode() != fasthttp.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request for duplicate keys, got %d", ctxDup.Response.StatusCode())
	}
}

func TestCorsWildcardWithCredentialsRejection(t *testing.T) {
	// 1. Standalone "*" with credentials -> error
	cfg1 := rpc.Config{
		RpcPath: "/rpc",
		CORS: &rpc.CorsConfig{
			AllowOrigins:     []string{"*"},
			AllowCredentials: true,
		},
	}
	if err := cfg1.Validate(); err == nil {
		t.Fatalf("Expected error rejecting wildcard origin with credentials")
	}

	// 2. Mixed wildcard with credentials -> error
	cfg2 := rpc.Config{
		RpcPath: "/rpc",
		CORS: &rpc.CorsConfig{
			AllowOrigins:     []string{"*", "https://cellsweb.com"},
			AllowCredentials: true,
		},
	}
	if err := cfg2.Validate(); err == nil {
		t.Fatalf("Expected error rejecting mixed wildcard origin with credentials")
	}

	// 3. Explicit origin with credentials -> valid
	cfg3 := rpc.Config{
		RpcPath: "/rpc",
		CORS: &rpc.CorsConfig{
			AllowOrigins:     []string{"https://cellsweb.com"},
			AllowCredentials: true,
		},
	}
	if err := cfg3.Validate(); err != nil {
		t.Fatalf("Expected explicit origin with credentials to pass, got: %v", err)
	}
}

func TestHeaderBasedMultipartPreAuthRejection(t *testing.T) {
	app, err := rpc.New(rpc.Config{
		RpcPath: "/rpc",
		CORS:    &rpc.CorsConfig{AllowOrigins: []string{"*"}},
	})
	if err != nil {
		t.Fatalf("Failed creating app: %v", err)
	}

	app.RegisterMiddleware("secureUploadAuth", "Checks token", func(ctx *rpc.Context, headers map[string]string, options map[string]any) error {
		if headers["x-upload-secret"] != "supersecret" {
			return rpc.NewError(rpc.ErrUnauthorized, "Invalid upload secret")
		}
		return nil
	}, nil, nil)

	rpc.Register(rpc.FunctionConfig[ProfileInput, ProfileOutput]{
		Namespace:    "docUpload",
		FunctionName: "submit",
		InputSchema:  schema.FromStruct(ProfileInput{}),
		Middlewares:  rpc.DefineMiddlewares(rpc.M("secureUploadAuth", nil)),
		Handler: func(ctx *rpc.Context, input ProfileInput) (*ProfileOutput, error) {
			return &ProfileOutput{Name: "done", Email: "done@test.com"}, nil
		},
	})
	app.UseGlobalRegistry()

	handler := app.Handler()

	// Send request with X-CRPC-Namespace and X-CRPC-Function headers, but INVALID secret
	var ctx fasthttp.RequestCtx
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetRequestURI("/rpc")
	ctx.Request.Header.SetContentType("multipart/form-data; boundary=----WebKitFormBoundary7MA4YWxkTrZu0gW")
	ctx.Request.Header.Set("X-CRPC-Namespace", "docUpload")
	ctx.Request.Header.Set("X-CRPC-Function", "submit")
	ctx.Request.Header.Set("x-upload-secret", "wrong-secret")
	// Dummy unparseable body — proves the server NEVER touches the body when headers auth fails!
	ctx.Request.SetBody([]byte("THIS IS NOT VALID MULTIPART DATA"))

	handler(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("Expected uniform 200 OK status, got %d", ctx.Response.StatusCode())
	}

	bodyStr := string(ctx.Response.Body())
	if !strings.Contains(bodyStr, `"code":"unauthorized"`) {
		t.Fatalf("Expected unauthorized error code in response envelope, got: %s", bodyStr)
	}
}

func TestWrappedErrorUnwrapping(t *testing.T) {
	app, err := rpc.New(rpc.Config{
		RpcPath: "/rpc",
		CORS:    &rpc.CorsConfig{AllowOrigins: []string{"*"}},
	})
	if err != nil {
		t.Fatalf("Failed creating app: %v", err)
	}

	rpc.Register(rpc.FunctionConfig[ProfileInput, ProfileOutput]{
		Namespace:    "errorsTest",
		FunctionName: "wrapped",
		InputSchema:  schema.FromStruct(ProfileInput{}),
		Handler: func(ctx *rpc.Context, input ProfileInput) (*ProfileOutput, error) {
			domainErr := rpc.NewError(rpc.ErrConflict, "Resource conflict occurred")
			// Return wrapped error via %w
			return nil, fmt.Errorf("outer handler failed: %w", domainErr)
		},
	})
	app.UseGlobalRegistry()

	caller := rpc.NewTestCaller(app)
	res, err := caller.CallRaw("errorsTest.wrapped", ProfileInput{UserID: "u1"})
	if err != nil {
		t.Fatalf("Unexpected test error: %v", err)
	}
	if res.Status != "error" || res.Code != rpc.ErrConflict {
		t.Fatalf("Expected error code 'conflict' from wrapped error, got %q", res.Code)
	}
}

func TestTelemetryPanicIsolation(t *testing.T) {
	panicHookCalled := false

	app, err := rpc.New(rpc.Config{
		RpcPath: "/rpc",
		CORS:    &rpc.CorsConfig{AllowOrigins: []string{"*"}},
		TelemetryHooks: []rpc.TelemetryHook{
			func(event rpc.TelemetryEvent) {
				panicHookCalled = true
				panic("deliberate third-party telemetry crash")
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed creating app: %v", err)
	}

	rpc.Register(rpc.FunctionConfig[ProfileInput, ProfileOutput]{
		Namespace:    "telemetryTest",
		FunctionName: "ping",
		InputSchema:  schema.FromStruct(ProfileInput{}),
		Handler: func(ctx *rpc.Context, input ProfileInput) (*ProfileOutput, error) {
			return &ProfileOutput{Name: "pong", Email: "pong@test.com"}, nil
		},
	})
	app.UseGlobalRegistry()

	handler := app.Handler()
	var ctx fasthttp.RequestCtx
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetRequestURI("/rpc")
	ctx.Request.Header.SetContentType("application/json")
	ctx.Request.SetBody([]byte(`{"namespace":"telemetryTest","functionName":"ping","data":{"userId":"u1"}}`))
	handler(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("Expected 200 OK despite telemetry panic, got %d", ctx.Response.StatusCode())
	}
	if !panicHookCalled {
		t.Fatalf("Expected telemetry hook to be called")
	}
}

func TestDualModeRoutingProtocolInvariants(t *testing.T) {
	app, err := rpc.New(rpc.Config{
		RpcPath: "/rpc",
		CORS:    &rpc.CorsConfig{AllowOrigins: []string{"*"}},
	})
	if err != nil {
		t.Fatalf("Failed creating app: %v", err)
	}

	rpc.Register(rpc.FunctionConfig[map[string]any, map[string]any]{
		Namespace:    "doc",
		FunctionName: "upload",
		Handler: func(ctx *rpc.Context, input map[string]any) (*map[string]any, error) {
			res := map[string]any{"ok": true}
			return &res, nil
		},
	})
	app.UseGlobalRegistry()
	handler := app.Handler()

	// 1. Partial headers: X-CRPC-Namespace present, X-CRPC-Function missing -> ErrInvalidBody
	{
		var ctx fasthttp.RequestCtx
		ctx.Request.Header.SetMethod("POST")
		ctx.Request.SetRequestURI("/rpc")
		ctx.Request.Header.SetContentType("multipart/form-data; boundary=boundary123")
		ctx.Request.Header.Set("X-CRPC-Namespace", "doc")
		ctx.Request.SetBody([]byte("--boundary123\r\nContent-Disposition: form-data; name=\"namespace\"\r\n\r\ndoc\r\n--boundary123--\r\n"))
		handler(&ctx)

		body := string(ctx.Response.Body())
		if !strings.Contains(body, "invalid_body") || !strings.Contains(body, "Both X-CRPC-Namespace and X-CRPC-Function headers must be provided together") {
			t.Fatalf("Expected partial header rejection with invalid_body, got: %s", body)
		}
	}

	// 2. Routing mismatch: headers specify doc.upload, body specifies admin.delete -> ErrInvalidBody
	{
		var ctx fasthttp.RequestCtx
		ctx.Request.Header.SetMethod("POST")
		ctx.Request.SetRequestURI("/rpc")
		ctx.Request.Header.SetContentType("multipart/form-data; boundary=boundary456")
		ctx.Request.Header.Set("X-CRPC-Namespace", "doc")
		ctx.Request.Header.Set("X-CRPC-Function", "upload")

		mismatchBody := "--boundary456\r\nContent-Disposition: form-data; name=\"namespace\"\r\n\r\nadmin\r\n" +
			"--boundary456\r\nContent-Disposition: form-data; name=\"functionName\"\r\n\r\ndelete\r\n" +
			"--boundary456--\r\n"
		ctx.Request.SetBody([]byte(mismatchBody))
		handler(&ctx)

		body := string(ctx.Response.Body())
		if !strings.Contains(body, "invalid_body") || !strings.Contains(body, "Routing mismatch") {
			t.Fatalf("Expected routing mismatch rejection with invalid_body, got: %s", body)
		}
	}

	// 3. File part arrives before routing fields -> ErrInvalidBody
	{
		var ctx fasthttp.RequestCtx
		ctx.Request.Header.SetMethod("POST")
		ctx.Request.SetRequestURI("/rpc")
		ctx.Request.Header.SetContentType("multipart/form-data; boundary=boundary789")

		fileFirstBody := "--boundary789\r\nContent-Disposition: form-data; name=\"file\"; filename=\"test.txt\"\r\nContent-Type: text/plain\r\n\r\nhello world\r\n" +
			"--boundary789\r\nContent-Disposition: form-data; name=\"namespace\"\r\n\r\ndoc\r\n" +
			"--boundary789\r\nContent-Disposition: form-data; name=\"functionName\"\r\n\r\nupload\r\n" +
			"--boundary789--\r\n"
		ctx.Request.SetBody([]byte(fileFirstBody))
		handler(&ctx)

		body := string(ctx.Response.Body())
		if !strings.Contains(body, "invalid_body") || !strings.Contains(body, "Routing fields ('namespace', 'functionName') must appear before file parts") {
			t.Fatalf("Expected file-before-routing rejection with invalid_body, got: %s", body)
		}
	}

	// 4. Missing body routing fields when headers are supplied -> ErrInvalidBody
	{
		var ctx fasthttp.RequestCtx
		ctx.Request.Header.SetMethod("POST")
		ctx.Request.SetRequestURI("/rpc")
		ctx.Request.Header.SetContentType("multipart/form-data; boundary=boundaryNoBodyRouting")
		ctx.Request.Header.Set("X-CRPC-Namespace", "doc")
		ctx.Request.Header.Set("X-CRPC-Function", "upload")

		// Body contains data field only, omitting namespace and functionName
		noRoutingBody := "--boundaryNoBodyRouting\r\nContent-Disposition: form-data; name=\"title\"\r\n\r\nDocument Title\r\n" +
			"--boundaryNoBodyRouting--\r\n"
		ctx.Request.SetBody([]byte(noRoutingBody))
		handler(&ctx)

		body := string(ctx.Response.Body())
		if !strings.Contains(body, "invalid_body") || !strings.Contains(body, "Routing mismatch") {
			t.Fatalf("Expected rejection when body routing is missing with headers, got: %s", body)
		}
	}
}
