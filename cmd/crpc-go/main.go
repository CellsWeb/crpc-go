package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/CellsWeb/crpc-go/cmd/crpc-go/commands"
)

const version = "1.0.2"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "init":
		if len(os.Args) < 3 {
			fmt.Println("Usage: crpc-go init <project-name>")
			os.Exit(1)
		}
		targetDir := os.Args[2]
		if err := commands.InitProject(targetDir); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "create:function":
		fs := flag.NewFlagSet("create:function", flag.ExitOnError)
		ns := fs.String("ns", "", "Namespace")
		name := fs.String("name", "", "Function Name")
		multipart := fs.Bool("multipart", false, "Use multipart/form-data for file uploads")
		batch := fs.String("batch", "", "Batch JSON array of function specs")
		batchFile := fs.String("file", "", "Batch JSON spec file")
		_ = fs.Parse(os.Args[2:])

		baseDir, _ := os.Getwd()

		if *batch != "" {
			var specs []commands.FunctionSpec
			if err := json.Unmarshal([]byte(*batch), &specs); err != nil {
				fmt.Printf("Error parsing batch JSON: %v\n", err)
				os.Exit(1)
			}
			if err := commands.ScaffoldBatch(baseDir, specs); err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		if *batchFile != "" {
			if err := commands.ScaffoldBatchFromFile(baseDir, *batchFile); err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		if *ns == "" || *name == "" {
			fmt.Println("Usage: crpc-go create:function --ns <namespace> --name <functionName> [--multipart]")
			os.Exit(1)
		}

		if err := commands.ScaffoldFunction(baseDir, *ns, *name, *multipart, nil); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "create:middleware":
		fs := flag.NewFlagSet("create:middleware", flag.ExitOnError)
		name := fs.String("name", "", "Middleware Name")
		desc := fs.String("desc", "", "Description")
		_ = fs.Parse(os.Args[2:])

		if *name == "" {
			fmt.Println("Usage: crpc-go create:middleware --name <name> [--desc <description>]")
			os.Exit(1)
		}

		baseDir, _ := os.Getwd()
		if err := commands.ScaffoldMiddleware(baseDir, *name, *desc); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "dev":
		baseDir, _ := os.Getwd()
		if err := commands.DevServer(baseDir); err != nil {
			fmt.Printf("Dev server error: %v\n", err)
			os.Exit(1)
		}

	case "build":
		baseDir, _ := os.Getwd()
		outDir := filepath.Join(baseDir, "dist")
		_ = os.MkdirAll(outDir, 0755)
		outBinary := filepath.Join(outDir, "server")

		fmt.Println("📦 Building optimized binary...")
		cmd := exec.Command("go", "build", "-ldflags=-s -w", "-o", outBinary, "cmd/server/main.go")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("Build failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ Binary compiled successfully to %s\n", outBinary)

	case "version", "--version", "-v":
		fmt.Printf("crpc-go v%s\n", version)

	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`⚡ CellsWeb RPC Go CLI (crpc-go)

Usage:
  crpc-go <command> [options]

Commands:
  init <project-name>       Scaffold a production-ready CellsWeb RPC Go project
  create:function           Scaffold a new RPC function (--ns, --name, --multipart)
  create:middleware         Scaffold a new reusable middleware (--name, --desc)
  dev                       Start server in development mode with auto-reload
  build                     Compile production-ready optimized standalone binary
  version                   Print version information`)
}
