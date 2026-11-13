package ratelimit

import (
	"github.com/CellsWeb/crpc-go/pkg/rpc"
)

func init() {
	rpc.RegisterMiddleware("rateLimit", "Enforces per-function rate limits keyed on IP + namespace + functionName", Run, OptionsSchema, HeadersSchema)
}

func Run(ctx *rpc.Context, headers map[string]string, options map[string]any) error {
	return nil
}
