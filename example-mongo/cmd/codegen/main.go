package main

import (
	"fmt"
	"log"
	"os"

	"github.com/CellsWeb/crpc-go/pkg/nodebridge"
	"github.com/CellsWeb/crpc-go/pkg/rpc"

	_ "example-mongo/functions/public/hello"
	_ "example-mongo/functions/system/ping"
	_ "example-mongo/functions/user/create"
	_ "example-mongo/functions/user/get"
	_ "example-mongo/functions/user/getProfile"
	_ "example-mongo/functions/user/uploadAvatar"
	_ "example-mongo/middlewares/auth"
	_ "example-mongo/middlewares/rateLimit"
)

func main() {
	app, err := rpc.New(rpc.Config{
		RpcPath: "/rpc",
		CORS: &rpc.CorsConfig{
			AllowOrigins: []string{"*"},
		},
	})
	if err != nil {
		log.Fatalf("RPC init error: %v", err)
	}
	app.UseGlobalRegistry()

	dts := nodebridge.GenerateTypeScriptDeclarations(app.Functions())

	outPath := "crpc.d.ts"
	if len(os.Args) > 1 {
		outPath = os.Args[1]
	}

	if err := os.WriteFile(outPath, []byte(dts), 0644); err != nil {
		log.Fatalf("Failed to write %s: %v", outPath, err)
	}

	fmt.Printf("✓ TypeScript definitions generated successfully at %s\n", outPath)
}
