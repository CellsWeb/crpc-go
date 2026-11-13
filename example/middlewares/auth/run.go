package auth

import (
	"github.com/CellsWeb/crpc-go/pkg/rpc"
)

func init() {
	rpc.RegisterMiddleware("auth", "Validates Bearer token and injects auth context", Run, nil, nil)
}

func Run(ctx *rpc.Context, headers map[string]string, options map[string]any) error {
	// Middleware business logic: validate headers or inject context
	// Return rpc.NewError(rpc.ErrUnauthorized, "...") to fail closed immediately
	return nil
}
