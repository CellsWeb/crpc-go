package hello

import (
	"fmt"

	"github.com/CellsWeb/crpc-go/pkg/rpc"
)

func init() {
	rpc.Register(rpc.FunctionConfig[Input, Output]{
		Namespace:    "public",
		FunctionName: "hello",
		ContentType:  "application/json",
		Middlewares:  Middlewares,
		Handler:      Run,
	})
}

func Run(ctx *rpc.Context, input Input) (*Output, error) {
	return &Output{
		Greeting: fmt.Sprintf("Hello, %s!", input.Name),
	}, nil
}
