package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FunctionSpec represents a function specification for batch or CLI generation
type FunctionSpec struct {
	Namespace    string   `json:"namespace"`
	FunctionName string   `json:"functionName"`
	Multipart    bool     `json:"multipart,omitempty"`
	Middlewares  []string `json:"middlewares,omitempty"`
}

// ScaffoldFunction generates the co-located files for an RPC function
func ScaffoldFunction(baseDir, namespace, functionName string, isMultipart bool, middlewares []string) error {
	ns := strings.TrimSpace(namespace)
	fn := strings.TrimSpace(functionName)
	if ns == "" || fn == "" {
		return fmt.Errorf("namespace and functionName are required")
	}

	targetDir := filepath.Join(baseDir, "functions", ns, fn)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", targetDir, err)
	}

	packageName := sanitizePackageName(fn)

	// 1. Generate contracts.go
	contractsContent := generateContractsCode(packageName, ns, fn, isMultipart)
	if err := os.WriteFile(filepath.Join(targetDir, "contracts.go"), []byte(contractsContent), 0644); err != nil {
		return err
	}

	// 2. Generate middlewares.go
	middlewaresContent := generateMiddlewaresCode(packageName, middlewares)
	if err := os.WriteFile(filepath.Join(targetDir, "middlewares.go"), []byte(middlewaresContent), 0644); err != nil {
		return err
	}

	// 3. Generate run.go
	runContent := generateRunCode(packageName, isMultipart)
	if err := os.WriteFile(filepath.Join(targetDir, "run.go"), []byte(runContent), 0644); err != nil {
		return err
	}

	fmt.Printf("✓ Scaffolded function %s.%s at %s\n", ns, fn, targetDir)
	return nil
}

// ScaffoldMiddleware generates boilerplate for a reusable middleware
func ScaffoldMiddleware(baseDir, name, description string) error {
	mwName := strings.TrimSpace(name)
	if mwName == "" {
		return fmt.Errorf("middleware name is required")
	}
	if description == "" {
		description = fmt.Sprintf("Middleware %s", mwName)
	}

	targetDir := filepath.Join(baseDir, "middlewares", mwName)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed creating directory %s: %w", targetDir, err)
	}

	packageName := sanitizePackageName(mwName)

	// 1. constants.go
	constantsCode := fmt.Sprintf(`package %s

import "github.com/CellsWeb/crpc-go/pkg/schema"

type Options struct {
	// Add custom middleware options here
}

var OptionsSchema = schema.Object(map[string]*schema.SchemaRule{
	// Add configurable options schema here
})

var HeadersSchema = schema.Object(map[string]*schema.SchemaRule{
	// Add required HTTP headers schema here
})
`, packageName)
	if err := os.WriteFile(filepath.Join(targetDir, "constants.go"), []byte(constantsCode), 0644); err != nil {
		return err
	}

	// 2. run.go
	runCode := fmt.Sprintf(`package %s

import (
	"github.com/CellsWeb/crpc-go/pkg/rpc"
)

func init() {
	rpc.RegisterMiddleware("%s", "%s", Run, OptionsSchema, HeadersSchema)
}

func Run(ctx *rpc.Context, headers map[string]string, options map[string]any) error {
	// Middleware business logic: validate headers or inject context
	// Return rpc.NewError(rpc.ErrUnauthorized, "...") to fail closed immediately
	return nil
}
`, packageName, mwName, description)
	if err := os.WriteFile(filepath.Join(targetDir, "run.go"), []byte(runCode), 0644); err != nil {
		return err
	}

	fmt.Printf("✓ Scaffolded middleware %s at %s\n", mwName, targetDir)
	return nil
}

// ScaffoldBatch processes a JSON array of FunctionSpecs
func ScaffoldBatch(baseDir string, specs []FunctionSpec) error {
	for _, s := range specs {
		if err := ScaffoldFunction(baseDir, s.Namespace, s.FunctionName, s.Multipart, s.Middlewares); err != nil {
			return err
		}
	}
	return nil
}

