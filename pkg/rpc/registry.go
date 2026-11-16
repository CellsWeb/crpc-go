package rpc

import (
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/CellsWeb/crpc-go/pkg/schema"
)

// MiddlewareAttachment defines a named middleware with options attached to a function
type MiddlewareAttachment struct {
	Name    string         `json:"name"`
	Options map[string]any `json:"options"`
}

// M is a clean helper to define a middleware attachment with zero boilerplate
func M(name string, options map[string]any) MiddlewareAttachment {
	if options == nil {
		options = make(map[string]any)
	}
	return MiddlewareAttachment{
		Name:    name,
		Options: options,
	}
}

// DefineMiddlewares creates a slice of MiddlewareAttachments
func DefineMiddlewares(items ...MiddlewareAttachment) []MiddlewareAttachment {
	return items
}

// MiddlewareFn is the executable middleware handler function
type MiddlewareFn func(ctx *Context, headers map[string]string, options map[string]any) error

// MiddlewareMetadata contains descriptive documentation for the /docs UI
type MiddlewareMetadata struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Options     *schema.SchemaRule `json:"options,omitempty"`
	Headers     *schema.SchemaRule `json:"headers,omitempty"`
}

// RegisteredMiddleware holds the execution handler and metadata
type RegisteredMiddleware struct {
	Metadata MiddlewareMetadata
	Handler  MiddlewareFn
}

// FunctionMetadata contains descriptive documentation and operational flags for the /docs UI
type FunctionMetadata struct {
	Description string   `json:"description,omitempty"`
	IsPublic    bool     `json:"isPublic,omitempty"`
	Deprecated  bool     `json:"deprecated,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// FunctionDefinition holds metadata, schemas, and execution handlers
type FunctionDefinition struct {
	Namespace    string
	FunctionName string
	Key          string // e.g. "user.getProfile" or "admin.catalog.products.create"
	ContentType  string // "application/json" or "multipart/form-data"
	Metadata     FunctionMetadata
	InputSchema  *schema.SchemaRule
	OutputSchema *schema.SchemaRule
	Middlewares  []MiddlewareAttachment
	Handler      func(ctx *Context, rawInput any) (any, error)
	InputType    reflect.Type
}

// Global Registry instance for package init() self-registration
var (
	globalMu         sync.RWMutex
	globalFunctions  = make(map[string]*FunctionDefinition)
	globalMws        = make(map[string]*RegisteredMiddleware)
)

// RegisterFunction adds a function to the global registry (usable in package init())
func RegisterFunction(fn *FunctionDefinition) {
	globalMu.Lock()
	defer globalMu.Unlock()
	key := FormatFunctionKey(fn.Namespace, fn.FunctionName)
	fn.Key = key
	globalFunctions[key] = fn
}

// RegisterMiddleware registers a reusable middleware in the global registry
func RegisterMiddleware(name, description string, fn MiddlewareFn, optionsSchema *schema.SchemaRule, headersSchema *schema.SchemaRule) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalMws[name] = &RegisteredMiddleware{
		Metadata: MiddlewareMetadata{
			Name:        name,
			Description: description,
			Options:     optionsSchema,
			Headers:     headersSchema,
		},
		Handler: fn,
	}
}

// FormatFunctionKey computes the unique lookup key supporting both standard and nested dot-names
func FormatFunctionKey(namespace, functionName string) string {
	ns := strings.TrimSpace(namespace)
	fn := strings.TrimSpace(functionName)
	if ns == "" {
		return fn
	}
	return fmt.Sprintf("%s.%s", ns, fn)
}

// SplitFunctionKey parses a combined key "admin.products.create" into namespace and functionName
func SplitFunctionKey(key string) (string, string) {
	idx := strings.LastIndex(key, ".")
	if idx == -1 {
		return "", key
	}
	return key[:idx], key[idx+1:]
}
