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

// UploadResult contains the normalized paths and URLs of an uploaded file
type UploadResult struct {
	URL     string `json:"url"`     // Relative path (e.g. "uploads/products/image.png")
	FullURL string `json:"fullUrl"` // Absolute URL (e.g. "http://localhost:4000/uploads/products/image.png")
	Path    string `json:"path"`    // Relative storage path for deletion
	Name    string `json:"name"`    // Final sanitized filename
}

// Config holds runtime storage settings
type Config struct {
	Mode      StorageMode
	UploadDir string // Base upload folder (e.g. "public/uploads")
	ServerURL string // Base server URL (e.g. "http://localhost:4000")
}

// Engine implements domain-agnostic file storage
type Engine struct {
	config Config
}

// New creates a new Storage Engine
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

// DefaultEngine instance for standard project usage
var DefaultEngine = New(Config{
	UploadDir: os.Getenv("UPLOAD_DIR"),
	ServerURL: os.Getenv("SERVER_URL"),
})

// ToRelativeImageUrl normalizes any URL/path into domain-agnostic "uploads/..."
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

	// External S3 or CDN URLs remain unchanged
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return trimmed
	}

	return trimmed
}

// ResolveImageUrl resolves any relative path into a client-accessible full URL
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

// Upload writes a binary buffer into target storage directory safely
func (e *Engine) Upload(data []byte, originalName, subDir string) (*UploadResult, error) {
	uploadRoot, err := filepath.Abs(e.config.UploadDir)
	if err != nil {
		return nil, fmt.Errorf("invalid upload root directory: %w", err)
	}

	// Prevent directory traversal in subDir
	cleanSub := e.cleanStoragePath(subDir)
	cleanSub = strings.ReplaceAll(cleanSub, "..", "")
	cleanSub = strings.TrimLeft(cleanSub, "/")

	targetDir := filepath.Join(uploadRoot, cleanSub)
	targetDirClean := filepath.Clean(targetDir)

	// Enforce storage sandbox boundary
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

// Delete removes a file from local storage safely
func (e *Engine) Delete(relativePath string) bool {
	uploadRoot, err := filepath.Abs(e.config.UploadDir)
	if err != nil {
		return false
	}

	clean := e.cleanStoragePath(relativePath)
	fullPath := filepath.Clean(filepath.Join(uploadRoot, clean))

	// Enforce sandbox boundary
	if !strings.HasPrefix(fullPath, uploadRoot) && fullPath != uploadRoot {
		return false
	}

	if _, err := os.Stat(fullPath); err == nil {
		return os.Remove(fullPath) == nil
	}
	return false
}

// Serve reads a file from storage and determines its MIME type
func (e *Engine) Serve(relativePath string) ([]byte, string, error) {
	uploadRoot, err := filepath.Abs(e.config.UploadDir)
	if err != nil {
		return nil, "", err
	}

	clean := e.cleanStoragePath(relativePath)
	fullPath := filepath.Clean(filepath.Join(uploadRoot, clean))

	// Enforce sandbox boundary
	if !strings.HasPrefix(fullPath, uploadRoot) && fullPath != uploadRoot {
		return nil, "", fmt.Errorf("access denied")
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, "", err
	}

	mimeType := detectMimeType(fullPath, data)
	return data, mimeType, nil
}

func (e *Engine) sanitizeFilename(original string) string {
	ext := filepath.Ext(original)
	base := strings.TrimSuffix(filepath.Base(original), ext)

	// Clean base name
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

	// Remove server URL if present
	if strings.HasPrefix(p, e.config.ServerURL) {
		p = strings.TrimPrefix(p, e.config.ServerURL)
	}

	// Strip query parameters and hash fragments
	if idx := strings.IndexAny(p, "?#"); idx != -1 {
		p = p[:idx]
	}

	p = filepath.ToSlash(p)

	// Strip leading /uploads/ or /
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

func detectMimeType(filename string, data []byte) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".pdf":
		return "application/pdf"
	case ".json":
		return "application/json"
	default:
		return "application/octet-stream"
	}
}
