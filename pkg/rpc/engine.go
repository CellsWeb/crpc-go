package rpc

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/CellsWeb/crpc-go/pkg/schema"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

var reqCounter uint64

func generateRequestID() string {
	count := atomic.AddUint64(&reqCounter, 1)
	return fmt.Sprintf("crpc_%d_%d", time.Now().UnixNano(), count)
}

// CoreRPC is the high-performance RPC server engine built on fasthttp
type CoreRPC struct {
	config      Config
	mu          sync.RWMutex
	functions   map[string]*FunctionDefinition
	middlewares map[string]*RegisteredMiddleware
	httpRoutes  map[string]HTTPHandlerFunc
	server      *fasthttp.Server
	listener    net.Listener
}

// New creates and strictly validates a new CoreRPC server instance
func New(config Config) (*CoreRPC, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	app := &CoreRPC{
		config:      config,
		functions:   make(map[string]*FunctionDefinition),
		middlewares: make(map[string]*RegisteredMiddleware),
		httpRoutes:  make(map[string]HTTPHandlerFunc),
	}

	return app, nil
}

// UseGlobalRegistry imports all functions and middlewares registered via package init()
func (app *CoreRPC) UseGlobalRegistry() *CoreRPC {
	globalMu.RLock()
	defer globalMu.RUnlock()

	app.mu.Lock()
	defer app.mu.Unlock()

	for k, v := range globalFunctions {
		app.functions[k] = v
	}
	for k, v := range globalMws {
		app.middlewares[k] = v
	}

	return app
}

// RegisterFunction registers a function directly on the server instance
func (app *CoreRPC) RegisterFunction(fn *FunctionDefinition) *CoreRPC {
	app.mu.Lock()
	defer app.mu.Unlock()
	key := FormatFunctionKey(fn.Namespace, fn.FunctionName)
	fn.Key = key
	app.functions[key] = fn
	return app
}

// RegisterMiddleware registers a reusable middleware on the server instance
func (app *CoreRPC) RegisterMiddleware(name, description string, fn MiddlewareFn, optionsSchema *schema.SchemaRule, headersSchema *schema.SchemaRule) *CoreRPC {
	app.mu.Lock()
	defer app.mu.Unlock()
	app.middlewares[name] = &RegisteredMiddleware{
		Metadata: MiddlewareMetadata{
			Name:        name,
			Description: description,
			Options:     optionsSchema,
			Headers:     headersSchema,
		},
		Handler: fn,
	}
	return app
}

// Custom HTTP Route Registration (Express/Fiber style)

func (app *CoreRPC) GET(path string, handler HTTPHandlerFunc) *CoreRPC {
	app.httpRoutes[routeKey("GET", path)] = handler
	return app
}

func (app *CoreRPC) POST(path string, handler HTTPHandlerFunc) *CoreRPC {
	app.httpRoutes[routeKey("POST", path)] = handler
	return app
}

func (app *CoreRPC) PUT(path string, handler HTTPHandlerFunc) *CoreRPC {
	app.httpRoutes[routeKey("PUT", path)] = handler
	return app
}

func (app *CoreRPC) DELETE(path string, handler HTTPHandlerFunc) *CoreRPC {
	app.httpRoutes[routeKey("DELETE", path)] = handler
	return app
}

// Functions returns a copy of all registered functions (used by docs engine)
func (app *CoreRPC) Functions() map[string]*FunctionDefinition {
	app.mu.RLock()
	defer app.mu.RUnlock()
	res := make(map[string]*FunctionDefinition, len(app.functions))
	for k, v := range app.functions {
		res[k] = v
	}
	return res
}

// Middlewares returns a copy of all registered middlewares (used by docs engine)
func (app *CoreRPC) Middlewares() map[string]*RegisteredMiddleware {
	app.mu.RLock()
	defer app.mu.RUnlock()
	res := make(map[string]*RegisteredMiddleware, len(app.middlewares))
	for k, v := range app.middlewares {
		res[k] = v
	}
	return res
}

