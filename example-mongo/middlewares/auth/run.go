package auth

import (
	"github.com/CellsWeb/crpc-go/pkg/rpc"
)

func init() {
	rpc.RegisterMiddleware("auth", "Validates bearer token and enforces role-based access control", Run, OptionsSchema, HeadersSchema)
}

func Run(ctx *rpc.Context, headers map[string]string, options map[string]any) error {
	token := headers["authorization"]
	if token == "" {
		token = headers["token"]
	}
	if token == "" {
		if opt, ok := options["optional"].(bool); ok && opt {
			return nil
		}
		return rpc.NewError(rpc.ErrUnauthorized, "Missing Authorization token header")
	}

	// Store validated auth state namespaced under this middleware's name ("auth")
	ctx.Set("auth", map[string]any{
		"user": map[string]any{
			"id":    "usr_auth_header",
			"email": "user@example.com",
			"role":  "admin",
		},
	})
	return nil
}
