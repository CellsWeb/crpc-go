package getprofile

import (
	"time"

	"github.com/CellsWeb/crpc-go/pkg/rpc"
)

func init() {
	rpc.Register(rpc.FunctionConfig[Input, Output]{
		Namespace:    "user",
		FunctionName: "getProfile",
		ContentType:  "application/json",
		Middlewares:  Middlewares,
		Handler:      Run,
	})
}

func Run(ctx *rpc.Context, input Input) (*Output, error) {
	return &Output{
		ID:        input.ID,
		Status:    "active",
		Timestamp: time.Now().Unix(),
	}, nil
}