// Handler returns the primary fasthttp RequestHandler
func (app *CoreRPC) Handler() fasthttp.RequestHandler {
	return func(reqCtx *fasthttp.RequestCtx) {
		method := string(reqCtx.Method())
		path := string(reqCtx.Path())

		// 1. Handle CORS Preflight OPTIONS
		if method == "OPTIONS" {
			app.handleCorsPreflight(reqCtx)
			return
		}

		// Apply standard CORS headers
		app.applyCorsHeaders(reqCtx)

		// 2. Custom HTTP Routes
		routeHandler, exists := app.httpRoutes[routeKey(method, path)]
		if !exists {
			for rKey, h := range app.httpRoutes {
				if strings.HasSuffix(rKey, "/*") {
					prefix := strings.TrimSuffix(rKey, "/*")
					current := routeKey(method, path)
					if strings.HasPrefix(current, prefix+"/") || current == prefix {
						routeHandler = h
						exists = true
						break
					}
				}
			}
		}

		if exists {
			httpCtx := NewHTTPContext(reqCtx, app)
			if err := routeHandler(httpCtx); err != nil {
				reqCtx.SetStatusCode(fasthttp.StatusInternalServerError)
				reqCtx.SetBodyString(err.Error())
			}
			return
		}

		// 3. Documentation UI Route
		if app.config.DocsEnabled && (path == app.config.DocsPath || strings.HasPrefix(path, app.config.DocsPath+"/")) {
			app.handleDocsRequest(reqCtx)
			return
		}

		// 4. Primary RPC Endpoint
		if path == app.config.RpcPath && method == "POST" {
			app.handleRpcRequest(reqCtx)
			return
		}

		// 404 Not Found for non-matching endpoints
		reqCtx.SetStatusCode(fasthttp.StatusNotFound)
		reqCtx.SetContentType("application/json")
		_ = jsonMarshalToCtx(reqCtx, map[string]string{
			"error": fmt.Sprintf("Cannot %s %s", method, path),
		})
	}
}

// NetHTTPHandler provides standard net/http.Handler compatibility
func (app *CoreRPC) NetHTTPHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if app.config.MaxRequestBodySize > 0 && r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, int64(app.config.MaxRequestBodySize))
		}
		var reqCtx fasthttp.RequestCtx
		fasthttpadaptor.ConvertNetHTTPRequestToFastHTTPRequest(r, &reqCtx.Request)
		app.Handler()(&reqCtx)
		reqCtx.Response.Header.VisitAll(func(k, v []byte) {
			w.Header().Set(string(k), string(v))
		})
		w.WriteHeader(reqCtx.Response.StatusCode())
		_, _ = w.Write(reqCtx.Response.Body())
	})
}

// Primary RPC Dispatcher
func (app *CoreRPC) handleRpcRequest(reqCtx *fasthttp.RequestCtx) {
	start := time.Now()
	requestID := generateRequestID()
	ctx := NewContext(reqCtx, requestID, app.config.TrustProxy)
	defer ctx.Cleanup()

	cType := string(reqCtx.Request.Header.ContentType())

	if strings.HasPrefix(cType, "multipart/form-data") {
		app.handleMultipartRPC(reqCtx, ctx, start)
		return
	}

	if cType == "" || strings.HasPrefix(cType, "application/json") {
		app.handleJsonRPC(reqCtx, ctx, start)
		return
	}

	app.sendErrorResponse(reqCtx, ctx.RequestID(), ErrInvalidContentType, fmt.Sprintf("Unsupported Content-Type %q: expected application/json or multipart/form-data", cType), nil, start)
}

