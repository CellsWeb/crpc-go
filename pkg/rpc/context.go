package rpc

import (
	"fmt"
	"strings"
	"sync"

	"github.com/valyala/fasthttp"
)

// Context represents the request lifecycle context and dependency injection container
type Context struct {
	mu         sync.RWMutex
	store      map[string]any
	requestID  string
	headers    map[string]string
	clientIP   string
	method     string
	path       string
	rawCtx     *fasthttp.RequestCtx
	files      []*UploadedFile // Tracked for post-request cleanup
}

// NewContext creates a new Context for an incoming fasthttp request
func NewContext(reqCtx *fasthttp.RequestCtx, requestID string, trustProxy bool) *Context {
	ctx := &Context{
		store:     make(map[string]any),
		requestID: requestID,
		rawCtx:    reqCtx,
	}

	if reqCtx != nil {
		ctx.method = string(reqCtx.Method())
		ctx.path = string(reqCtx.Path())

		// Extract client IP
		if trustProxy {
			xForwardedFor := string(reqCtx.Request.Header.Peek("X-Forwarded-For"))
			if xForwardedFor != "" {
				parts := strings.Split(xForwardedFor, ",")
				ctx.clientIP = strings.TrimSpace(parts[0])
			}
		}
		if ctx.clientIP == "" {
			ctx.clientIP = reqCtx.RemoteIP().String()
		}

		// Extract all headers
		ctx.headers = make(map[string]string)
		reqCtx.Request.Header.VisitAll(func(key, value []byte) {
			ctx.headers[strings.ToLower(string(key))] = string(value)
		})
	} else {
		ctx.headers = make(map[string]string)
	}

	return ctx
}

// RequestID returns the unique correlation ID for this request
func (c *Context) RequestID() string {
	return c.requestID
}

// ClientIP returns the client IP address
func (c *Context) ClientIP() string {
	return c.clientIP
}

// Header returns a request header by case-insensitive name
func (c *Context) Header(name string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.headers[strings.ToLower(name)]
}

// Headers returns a copy of all request headers
func (c *Context) Headers() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	res := make(map[string]string, len(c.headers))
	for k, v := range c.headers {
		res[k] = v
	}
	return res
}

// Set stores a value in the request context (dependency injection)
func (c *Context) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = value
}

// Get retrieves a value from the request context
func (c *Context) Get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.store[key]
	return val, ok
}

// MustGet retrieves a value or panics if it does not exist
func (c *Context) MustGet(key string) any {
	val, ok := c.Get(key)
	if !ok {
		panic(fmt.Sprintf("rpc.Context: key %q not found in store", key))
	}
	return val
}

// GetContext is a generic helper to retrieve a strictly-typed context value
func GetContext[T any](c *Context, key string) (T, bool) {
	val, ok := c.Get(key)
	if !ok {
		var zero T
		return zero, false
	}
	typed, ok := val.(T)
	if !ok {
		var zero T
		return zero, false
	}
	return typed, true
}

// TrackFile registers an uploaded file for automatic cleanup when the request completes
func (c *Context) TrackFile(f *UploadedFile) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.files = append(c.files, f)
}

// Cleanup releases all temporary files associated with this context
func (c *Context) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, f := range c.files {
		f.Cleanup()
	}
	c.files = nil
}

// Raw returns the underlying fasthttp RequestCtx (or nil for in-process tests)
func (c *Context) Raw() *fasthttp.RequestCtx {
	return c.rawCtx
}
