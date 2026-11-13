package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/CellsWeb/crpc-go/pkg/rpc"
)

type StorageResult struct {
	URL      string
	Path     string
	Size     int64
	MimeType string
	Hash     string
}

// Save writes an uploaded file into destination directory and returns storage metadata
func Save(file *rpc.UploadedFile, subDir string) (*StorageResult, error) {
	bytes, err := file.Bytes()
	if err != nil {
		return nil, err
	}

	hash := sha256.Sum256(bytes)
	hashStr := hex.EncodeToString(hash[:])

	safeName := fmt.Sprintf("%d_%s", time.Now().UnixNano(), file.Filename())
	relPath := filepath.Join("uploads", subDir, safeName)
	fullPath := filepath.Join("public", relPath)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return nil, err
	}

	if err := os.WriteFile(fullPath, bytes, 0644); err != nil {
		return nil, err
	}

	return &StorageResult{
		URL:      "/" + relPath,
		Path:     fullPath,
		Size:     file.Size(),
		MimeType: file.MimeType(),
		Hash:     hashStr,
	}, nil
}
