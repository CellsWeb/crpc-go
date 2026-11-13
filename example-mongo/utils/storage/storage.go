package storage

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
	URL     string `json:"url"`
	FullURL string `json:"fullUrl"`
	Path    string `json:"path"`
	Name    string `json:"name"`
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