// JSON RPC Dispatcher
func (app *CoreRPC) handleJsonRPC(reqCtx *fasthttp.RequestCtx, ctx *Context, start time.Time) {
	// Safe buffer cloning from fasthttp request pool to avoid race conditions in goroutines
	bodyBytes := reqCtx.PostBody()
	if len(bodyBytes) == 0 {
		app.sendErrorResponse(reqCtx, ctx.RequestID(), ErrInvalidBody, "Request body is empty", nil, start)
		return
	}

	// Parse JSON RPC payload envelope
	var payload struct {
		Namespace    string `json:"namespace"`
		FunctionName string `json:"functionName"`
		Data         any    `json:"data"`
	}

	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		app.sendErrorResponse(reqCtx, ctx.RequestID(), ErrInvalidBody, fmt.Sprintf("Malformed JSON body: %v", err), nil, start)
		return
	}

	payload.Namespace = strings.TrimSpace(payload.Namespace)
	payload.FunctionName = strings.TrimSpace(payload.FunctionName)

	if payload.Namespace == "" || payload.FunctionName == "" {
		app.sendErrorResponse(reqCtx, ctx.RequestID(), ErrInvalidBody, "Both 'namespace' and 'functionName' are required", nil, start)
		return
	}

	if !isValidRoutingIdentifier(payload.Namespace) || !isValidRoutingIdentifier(payload.FunctionName) {
		app.sendErrorResponse(reqCtx, ctx.RequestID(), ErrInvalidBody, "Invalid characters in namespace or functionName identifier", nil, start)
		return
	}

	if payload.Data == nil {
		payload.Data = make(map[string]any)
	}

	functionKey := FormatFunctionKey(payload.Namespace, payload.FunctionName)

	app.mu.RLock()
	fn, exists := app.functions[functionKey]
	app.mu.RUnlock()

	if !exists {
		app.sendErrorResponse(reqCtx, ctx.RequestID(), ErrFunctionNotFound, fmt.Sprintf("Function %q not found in registry", functionKey), nil, start)
		return
	}

	// Execute middlewares (Fail-closed)
	for _, attachment := range fn.Middlewares {
		mw, ok := app.middlewares[attachment.Name]
		if !ok {
			app.sendErrorResponse(reqCtx, ctx.RequestID(), ErrInternal, fmt.Sprintf("Configured middleware %q missing", attachment.Name), nil, start)
			return
		}
		if err := mw.Handler(ctx, ctx.headers, attachment.Options); err != nil {
			app.dispatchError(reqCtx, ctx.RequestID(), err, start)
			return
		}
	}

	// Validate input schema
	if fn.InputSchema != nil {
		validationErrors := schema.Validate(fn.InputSchema, payload.Data, "")
		if len(validationErrors) > 0 {
			app.sendErrorResponse(reqCtx, ctx.RequestID(), ErrValidation, fmt.Sprintf("Input validation failed: %d field error(s)", len(validationErrors)), validationErrors, start)
			return
		}
	}

	// Execute business handler
	result, err := fn.Handler(ctx, payload.Data)
	if err != nil {
		app.dispatchError(reqCtx, ctx.RequestID(), err, start)
		return
	}

	// Validate output schema
	if fn.OutputSchema != nil && result != nil {
		if valErrors := schema.Validate(fn.OutputSchema, result, ""); len(valErrors) > 0 {
			app.sendErrorResponse(reqCtx, ctx.RequestID(), ErrSchemaMismatch, fmt.Sprintf("Handler output schema violation: %d error(s)", len(valErrors)), valErrors, start)
			return
		}
	}

	app.sendSuccessResponse(reqCtx, ctx.RequestID(), result, start)
}

