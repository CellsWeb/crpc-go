package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
)

// Source builds have no published module version. Use a real released fallback;
// an explicit Go workspace can select an unreleased SDK checkout for development.
const releasedSDKVersion = "v1.0.0"

func scaffoldSDKVersion(info *debug.BuildInfo) string {
	if info == nil || info.Main.Path != "github.com/CellsWeb/crpc-go" || info.Main.Version == "" || info.Main.Version == "(devel)" || info.Main.Replace != nil {
		return releasedSDKVersion
	}
	// Go can derive a version from a local Git commit that was never published.
	// Only a versioned module installation may supply the SDK dependency version.
	for _, setting := range info.Settings {
		if setting.Key == "vcs" || strings.HasPrefix(setting.Key, "vcs.") {
			return releasedSDKVersion
		}
	}
	return info.Main.Version
}

// InitProject scaffolds a complete, production-ready CellsWeb RPC Go project
func InitProject(targetDir string) error {
	projectName := filepath.Base(targetDir)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	// 1. go.mod
	buildInfo, _ := debug.ReadBuildInfo()
	sdkVersion := scaffoldSDKVersion(buildInfo)
	goMod := fmt.Sprintf(`module %s

go 1.27.4

require github.com/CellsWeb/crpc-go %s
`, projectName, sdkVersion)
	_ = os.WriteFile(filepath.Join(targetDir, "go.mod"), []byte(goMod), 0644)

	// 2. .env.example & .env
	envContent := `# Neither the generated binary nor crpc-go dev automatically loads .env.
# Export values before starting; for this trusted shell-compatible file:
# set -a; . ./.env; set +a
PORT=4000
HOST=0.0.0.0
RPC_PATH=/rpc
# Unset/blank DOCS_ENABLED uses NODE_ENV != production. Invalid booleans stop startup.
DOCS_ENABLED=true
# Use an absolute documentation endpoint, e.g. /internal/docs. Blank defaults to /docs.
DOCS_PATH=/docs
NODE_ENV=development
`
	_ = os.WriteFile(filepath.Join(targetDir, ".env.example"), []byte(envContent), 0644)
	_ = os.WriteFile(filepath.Join(targetDir, ".env"), []byte(envContent), 0644)

	// 3. Makefile
	makefile := `dev:
	crpc-go dev

build:
	go build -o dist/server cmd/server/main.go

test:
	go test -v ./...
`
	_ = os.WriteFile(filepath.Join(targetDir, "Makefile"), []byte(makefile), 0644)

	// 4. storage and imagehandler utilities (bonus headstart)
	_ = os.MkdirAll(filepath.Join(targetDir, "utils", "storage"), 0755)
	_ = os.MkdirAll(filepath.Join(targetDir, "utils", "imagehandler"), 0755)

	storageGo := `package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type StorageMode string

const (
	ModeLocal StorageMode = "local"
	ModeS3    StorageMode = "s3"
)

type UploadResult struct {
	URL     string `+"`"+`json:"url"`+"`"+`
	FullURL string `+"`"+`json:"fullUrl"`+"`"+`
	Path    string `+"`"+`json:"path"`+"`"+`
	Name    string `+"`"+`json:"name"`+"`"+`
}

type Config struct {
	Mode      StorageMode
	UploadDir string
	ServerURL string
}

type Engine struct {
	config Config
}

func New(cfg Config) *Engine {
	if cfg.Mode == "" {
		cfg.Mode = ModeLocal
	}
	if cfg.UploadDir == "" {
		cfg.UploadDir = "public/uploads"
	}
	if cfg.ServerURL == "" {
		cfg.ServerURL = "http://localhost:4000"
	}
	cfg.ServerURL = strings.TrimRight(cfg.ServerURL, "/")
	return &Engine{config: cfg}
}

var DefaultEngine = New(Config{
	UploadDir: os.Getenv("UPLOAD_DIR"),
	ServerURL: os.Getenv("SERVER_URL"),
})

func (e *Engine) ToRelativeImageUrl(pathOrUrl string) string {
	trimmed := strings.TrimSpace(pathOrUrl)
	if trimmed == "" || trimmed == "-" {
		return ""
	}
	if idx := strings.Index(trimmed, "/uploads/"); idx != -1 {
		rel := trimmed[idx+len("/uploads/"):]
		return "uploads/" + strings.TrimLeft(rel, "/")
	}
	if strings.HasPrefix(trimmed, "uploads/") {
		return trimmed
	}
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return trimmed
	}
	return trimmed
}

func (e *Engine) ResolveImageUrl(pathOrUrl string) string {
	trimmed := strings.TrimSpace(pathOrUrl)
	if trimmed == "" || trimmed == "-" {
		return ""
	}
	serverURL := e.config.ServerURL
	if serverURL == "" {
		serverURL = "http://localhost:4000"
	}
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return trimmed
	}
	clean := strings.TrimLeft(trimmed, "/")
	if !strings.HasPrefix(clean, "uploads/") {
		clean = "uploads/" + clean
	}
	return fmt.Sprintf("%s/%s", serverURL, clean)
}

func (e *Engine) Upload(data []byte, originalName, subDir string) (*UploadResult, error) {
	uploadRoot, err := filepath.Abs(e.config.UploadDir)
	if err != nil {
		return nil, fmt.Errorf("invalid upload root directory: %w", err)
	}

	cleanSub := e.cleanStoragePath(subDir)
	cleanSub = strings.ReplaceAll(cleanSub, "..", "")
	cleanSub = strings.TrimLeft(cleanSub, "/")

	targetDir := filepath.Join(uploadRoot, cleanSub)
	targetDirClean := filepath.Clean(targetDir)

	if !strings.HasPrefix(targetDirClean, uploadRoot) && targetDirClean != uploadRoot {
		return nil, fmt.Errorf("security violation: path traversal detected in upload directory")
	}

	if err := os.MkdirAll(targetDirClean, 0755); err != nil {
		return nil, fmt.Errorf("failed creating upload directory: %w", err)
	}

	safeName := e.sanitizeFilename(originalName)
	savePath := filepath.Join(targetDirClean, safeName)

	if err := os.WriteFile(savePath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed writing uploaded file: %w", err)
	}

	relPath := safeName
	if cleanSub != "" {
		relPath = filepath.ToSlash(filepath.Join(cleanSub, safeName))
	}

	relURL := "uploads/" + relPath

	return &UploadResult{
		URL:     relURL,
		FullURL: e.ResolveImageUrl(relURL),
		Path:    relPath,
		Name:    safeName,
	}, nil
}

func (e *Engine) Delete(relativePath string) bool {
	uploadRoot, err := filepath.Abs(e.config.UploadDir)
	if err != nil {
		return false
	}
	clean := e.cleanStoragePath(relativePath)
	fullPath := filepath.Clean(filepath.Join(uploadRoot, clean))
	if !strings.HasPrefix(fullPath, uploadRoot) && fullPath != uploadRoot {
		return false
	}
	if _, err := os.Stat(fullPath); err == nil {
		return os.Remove(fullPath) == nil
	}
	return false
}

func (e *Engine) Serve(relativePath string) ([]byte, string, error) {
	uploadRoot, err := filepath.Abs(e.config.UploadDir)
	if err != nil {
		return nil, "", err
	}
	clean := e.cleanStoragePath(relativePath)
	fullPath := filepath.Clean(filepath.Join(uploadRoot, clean))
	if !strings.HasPrefix(fullPath, uploadRoot) && fullPath != uploadRoot {
		return nil, "", fmt.Errorf("access denied")
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, "", err
	}
	ext := strings.ToLower(filepath.Ext(fullPath))
	mimeType := "application/octet-stream"
	switch ext {
	case ".jpg", ".jpeg":
		mimeType = "image/jpeg"
	case ".png":
		mimeType = "image/png"
	case ".webp":
		mimeType = "image/webp"
	case ".gif":
		mimeType = "image/gif"
	case ".svg":
		mimeType = "image/svg+xml"
	}
	return data, mimeType, nil
}

func (e *Engine) sanitizeFilename(original string) string {
	ext := filepath.Ext(original)
	base := strings.TrimSuffix(filepath.Base(original), ext)
	var clean strings.Builder
	for _, r := range base {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			clean.WriteRune(r)
		} else {
			clean.WriteRune('_')
		}
	}
	baseStr := clean.String()
	if baseStr == "" {
		baseStr = "file"
	}
	randBytes := make([]byte, 4)
	_, _ = rand.Read(randBytes)
	randHex := hex.EncodeToString(randBytes)
	return fmt.Sprintf("%s_%d_%s%s", baseStr, time.Now().UnixNano(), randHex, ext)
}

func (e *Engine) cleanStoragePath(inputPath string) string {
	p := inputPath
	if unescaped, err := url.QueryUnescape(p); err == nil {
		p = unescaped
	}
	if strings.HasPrefix(p, e.config.ServerURL) {
		p = strings.TrimPrefix(p, e.config.ServerURL)
	}
	if idx := strings.IndexAny(p, "?#"); idx != -1 {
		p = p[:idx]
	}
	p = filepath.ToSlash(p)
	for {
		trimmed := false
		if strings.HasPrefix(p, "/uploads/") {
			p = strings.TrimPrefix(p, "/uploads/")
			trimmed = true
		} else if strings.HasPrefix(p, "uploads/") {
			p = strings.TrimPrefix(p, "uploads/")
			trimmed = true
		} else if strings.HasPrefix(p, "/") {
			p = strings.TrimPrefix(p, "/")
			trimmed = true
		}
		if !trimmed {
			break
		}
	}
	return filepath.Clean(p)
}
`
	_ = os.WriteFile(filepath.Join(targetDir, "utils", "storage", "storage.go"), []byte(storageGo), 0644)

	imageHandlerGo := `package imagehandler

import (
	"github.com/CellsWeb/crpc-go/pkg/rpc"
	"` + projectName + `/utils/storage"
)

func ProcessImageField(file *rpc.UploadedFile, dir string, existingURL string, engine ...*storage.Engine) (string, error) {
	eng := storage.DefaultEngine
	if len(engine) > 0 && engine[0] != nil {
		eng = engine[0]
	}
	if dir == "" {
		dir = "general"
	}

	if file != nil {
		data, err := file.Bytes()
		if err != nil {
			return "", err
		}
		res, err := eng.Upload(data, file.Filename(), dir)
		if err != nil {
			return "", err
		}
		if existingURL != "" {
			_ = eng.Delete(existingURL)
		}
		return eng.ToRelativeImageUrl(res.URL), nil
	}

	if existingURL != "" {
		return eng.ToRelativeImageUrl(existingURL), nil
	}
	return "", nil
}

func ProcessMultipleImageFields(files []*rpc.UploadedFile, dir string, removeList []string, existingList []string, engine ...*storage.Engine) ([]string, error) {
	eng := storage.DefaultEngine
	if len(engine) > 0 && engine[0] != nil {
		eng = engine[0]
	}
	if dir == "" {
		dir = "gallery"
	}

	for _, toRemove := range removeList {
		if toRemove != "" {
			_ = eng.Delete(toRemove)
		}
	}

	var finalURLs []string
	removeSet := make(map[string]bool)
	for _, r := range removeList {
		removeSet[eng.ToRelativeImageUrl(r)] = true
	}

	for _, ex := range existingList {
		rel := eng.ToRelativeImageUrl(ex)
		if !removeSet[rel] && rel != "" {
			finalURLs = append(finalURLs, rel)
		}
	}

	for _, file := range files {
		if file != nil {
			data, err := file.Bytes()
			if err != nil {
				return nil, err
			}
			res, err := eng.Upload(data, file.Filename(), dir)
			if err != nil {
				return nil, err
			}
			finalURLs = append(finalURLs, eng.ToRelativeImageUrl(res.URL))
		}
	}
	return finalURLs, nil
}
`
	_ = os.WriteFile(filepath.Join(targetDir, "utils", "imagehandler", "imagehandler.go"), []byte(imageHandlerGo), 0644)

	// 5. Scaffold sample middleware: auth
	_ = ScaffoldMiddleware(targetDir, "auth", "Validates Bearer token and injects auth context")

	// 6. Scaffold sample function: user.getProfile
	_ = ScaffoldFunction(targetDir, "user", "getProfile", false, []string{"auth"})

	// 7. Scaffold sample function: user.updateProfile (multipart)
	_ = ScaffoldFunction(targetDir, "user", "updateProfile", true, []string{"auth"})

	// 8. cmd/server/main.go
	_ = os.MkdirAll(filepath.Join(targetDir, "cmd", "server"), 0755)
	mainGo := fmt.Sprintf(`package main

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/CellsWeb/crpc-go/pkg/rpc"

	// Register functions & middlewares via init()
	_ "%s/functions/user/getProfile"
	_ "%s/functions/user/updateProfile"
	_ "%s/middlewares/auth"
)

func main() {
	// Environment values must be exported by the shell or deployment platform.
	// .env is not automatically loaded by this executable or crpc-go dev.
	docsEnabled := os.Getenv("NODE_ENV") != "production"
	if raw := strings.TrimSpace(os.Getenv("DOCS_ENABLED")); raw != "" {
		var err error
		docsEnabled, err = strconv.ParseBool(strings.ToLower(raw))
		if err != nil {
			log.Fatal("Invalid DOCS_ENABLED: expected true/false, 1/0, or t/f")
		}
	}
	docsPath := strings.TrimSpace(os.Getenv("DOCS_PATH"))
	if docsPath == "" {
		docsPath = "/docs"
	}

	app, err := rpc.New(rpc.Config{
		RpcPath:     "/rpc",
		Port:        4000,
		Host:        "0.0.0.0",
		DocsEnabled: docsEnabled,
		DocsPath:    docsPath,
		CORS: &rpc.CorsConfig{
			AllowOrigins: []string{"*"},
			AllowMethods: []string{"POST", "GET", "OPTIONS"},
			AllowHeaders: []string{"Content-Type", "Authorization"},
		},
	})
	if err != nil {
		log.Fatalf("Config error: %%v", err)
	}

	app.UseGlobalRegistry()

	// Custom HTTP route for health checks
	app.GET("/health", func(c *rpc.HTTPContext) error {
		return c.SendString("OK")
	})

	log.Printf("⚡ CellsWeb RPC Go Engine running at http://0.0.0.0:4000/rpc")
	if docsEnabled {
		log.Printf("📖 Interactive Docs available at http://0.0.0.0:4000%%s", docsPath)
	}

	if err := app.ListenAndServe(); err != nil {
		log.Fatalf("Server stopped: %%v", err)
	}
}
`, projectName, projectName, projectName)
	_ = os.WriteFile(filepath.Join(targetDir, "cmd", "server", "main.go"), []byte(mainGo), 0644)

	fmt.Printf("\n🎉 Project %s created successfully!\n", projectName)
	fmt.Printf("SDK dependency: github.com/CellsWeb/crpc-go@%s\n", sdkVersion)
	fmt.Printf("Next steps:\n  cd %s\n  go mod tidy\n  crpc-go dev\n", projectName)
	return nil
}