// ScaffoldBatchFromFile reads a JSON spec file and scaffolds functions
func ScaffoldBatchFromFile(baseDir, filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed reading batch file %s: %w", filePath, err)
	}

	var specs []FunctionSpec
	if err := json.Unmarshal(data, &specs); err != nil {
		return fmt.Errorf("failed parsing batch JSON: %w", err)
	}

	return ScaffoldBatch(baseDir, specs)
}

func sanitizePackageName(name string) string {
	clean := strings.ReplaceAll(name, ".", "_")
	clean = strings.ReplaceAll(clean, "-", "_")
	return strings.ToLower(clean)
}

func generateContractsCode(pkg, ns, fn string, isMultipart bool) string {
	cType := "application/json"
	if isMultipart {
		cType = "multipart/form-data"
		return fmt.Sprintf(`package %s

import "github.com/CellsWeb/crpc-go/pkg/rpc"

const (
	Namespace    = %q
	FunctionName = %q
	ContentType  = %q
)

var Metadata = rpc.FunctionMetadata{
	Description: "Handler for %s.%s",
	Tags:        []string{%q, "upload"},
}

type Input struct {
	Name   string            `+"`"+`json:"name" doc:"User display name"`+"`"+`
	Avatar *rpc.UploadedFile `+"`"+`json:"avatar" doc:"Uploaded avatar image"`+"`"+`
}

type Output struct {
	Success   bool   `+"`"+`json:"success"`+"`"+`
	AvatarURL string `+"`"+`json:"avatarUrl"`+"`"+`
}
`, pkg, ns, fn, cType, ns, fn, ns)
	}

	return fmt.Sprintf(`package %s

import "github.com/CellsWeb/crpc-go/pkg/rpc"

const (
	Namespace    = %q
	FunctionName = %q
	ContentType  = %q
)

var Metadata = rpc.FunctionMetadata{
	Description: "Handler for %s.%s",
	Tags:        []string{%q},
}

type Input struct {
	ID string `+"`"+`json:"id" doc:"Target resource identifier"`+"`"+`
}

type Output struct {
	ID        string `+"`"+`json:"id"`+"`"+`
	Status    string `+"`"+`json:"status"`+"`"+`
	Timestamp int64  `+"`"+`json:"timestamp"`+"`"+`
}
`, pkg, ns, fn, cType, ns, fn, ns)
}

func generateMiddlewaresCode(pkg string, middlewares []string) string {
	var mwLines strings.Builder
	for _, m := range middlewares {
		mwLines.WriteString(fmt.Sprintf("\n\t\trpc.M(%q, map[string]any{}),", m))
	}
	if len(middlewares) > 0 {
		mwLines.WriteString("\n\t")
	}

	return fmt.Sprintf(`package %s

import "github.com/CellsWeb/crpc-go/pkg/rpc"

var Middlewares = rpc.DefineMiddlewares(%s)
`, pkg, mwLines.String())
}

func generateRunCode(pkg string, isMultipart bool) string {
	returnStatement := `return &Output{
		ID:        input.ID,
		Status:    "active",
		Timestamp: time.Now().Unix(),
	}, nil`

	if isMultipart {
		returnStatement = `avatarURL := ""
	if input.Avatar != nil {
		avatarURL = "/uploads/" + input.Avatar.Filename()
	}
	return &Output{
		Success:   true,
		AvatarURL: avatarURL,
	}, nil`
	}

	return fmt.Sprintf(`package %s

import (
	"time"

	"github.com/CellsWeb/crpc-go/pkg/rpc"
)

func init() {
	rpc.Register(rpc.FunctionConfig[Input, Output]{
		Namespace:    Namespace,
		FunctionName: FunctionName,
		ContentType:  ContentType,
		Metadata:     Metadata,
		Middlewares:  Middlewares,
		Handler:      Run,
	})
}

func Run(ctx *rpc.Context, input Input) (*Output, error) {
	_ = time.Now()
	%s
}
`, pkg, returnStatement)
}