// Multipart Form-Data RPC Dispatcher (Multi-phase)
func (app *CoreRPC) handleMultipartRPC(reqCtx *fasthttp.RequestCtx, ctx *Context, start time.Time) {
	headerNs := strings.TrimSpace(string(reqCtx.Request.Header.Peek("X-CRPC-Namespace")))
	headerFn := strings.TrimSpace(string(reqCtx.Request.Header.Peek("X-CRPC-Function")))

	if (headerNs != "" && headerFn == "") || (headerNs == "" && headerFn != "") {
		app.sendErrorResponse(reqCtx, ctx.RequestID(), ErrInvalidBody, "Both X-CRPC-Namespace and X-CRPC-Function headers must be provided together, or both omitted", nil, start)
		return
	}

	var ns, fnName string
	var err error

	if headerNs != "" && headerFn != "" {
		if !isValidRoutingIdentifier(headerNs) || !isValidRoutingIdentifier(headerFn) {
			app.sendErrorResponse(reqCtx, ctx.RequestID(), ErrInvalidBody, "Invalid characters in X-CRPC-Namespace or X-CRPC-Function header", nil, start)
			return
		}
		ns = headerNs
		fnName = headerFn
	} else {
		// Phase 1 fallback: Quick extract namespace & functionName from body stream
		ns, fnName, err = ParseMultipartPhase1(reqCtx)
		if err != nil {
			app.dispatchError(reqCtx, ctx.RequestID(), err, start)
			return
		}
	}

	functionKey := FormatFunctionKey(ns, fnName)

	app.mu.RLock()
	fn, exists := app.functions[functionKey]
	app.mu.RUnlock()

	if !exists {
		app.sendErrorResponse(reqCtx, ctx.RequestID(), ErrFunctionNotFound, fmt.Sprintf("Function %q not found in registry", functionKey), nil, start)
		return
	}

	// Phase 2: Execute middlewares BEFORE reading/processing heavy files
	for _, attachment := range fn.Middlewares {
		mw, ok := app.middlewares[attachment.Name]
		if !ok {
			app.sendErrorResponse(reqCtx, ctx.RequestID(), ErrInternal, fmt.Sprintf("Configured middleware %q missing", attachment.Name), nil, start)
			return
		}
		if err := mw.Handler(ctx, ctx.headers, attachment.Options); err != nil {
			app.dispatchError(reqCtx, ctx.RequestID(), err, start)
			return
		}
	}

	// Phase 3: Extract form data and files (stream directly to disk temp storage)
	parsed, err := ParseMultipartPhase2(reqCtx, int64(app.config.MaxRequestBodySize), app.config.MaxMultipartFiles)
	if err != nil {
		app.dispatchError(reqCtx, ctx.RequestID(), err, start)
		return
	}

	// Track all extracted temporary files immediately so ctx.Cleanup removes them on any failure
	for _, f := range parsed.Files {
		ctx.TrackFile(f)
	}

	// Invariant: Verify consistency between headers and body if headers are used (redundant dual-mode standard)
	if headerNs != "" && (parsed.Namespace != headerNs || parsed.FunctionName != headerFn) {
		app.sendErrorResponse(reqCtx, ctx.RequestID(), ErrInvalidBody, fmt.Sprintf("Routing mismatch: headers specify %q but payload specifies %q", FormatFunctionKey(headerNs, headerFn), FormatFunctionKey(parsed.Namespace, parsed.FunctionName)), nil, start)
		return
	}

	// Validate input schema
	if fn.InputSchema != nil {
		validationErrors := schema.Validate(fn.InputSchema, parsed.Data, "")
		if len(validationErrors) > 0 {
			app.sendErrorResponse(reqCtx, ctx.RequestID(), ErrValidation, fmt.Sprintf("Input validation failed: %d field error(s)", len(validationErrors)), validationErrors, start)
			return
		}
	}

	// Execute handler
	result, err := fn.Handler(ctx, parsed.Data)
	if err != nil {
		app.dispatchError(reqCtx, ctx.RequestID(), err, start)
		return
	}

	// Validate output schema
	if fn.OutputSchema != nil && result != nil {
		if valErrors := schema.Validate(fn.OutputSchema, result, ""); len(valErrors) > 0 {
			app.sendErrorResponse(reqCtx, ctx.RequestID(), ErrSchemaMismatch, fmt.Sprintf("Handler output schema violation: %d error(s)", len(valErrors)), valErrors, start)
			return
		}
	}

	app.sendSuccessResponse(reqCtx, ctx.RequestID(), result, start)
}

// Response Enveloping (Uniform HTTP 200)

func (app *CoreRPC) triggerTelemetry(event TelemetryEvent) {
	for _, hook := range app.config.TelemetryHooks {
		if hook != nil {
			func() {
				defer func() {
					_ = recover()
				}()
				hook(event)
			}()
		}
	}
}

