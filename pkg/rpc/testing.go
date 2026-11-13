package rpc

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/CellsWeb/crpc-go/pkg/schema"
)

// TestOptions configures test execution parameters
type TestOptions struct {
	Headers     map[string]string
	ClientIP    string
	ContextData map[string]any
}

// TestCaller provides direct in-process RPC test execution with zero network socket binding
type TestCaller struct {
	app            *CoreRPC
	defaultOptions TestOptions
}

// NewTestCaller initializes a TestCaller
func NewTestCaller(app *CoreRPC, defaultOptions ...TestOptions) *TestCaller {
	opts := TestOptions{}
	if len(defaultOptions) > 0 {
		opts = defaultOptions[0]
	}
	return &TestCaller{
		app:            app,
		defaultOptions: opts,
	}
}

// Call executes the RPC function in-process, unwraps data into outputTarget or returns an AppError
func (tc *TestCaller) Call(functionKey string, input any, outputTarget any) error {
	return tc.app.CallInMemory(functionKey, input, outputTarget, CallOptions{
		Headers:     tc.defaultOptions.Headers,
		ClientIP:    tc.defaultOptions.ClientIP,
		ContextData: tc.defaultOptions.ContextData,
	})
}

// CallRaw executes the RPC function in-process and returns the full ResponseEnvelope (status, code, details, meta)
func (tc *TestCaller) CallRaw(functionKey string, input any, customOpts ...TestOptions) (*ResponseEnvelope, error) {
	opts := tc.defaultOptions
	if len(customOpts) > 0 {
		if customOpts[0].Headers != nil {
			opts.Headers = customOpts[0].Headers
		}
		if customOpts[0].ClientIP != "" {
			opts.ClientIP = customOpts[0].ClientIP
		}
		if customOpts[0].ContextData != nil {
			opts.ContextData = customOpts[0].ContextData
		}
	}

	start := time.Now()
	reqID := generateRequestID()

	tc.app.mu.RLock()
	fn, exists := tc.app.functions[functionKey]
	tc.app.mu.RUnlock()

	if !exists {
		return &ResponseEnvelope{
			Status:  "error",
			Code:    ErrNotFound,
			Message: fmt.Sprintf("Function %q not found", functionKey),
			Meta: ResponseMeta{
				RequestID:  reqID,
				DurationMs: float64(time.Since(start).Microseconds()) / 1000.0,
			},
		}, nil
	}

	ctx := &Context{
		store:     make(map[string]any),
		requestID: reqID,
		headers:   opts.Headers,
		clientIP:  opts.ClientIP,
		path:      tc.app.config.RpcPath,
	}
	if ctx.headers == nil {
		ctx.headers = make(map[string]string)
	}
	for k, v := range opts.ContextData {
		ctx.Set(k, v)
	}

	// Run middlewares
	for _, attachment := range fn.Middlewares {
		mw, ok := tc.app.middlewares[attachment.Name]
		if !ok {
			return &ResponseEnvelope{
				Status:  "error",
				Code:    ErrInternal,
				Message: fmt.Sprintf("Middleware %q missing", attachment.Name),
				Meta: ResponseMeta{
					RequestID:  reqID,
					DurationMs: float64(time.Since(start).Microseconds()) / 1000.0,
				},
			}, nil
		}
		if err := mw.Handler(ctx, ctx.headers, attachment.Options); err != nil {
			code := ErrInternal
			msg := err.Error()
			var details []ValidationErrorDetail
			if appErr, ok := err.(*AppError); ok {
				code = appErr.Code
				msg = appErr.Message
				details = appErr.Details
			}
			return &ResponseEnvelope{
				Status:  "error",
				Code:    code,
				Message: msg,
				Details: details,
				Meta: ResponseMeta{
					RequestID:  reqID,
					DurationMs: float64(time.Since(start).Microseconds()) / 1000.0,
				},
			}, nil
		}
	}

	// Prepare raw input map
	var rawMap map[string]any
	if input != nil {
		switch v := input.(type) {
		case map[string]any:
			rawMap = v
		default:
			bytes, err := json.Marshal(input)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(bytes, &rawMap); err != nil {
				return nil, err
			}
		}
	}

	// Validate input schema if present
	if fn.InputSchema != nil {
		validationErrors := schema.Validate(fn.InputSchema, rawMap, "")
		if len(validationErrors) > 0 {
			return &ResponseEnvelope{
				Status:  "error",
				Code:    ErrValidation,
				Message: fmt.Sprintf("Input validation failed: %d field error(s)", len(validationErrors)),
				Details: validationErrors,
				Meta: ResponseMeta{
					RequestID:  reqID,
					DurationMs: float64(time.Since(start).Microseconds()) / 1000.0,
				},
			}, nil
		}
	}

	// Execute handler
	result, err := fn.Handler(ctx, rawMap)
	duration := float64(time.Since(start).Microseconds()) / 1000.0

	if err != nil {
		code := ErrInternal
		msg := err.Error()
		var details []ValidationErrorDetail
		if appErr, ok := err.(*AppError); ok {
			code = appErr.Code
			msg = appErr.Message
			details = appErr.Details
		}
		return &ResponseEnvelope{
			Status:  "error",
			Code:    code,
			Message: msg,
			Details: details,
			Meta: ResponseMeta{
				RequestID:  reqID,
				DurationMs: duration,
			},
		}, nil
	}

	return &ResponseEnvelope{
		Status: "ok",
		Data:   result,
		Meta: ResponseMeta{
			RequestID:  reqID,
			DurationMs: duration,
		},
	}, nil
}
