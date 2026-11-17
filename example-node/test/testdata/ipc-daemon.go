// Test-only daemon: exercise the unchanged production Unix IPC bridge without MongoDB.
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/CellsWeb/crpc-go/pkg/nodebridge"
	"github.com/CellsWeb/crpc-go/pkg/rpc"
	"github.com/CellsWeb/crpc-go/pkg/schema"
)

func main() {
	app, err := rpc.New(rpc.Config{RpcPath: "/rpc", CORS: &rpc.CorsConfig{AllowOrigins: []string{"*"}}})
	if err != nil {
		log.Fatal(err)
	}
	app.RegisterFunction(&rpc.FunctionDefinition{Namespace: "system", FunctionName: "ping", Handler: func(_ *rpc.Context, data any) (any, error) {
		if object, ok := data.(map[string]any); ok && object["message"] == "hold" {
			fmt.Println("HOLD")
			time.Sleep(10 * time.Second)
		}
		return data, nil
	}})
	app.RegisterFunction(&rpc.FunctionDefinition{Namespace: "public", FunctionName: "hello",
		InputSchema: schema.Object(map[string]*schema.SchemaRule{"name": schema.String()}),
		Handler: func(_ *rpc.Context, data any) (any, error) {
			return map[string]any{"greeting": "Hello, " + data.(map[string]any)["name"].(string) + "!"}, nil
		},
	})
	app.RegisterMiddleware("auth", "Test authorization", func(_ *rpc.Context, headers map[string]string, _ map[string]any) error {
		if headers["authorization"] != "Bearer valid" {
			return rpc.NewError(rpc.ErrUnauthorized, "Denied")
		}
		return nil
	}, nil, nil)
	app.RegisterFunction(&rpc.FunctionDefinition{Namespace: "user", FunctionName: "getProfile", Middlewares: rpc.DefineMiddlewares(rpc.M("auth", nil)), Handler: func(_ *rpc.Context, data any) (any, error) { return data, nil }})
	if err := nodebridge.ServeUnixSocket(app, os.Args[1]); err != nil {
		log.Fatal(err)
	}
}
