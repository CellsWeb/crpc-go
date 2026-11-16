package rpc

import (
	"encoding/json"
	"fmt"

	"github.com/CellsWeb/crpc-go/pkg/schema"
)

// CallOptions configures in-memory RPC execution parameters
type CallOptions struct {
	Headers  map[string]string
	ClientIP string
}

// CallInMemory executes an RPC function in-process without network overhead
func (app *CoreRPC) CallInMemory(functionKey string, input any, outputTarget any, opts ...CallOptions) error {
	var opt CallOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	app.mu.RLock()
	fn, exists := app.functions[functionKey]
	app.mu.RUnlock()

	if !exists {
		return NewError(ErrNotFound, fmt.Sprintf("Function %q not found in registry", functionKey))
	}

	// Build context
	ctx := &Context{
		store:     make(map[string]any),
		requestID: generateRequestID(),
		headers:   opt.Headers,
		clientIP:  opt.ClientIP,
		path:      app.config.RpcPath,
	}
	if ctx.headers == nil {
		ctx.headers = make(map[string]string)
	}

	// Execute middlewares (always enforced across all transports)
	if len(fn.Middlewares) > 0 {
		for _, attachment := range fn.Middlewares {
			mw, ok := app.middlewares[attachment.Name]
			if !ok {
				return NewError(ErrInternal, fmt.Sprintf("Middleware %q configured for %q not found", attachment.Name, functionKey))
			}
			if err := mw.Handler(ctx, ctx.headers, attachment.Options); err != nil {
				return err
			}
		}
	}

	// Preserve arbitrary JSON shapes while retaining native values in non-nil maps.
	var rawData any
	if input != nil {
		switch v := input.(type) {
		case map[string]any:
			if v != nil {
				rawData = v
			}
		default:
			bytes, err := json.Marshal(input)
			if err != nil {
				return NewError(ErrInvalidBody, fmt.Sprintf("Failed to marshal input: %v", err))
			}
			if err := json.Unmarshal(bytes, &rawData); err != nil {
				return NewError(ErrInvalidBody, fmt.Sprintf("Failed to parse input: %v", err))
			}
		}
	}

	// Validate input schema
	if fn.InputSchema != nil {
		if valErrors := schema.Validate(fn.InputSchema, rawData, ""); len(valErrors) > 0 {
			return NewError(ErrValidation, fmt.Sprintf("Input validation failed: %d error(s)", len(valErrors)))
		}
	}

	// Execute function handler
	result, err := fn.Handler(ctx, rawData)
	if err != nil {
		return err
	}

	// Validate output schema
	if fn.OutputSchema != nil && result != nil {
		if valErrors := schema.Validate(fn.OutputSchema, result, ""); len(valErrors) > 0 {
			return NewError(ErrInternal, fmt.Sprintf("Handler output schema violation: %d error(s)", len(valErrors)))
		}
	}

	// Marshal result into output target if provided
	if outputTarget != nil && result != nil {
		bytes, err := json.Marshal(result)
		if err != nil {
			return NewError(ErrInternal, fmt.Sprintf("Failed to marshal output: %v", err))
		}
		if err := json.Unmarshal(bytes, outputTarget); err != nil {
			return NewError(ErrInternal, fmt.Sprintf("Failed to unmarshal into output target: %v", err))
		}
	}

	return nil
}
