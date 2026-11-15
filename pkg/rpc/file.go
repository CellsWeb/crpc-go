package rpc

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// UploadedFile represents a validated binary file uploaded via multipart/form-data
type UploadedFile struct {
	originalFilename string
	filename         string
	size             int64
	declaredMimeType string
	detectedMimeType string
	data             []byte
	tempPath         string
}

// NewUploadedFile creates an UploadedFile from in-memory bytes or temp path
func NewUploadedFile(rawFilename string, mimeType string, data []byte, tempPath string) *UploadedFile {
	cleanMime := strings.ToLower(strings.TrimSpace(mimeType))
	return &UploadedFile{
		originalFilename: rawFilename,
		filename:         SanitizeFilename(rawFilename),
		size:             int64(len(data)),
		declaredMimeType: cleanMime,
		detectedMimeType: cleanMime,
		data:             data,
		tempPath:         tempPath,
	}
}

// NewUploadedFileDetailed creates an UploadedFile with explicit declared vs detected MIME types
func NewUploadedFileDetailed(rawFilename string, declaredMime string, detectedMime string, size int64, data []byte, tempPath string) *UploadedFile {
	return &UploadedFile{
		originalFilename: rawFilename,
		filename:         SanitizeFilename(rawFilename),
		size:             size,
		declaredMimeType: strings.ToLower(strings.TrimSpace(declaredMime)),
		detectedMimeType: strings.ToLower(strings.TrimSpace(detectedMime)),
		data:             data,
		tempPath:         tempPath,
	}
}

// OriginalFilename returns the raw unescaped filename provided by the client
func (f *UploadedFile) OriginalFilename() string {
	return f.originalFilename
}

// DeclaredMimeType returns the content-type header declared by the client
func (f *UploadedFile) DeclaredMimeType() string {
	return f.declaredMimeType
}

// DetectedMimeType returns the content-type detected via magic bytes sniffing
func (f *UploadedFile) DetectedMimeType() string {
	return f.detectedMimeType
}

// Filename returns the safe, sanitized base filename (free of directory traversal)
func (f *UploadedFile) Filename() string {
	return f.filename
}

// Size returns the file size in bytes
func (f *UploadedFile) Size() int64 {
	return f.size
}

// MimeType returns the detected content-type MIME string (falling back to declared)
func (f *UploadedFile) MimeType() string {
	if f.detectedMimeType != "" {
		return f.detectedMimeType
	}
	return f.declaredMimeType
}

// Bytes returns the raw bytes of the file
func (f *UploadedFile) Bytes() ([]byte, error) {
	if f.data != nil {
		return f.data, nil
	}
	if f.tempPath != "" {
		return os.ReadFile(f.tempPath)
	}
	return nil, fmt.Errorf("uploaded file has no data or backing storage")
}

// Reader returns an io.ReadCloser for streaming the file contents
func (f *UploadedFile) Reader() (io.ReadCloser, error) {
	if f.data != nil {
		return io.NopCloser(bytes.NewReader(f.data)), nil
	}
	if f.tempPath != "" {
		return os.Open(f.tempPath)
	}
	return nil, fmt.Errorf("uploaded file has no readable stream")
}

// SaveTo writes the uploaded file to the specified destination path safely with atomic creation and 0600 permissions
func (f *UploadedFile) SaveTo(destPath string) error {
	cleanPath := filepath.Clean(destPath)
	dir := filepath.Dir(cleanPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Verify symlink containment on target directory
	if realDir, err := filepath.EvalSymlinks(dir); err == nil {
		baseName := filepath.Base(cleanPath)
		cleanPath = filepath.Join(realDir, baseName)
	}

	dst, err := os.OpenFile(cleanPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("failed to create destination file (atomic check failed): %w", err)
	}
	defer dst.Close()

	if f.data != nil {
		_, err = dst.Write(f.data)
		return err
	}

	if f.tempPath != "" {
		src, err := os.Open(f.tempPath)
		if err != nil {
			return err
		}
		defer src.Close()

		_, err = io.Copy(dst, src)
		return err
	}

	return fmt.Errorf("no content available to save")
}

// Cleanup removes any temporary on-disk files associated with this upload
func (f *UploadedFile) Cleanup() {
	if f.tempPath != "" {
		_ = os.Remove(f.tempPath)
	}
}

// SanitizeFilename strips directory traversal sequences (../, ..\), null bytes, and path delimiters
func SanitizeFilename(name string) string {
	clean := strings.ReplaceAll(name, "\x00", "")
	clean = filepath.Base(clean)
	clean = strings.TrimSpace(clean)
	if clean == "" || clean == "." || clean == "/" || clean == "\\" {
		return fmt.Sprintf("upload_%d.bin", os.Getpid())
	}
	return clean
}
