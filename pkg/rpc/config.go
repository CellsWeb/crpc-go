package rpc

import (
	"fmt"
	"strings"
	"time"
)

// CorsConfig defines explicit CORS access control policies
type CorsConfig struct {
	AllowOrigins     []string      `json:"allowOrigins"`     // Allowed origins, e.g. ["http://localhost:3000"]
	AllowMethods     []string      `json:"allowMethods"`     // Allowed methods, e.g. ["POST", "OPTIONS"]
	AllowHeaders     []string      `json:"allowHeaders"`     // Allowed headers, e.g. ["Content-Type", "Authorization"]
	AllowCredentials bool          `json:"allowCredentials"` // Access-Control-Allow-Credentials
	MaxAge           time.Duration `json:"maxAge"`           // Preflight cache duration
}

// TelemetryHandler is invoked for request metrics, logging, or crash reporting
type TelemetryEvent struct {
	RequestID  string
	Namespace  string
	Function   string
	DurationMs float64
	Status     string // "ok" or "error"
	ErrorCode  string
	ClientIP   string
	UserAgent  string
	Error      error
}

type TelemetryHook func(event TelemetryEvent)

// Config defines the strict configuration for CoreRPC
type Config struct {
	// RpcPath is the required endpoint path (e.g. "/rpc"). Must be non-empty. No silent fallback.
	RpcPath string

	// Port is the listening port (e.g. 4000)
	Port int

	// Host to bind to (e.g. "0.0.0.0" or "127.0.0.1")
	Host string

	// CORS is the required CORS policy. Must be explicitly provided. No silent fallback.
	CORS *CorsConfig

	// DocsEnabled controls whether the interactive documentation UI (/docs) is active
	DocsEnabled bool

	// DocsPath is the custom path for docs (e.g. "/docs"). Defaults to "/docs" if DocsEnabled is true.
	DocsPath string

	// MaxRequestBodySize in bytes (e.g. 10 * 1024 * 1024 for 10MB).
	MaxRequestBodySize int

	// MaxMultipartFiles is the maximum total number of files allowed in a single multipart request (default: 20).
	MaxMultipartFiles int

	// UseHttpStatusCodes maps RPC error codes to standard HTTP status codes (e.g. 400, 401, 404, 429, 500)
	// for enhanced integration with cloud monitoring, load balancers, and APM tools.
	// Defaults to false (standard uniform HTTP 200 RPC protocol).
	UseHttpStatusCodes bool

	// TelemetryHooks are optional callbacks for external metrics, Sentry, or batch loggers
	TelemetryHooks []TelemetryHook

	// TrustProxy determines whether X-Forwarded-For is trusted for client IP extraction
	TrustProxy bool

	// TrustedProxies is a list of trusted upstream proxy IP addresses or CIDRs
	TrustedProxies []string

	// ReadTimeout is the maximum duration for reading the entire request (default: 30s)
	ReadTimeout time.Duration

	// WriteTimeout is the maximum duration for writing the response (default: 30s)
	WriteTimeout time.Duration

	// IdleTimeout is the maximum amount of time to wait for the next request when keep-alives are enabled (default: 60s)
	IdleTimeout time.Duration
}

// Validate ensures all mandatory configuration settings are present without ambiguous defaults
func (c *Config) Validate() error {
	trimmedPath := strings.TrimSpace(c.RpcPath)
	if trimmedPath == "" {
		return fmt.Errorf("crpc config error: RpcPath is required and cannot be empty (e.g. RpcPath: \"/rpc\")")
	}
	if !strings.HasPrefix(trimmedPath, "/") {
		return fmt.Errorf("crpc config error: RpcPath must begin with a leading slash (got: %q)", c.RpcPath)
	}

	if c.CORS == nil {
		return fmt.Errorf("crpc config error: CORS configuration is required. Please explicitly configure CORS or pass rpc.CorsConfig{AllowOrigins: []string{\"*\"}}")
	}

	if c.CORS.AllowCredentials {
		for _, origin := range c.CORS.AllowOrigins {
			if strings.TrimSpace(origin) == "*" {
				return fmt.Errorf("crpc config error: wildcard origin '*' cannot be used when AllowCredentials is true")
			}
		}
	}

	if c.DocsEnabled && strings.TrimSpace(c.DocsPath) == "" {
		c.DocsPath = "/docs"
	}

	if c.Port <= 0 {
		c.Port = 4000
	}

	if c.Host == "" {
		c.Host = "0.0.0.0"
	}

	if c.MaxRequestBodySize <= 0 {
		c.MaxRequestBodySize = 10 * 1024 * 1024 // 10 MB default cap
	}

	if c.MaxMultipartFiles <= 0 {
		c.MaxMultipartFiles = 20 // 20 files default limit (aligning with rpc-core / Busboy)
	}

	if c.ReadTimeout <= 0 {
		c.ReadTimeout = 30 * time.Second
	}
	if c.WriteTimeout <= 0 {
		c.WriteTimeout = 30 * time.Second
	}
	if c.IdleTimeout <= 0 {
		c.IdleTimeout = 60 * time.Second
	}

	return nil
}
