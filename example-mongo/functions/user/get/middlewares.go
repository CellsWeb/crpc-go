package get

import "github.com/CellsWeb/crpc-go/pkg/rpc"

var Middlewares = rpc.DefineMiddlewares(
	rpc.M("auth", map[string]any{"roles": []string{"admin", "user"}}),
	rpc.M("rateLimit", map[string]any{"windowSeconds": 60, "maxRequests": 100}),
)
