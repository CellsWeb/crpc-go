package auth

import (
	"github.com/CellsWeb/crpc-go/pkg/rpc"
)

func init() {
	rpc.RegisterMiddleware("auth", "Validates bearer token and enforces role-based access control", Run, OptionsSchema, HeadersSchema)
}

func Run(ctx *rpc.Context, headers map[string]string, options map[string]any) error {
	// Middleware business logic: validate headers or inject context
	return nil
}
