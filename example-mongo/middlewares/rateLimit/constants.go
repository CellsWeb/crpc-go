package ratelimit

import "github.com/CellsWeb/crpc-go/pkg/schema"

type Options struct {
	WindowSeconds int `json:"windowSeconds"`
	MaxRequests   int `json:"maxRequests"`
}

var OptionsSchema = schema.Object(map[string]*schema.SchemaRule{
	"windowSeconds": schema.Integer().Describe("Time window duration in seconds").WithDefault(60),
	"maxRequests":   schema.Integer().Describe("Maximum requests allowed within window").WithDefault(100),
})

var HeadersSchema = schema.Object(map[string]*schema.SchemaRule{
	"x-client-id": schema.Optional(schema.String()).Describe("Client device ID or IP for rate limit quota"),
})
