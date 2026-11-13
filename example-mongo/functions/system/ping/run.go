package ping

import (
	"time"

	"github.com/CellsWeb/crpc-go/pkg/rpc"
)

var Middlewares = rpc.DefineMiddlewares()

func init() {
	rpc.Register(rpc.FunctionConfig[Input, Output]{
		Namespace:    "system",
		FunctionName: "ping",
		ContentType:  "application/json",
		Middlewares:  Middlewares,
		Handler:      Run,
	})
}

func Run(ctx *rpc.Context, input Input) (*Output, error) {
	msg := input.Message
	if msg == "" {
		msg = "pong"
	}
	return &Output{
		Reply:     msg,
		Timestamp: time.Now().UnixNano(),
	}, nil
}