func (app *CoreRPC) sendSuccessResponse(reqCtx *fasthttp.RequestCtx, requestID string, data any, start time.Time) {
	duration := float64(time.Since(start).Microseconds()) / 1000.0
	envelope := ResponseEnvelope{
		Status: "ok",
		Data:   data,
		Meta: ResponseMeta{
			RequestID:  requestID,
			DurationMs: duration,
		},
	}
	reqCtx.SetStatusCode(fasthttp.StatusOK)
	reqCtx.SetContentType("application/json")
	if err := jsonMarshalToCtx(reqCtx, envelope); err != nil {
		reqCtx.SetBodyString(`{"status":"error","code":"internal_error","message":"Failed to serialize response"}`)
	}

	if len(app.config.TelemetryHooks) > 0 {
		app.triggerTelemetry(TelemetryEvent{
			RequestID:  requestID,
			DurationMs: duration,
			Status:     "ok",
			ClientIP:   reqCtx.RemoteIP().String(),
			UserAgent:  string(reqCtx.Request.Header.UserAgent()),
		})
	}
}

func (app *CoreRPC) sendErrorResponse(reqCtx *fasthttp.RequestCtx, requestID, code, message string, details []ValidationErrorDetail, start time.Time) {
	duration := float64(time.Since(start).Microseconds()) / 1000.0
	envelope := ResponseEnvelope{
		Status:  "error",
		Code:    code,
		Message: message,
		Details: details,
		Meta: ResponseMeta{
			RequestID:  requestID,
			DurationMs: duration,
		},
	}
	statusCode := fasthttp.StatusOK
	if app.config.UseHttpStatusCodes {
		statusCode = mapErrorCodeToHTTPStatus(code)
	}
	reqCtx.SetStatusCode(statusCode)
	reqCtx.SetContentType("application/json")
	if err := jsonMarshalToCtx(reqCtx, envelope); err != nil {
		reqCtx.SetBodyString(`{"status":"error","code":"internal_error","message":"Failed to serialize response"}`)
	}

	if len(app.config.TelemetryHooks) > 0 {
		app.triggerTelemetry(TelemetryEvent{
			RequestID:  requestID,
			DurationMs: duration,
			Status:     "error",
			ErrorCode:  code,
			ClientIP:   reqCtx.RemoteIP().String(),
			UserAgent:  string(reqCtx.Request.Header.UserAgent()),
			Error:      fmt.Errorf("%s: %s", code, message),
		})
	}
}

func mapErrorCodeToHTTPStatus(code string) int {
	switch code {
	case ErrValidation, ErrInvalidBody, ErrInvalidContentType:
		return fasthttp.StatusBadRequest
	case ErrUnauthorized:
		return fasthttp.StatusUnauthorized
	case ErrForbidden:
		return fasthttp.StatusForbidden
	case ErrNotFound, ErrFunctionNotFound:
		return fasthttp.StatusNotFound
	case ErrConflict:
		return fasthttp.StatusConflict
	case ErrPayloadTooLarge:
		return fasthttp.StatusRequestEntityTooLarge
	case ErrRateLimited:
		return fasthttp.StatusTooManyRequests
	case ErrTimeout:
		return fasthttp.StatusGatewayTimeout
	case ErrInternal, ErrSchemaMismatch:
		return fasthttp.StatusInternalServerError
	default:
		return fasthttp.StatusInternalServerError
	}
}

func (app *CoreRPC) dispatchError(reqCtx *fasthttp.RequestCtx, requestID string, err error, start time.Time) {
	code := ErrInternal
	msg := err.Error()
	var details []ValidationErrorDetail

	var appErr *AppError
	if errors.As(err, &appErr) {
		code = appErr.Code
		msg = appErr.Message
		details = appErr.Details
	}

	app.sendErrorResponse(reqCtx, requestID, code, msg, details, start)
}

// CORS Helpers

