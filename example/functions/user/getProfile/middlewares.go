package getprofile

import "github.com/CellsWeb/crpc-go/pkg/rpc"

var Middlewares = rpc.DefineMiddlewares(
		rpc.M("auth", map[string]any{}),
	)
