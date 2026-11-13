package rpc

import (
	"fmt"
	"strings"

	"github.com/valyala/fasthttp"
)

// HTTPHandlerFunc represents a standard custom route handler (Express/Fiber style)
type HTTPHandlerFunc func(c *HTTPContext) error

// HTTPContext wraps fasthttp.RequestCtx with Express/Fiber-like helper methods
type HTTPContext struct {
	raw *fasthttp.RequestCtx
	app *CoreRPC
}

// NewHTTPContext creates an HTTPContext wrapper
func NewHTTPContext(reqCtx *fasthttp.RequestCtx, app *CoreRPC) *HTTPContext {
	return &HTTPContext{
		raw: reqCtx,
		app: app,
	}
}

// Method returns the HTTP method
func (c *HTTPContext) Method() string {
	return string(c.raw.Method())
}

// Path returns the requested URL path
func (c *HTTPContext) Path() string {
	return string(c.raw.Path())
}

// Query returns a query string parameter by key
func (c *HTTPContext) Query(key string) string {
	return string(c.raw.QueryArgs().Peek(key))
}

// Header returns a request header by key
func (c *HTTPContext) Header(key string) string {
	return string(c.raw.Request.Header.Peek(key))
}

// Body returns the raw request body bytes
func (c *HTTPContext) Body() []byte {
	return c.raw.PostBody()
}

// SetHeader sets an HTTP response header
func (c *HTTPContext) SetHeader(key, value string) {
	c.raw.Response.Header.Set(key, value)
}

// Status sets the HTTP response status code
func (c *HTTPContext) Status(code int) *HTTPContext {
	c.raw.SetStatusCode(code)
	return c
}

// SendString sends a plain text response
func (c *HTTPContext) SendString(str string) error {
	c.raw.SetContentType("text/plain; charset=utf-8")
	c.raw.SetBodyString(str)
	return nil
}

// JSON sends a JSON response
func (c *HTTPContext) JSON(data any) error {
	c.raw.SetContentType("application/json")
	return jsonMarshalToCtx(c.raw, data)
}

// Redirect sends an HTTP redirect
func (c *HTTPContext) Redirect(location string, statusCode int) error {
	c.raw.Redirect(location, statusCode)
	return nil
}

// Raw returns the underlying fasthttp RequestCtx
func (c *HTTPContext) Raw() *fasthttp.RequestCtx {
	return c.raw
}

// Call executes an RPC function directly in-memory from within a custom HTTP route handler
func (c *HTTPContext) Call(functionKey string, input any, outputTarget any, opts ...CallOptions) error {
	var opt CallOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	// Auto-propagate headers from the current HTTP request if not explicitly provided
	if opt.Headers == nil {
		opt.Headers = make(map[string]string)
		c.raw.Request.Header.VisitAll(func(k, v []byte) {
			opt.Headers[strings.ToLower(string(k))] = string(v)
		})
	}

	return c.app.CallInMemory(functionKey, input, outputTarget, opt)
}

// RouteKey creates a composite lookup key for method and path
func routeKey(method, path string) string {
	return fmt.Sprintf("%s:%s", strings.ToUpper(method), path)
}