func (app *CoreRPC) applyCorsHeaders(reqCtx *fasthttp.RequestCtx) {
	cors := app.config.CORS
	if cors == nil {
		return
	}

	origin := string(reqCtx.Request.Header.Peek("Origin"))
	if len(cors.AllowOrigins) > 0 {
		allowed := false
		for _, o := range cors.AllowOrigins {
			if o == origin || (o == "*" && !cors.AllowCredentials) {
				allowed = true
				break
			}
		}
		if allowed {
			if len(cors.AllowOrigins) == 1 && cors.AllowOrigins[0] == "*" && !cors.AllowCredentials {
				reqCtx.Response.Header.Set("Access-Control-Allow-Origin", "*")
			} else {
				reqCtx.Response.Header.Set("Access-Control-Allow-Origin", origin)
				reqCtx.Response.Header.Add("Vary", "Origin")
			}
		}
	}

	if cors.AllowCredentials {
		reqCtx.Response.Header.Set("Access-Control-Allow-Credentials", "true")
	}
}

func (app *CoreRPC) handleCorsPreflight(reqCtx *fasthttp.RequestCtx) {
	app.applyCorsHeaders(reqCtx)
	cors := app.config.CORS
	if cors != nil {
		if len(cors.AllowMethods) > 0 {
			reqCtx.Response.Header.Set("Access-Control-Allow-Methods", strings.Join(cors.AllowMethods, ", "))
		} else {
			reqCtx.Response.Header.Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		}

		if len(cors.AllowHeaders) > 0 {
			reqCtx.Response.Header.Set("Access-Control-Allow-Headers", strings.Join(cors.AllowHeaders, ", "))
		} else {
			reqCtx.Response.Header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, X-CRPC-Namespace, X-CRPC-Function")
		}

		if cors.MaxAge > 0 {
			reqCtx.Response.Header.Set("Access-Control-Max-Age", fmt.Sprintf("%d", int(cors.MaxAge.Seconds())))
		}
	}
	reqCtx.SetStatusCode(fasthttp.StatusNoContent)
}

// JSON Serialization via pooled buffer using Go 1.27 encoding/json/v2
func jsonMarshalToCtx(reqCtx *fasthttp.RequestCtx, v any) error {
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	if err := json.MarshalWrite(buf, v); err != nil {
		return err
	}
	reqCtx.SetBody(buf.Bytes())
	return nil
}

// Serve starts the server on the provided listener without installing OS signal handlers
func (app *CoreRPC) Serve(ln net.Listener) error {
	app.listener = ln
	if app.server == nil {
		app.server = &fasthttp.Server{
			Handler:            app.Handler(),
			MaxRequestBodySize: app.config.MaxRequestBodySize,
			ReadTimeout:        app.config.ReadTimeout,
			WriteTimeout:       app.config.WriteTimeout,
			IdleTimeout:        app.config.IdleTimeout,
		}
	}
	return app.server.Serve(ln)
}

// ListenAndServe starts the server with graceful shutdown handling
func (app *CoreRPC) ListenAndServe() error {
	addr := fmt.Sprintf("%s:%d", app.config.Host, app.config.Port)
	ln, err := net.Listen("tcp4", addr)
	if err != nil {
		return err
	}

	app.server = &fasthttp.Server{
		Handler:            app.Handler(),
		MaxRequestBodySize: app.config.MaxRequestBodySize,
		ReadTimeout:        app.config.ReadTimeout,
		WriteTimeout:       app.config.WriteTimeout,
		IdleTimeout:        app.config.IdleTimeout,
	}

	// Graceful shutdown channel
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(shutdown)

	go func() {
		<-shutdown
		_ = app.Shutdown()
	}()

	return app.Serve(ln)
}

var validIdentifierRegex = regexp.MustCompile(`^[a-zA-Z0-9_]+([.-][a-zA-Z0-9_]+)*$`)

func isValidRoutingIdentifier(s string) bool {
	if len(s) == 0 || len(s) > 128 {
		return false
	}
	if s == "__proto__" || s == "constructor" || s == "prototype" {
		return false
	}
	return validIdentifierRegex.MatchString(s)
}

// Shutdown gracefully terminates the server
func (app *CoreRPC) Shutdown() error {
	if app.server != nil {
		return app.server.Shutdown()
	}
	return nil
}
