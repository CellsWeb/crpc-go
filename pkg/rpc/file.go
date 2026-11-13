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
	filename string
	size     int64
	mimeType string
	data     []byte
	tempPath string
}

// NewUploadedFile creates an UploadedFile from in-memory bytes or temp path
func NewUploadedFile(rawFilename string, mimeType string, data []byte, tempPath string) *UploadedFile {
	return &UploadedFile{
		filename: SanitizeFilename(rawFilename),
		size:     int64(len(data)),
		mimeType: strings.ToLower(strings.TrimSpace(mimeType)),
		data:     data,
		tempPath: tempPath,
	}
}

// Filename returns the safe, sanitized base filename (free of directory traversal)
func (f *UploadedFile) Filename() string {
	return f.filename
}

// Size returns the file size in bytes
func (f *UploadedFile) Size() int64 {
	return f.size
}

// MimeType returns the detected content-type MIME string
func (f *UploadedFile) MimeType() string {
	return f.mimeType
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

// SaveTo writes the uploaded file to the specified destination path safely
func (f *UploadedFile) SaveTo(destPath string) error {
	cleanPath := filepath.Clean(destPath)
	dir := filepath.Dir(cleanPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	if f.data != nil {
		return os.WriteFile(cleanPath, f.data, 0644)
	}

	if f.tempPath != "" {
		src, err := os.Open(f.tempPath)
		if err != nil {
			return err
		}
		defer src.Close()

		dst, err := os.OpenFile(cleanPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}
		defer dst.Close()

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
