package rpc

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/valyala/fasthttp"
)

// ParsedMultipartData holds the extracted routing parameters, fields, and file references
type ParsedMultipartData struct {
	Namespace    string
	FunctionName string
	Data         map[string]any
	Files        []*UploadedFile
}

// ParseMultipartPhase1 parses only the routing keys (namespace, functionName) from multipart stream
func ParseMultipartPhase1(reqCtx *fasthttp.RequestCtx) (namespace, functionName string, err error) {
	form, err := reqCtx.MultipartForm()
	if err != nil {
		return "", "", NewError(ErrInvalidBody, fmt.Sprintf("Malformed multipart form: %v", err))
	}

	if vals, ok := form.Value["namespace"]; ok && len(vals) > 0 {
		namespace = vals[0]
	}
	if vals, ok := form.Value["functionName"]; ok && len(vals) > 0 {
		functionName = vals[0]
	}

	if namespace == "" || functionName == "" {
		return "", "", NewError(ErrInvalidBody, "Multipart requests must contain 'namespace' and 'functionName' fields")
	}

	if !isValidRoutingIdentifier(namespace) || !isValidRoutingIdentifier(functionName) {
		return "", "", NewError(ErrInvalidBody, "Invalid characters in namespace or functionName identifier")
	}

	return namespace, functionName, nil
}

// ParseMultipartPhase2 extracts all remaining fields and streams binary files directly to disk temp storage
func ParseMultipartPhase2(reqCtx *fasthttp.RequestCtx, maxSizeBytes int64, maxFiles ...int) (*ParsedMultipartData, error) {
	form, err := reqCtx.MultipartForm()
	if err != nil {
		return nil, NewError(ErrInvalidBody, fmt.Sprintf("Failed to parse multipart fields: %v", err))
	}

	limitFiles := 20
	if len(maxFiles) > 0 && maxFiles[0] > 0 {
		limitFiles = maxFiles[0]
	}

	// Guard against DoS / memory exhaustion: count total files before reading byte streams
	totalFiles := 0
	for _, fileHeaders := range form.File {
		totalFiles += len(fileHeaders)
	}
	if limitFiles > 0 && totalFiles > limitFiles {
		return nil, NewError(ErrPayloadTooLarge, fmt.Sprintf("Exceeded maximum allowed files limit (%d)", limitFiles))
	}

	data := make(map[string]any)

	// Extract standard text values
	for k, vals := range form.Value {
		if k == "namespace" || k == "functionName" {
			continue
		}
		if len(vals) == 1 {
			data[k] = vals[0]
		} else {
			data[k] = vals
		}
	}

	var files []*UploadedFile

	// Ensure isolated temp upload directory exists with 0700 permissions
	uploadDir := filepath.Join(os.TempDir(), "crpc-uploads")
	if err := os.MkdirAll(uploadDir, 0700); err != nil {
		return nil, NewError(ErrInternal, fmt.Sprintf("Failed creating temp upload directory: %v", err))
	}

	// Extract binary files by streaming directly to disk temp files (zero in-memory accumulation)
	for fieldName, fileHeaders := range form.File {
		var fieldFiles []any
		for _, fh := range fileHeaders {
			if maxSizeBytes > 0 && fh.Size > maxSizeBytes {
				return nil, NewError(ErrPayloadTooLarge, fmt.Sprintf("File %q exceeds max allowed size of %d bytes", fh.Filename, maxSizeBytes))
			}

			src, err := fh.Open()
			if err != nil {
				return nil, NewError(ErrInternal, fmt.Sprintf("Failed opening uploaded file %q: %v", fh.Filename, err))
			}

			tempFile, err := os.CreateTemp(uploadDir, "upload-*")
			if err != nil {
				src.Close()
				return nil, NewError(ErrInternal, fmt.Sprintf("Failed creating temp file for %q: %v", fh.Filename, err))
			}
			_ = os.Chmod(tempFile.Name(), 0600)

			// Sniff initial 512 bytes for MIME detection
			sniffBuf := make([]byte, 512)
			n, err := io.ReadFull(src, sniffBuf)
			if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
				src.Close()
				tempFile.Close()
				_ = os.Remove(tempFile.Name())
				return nil, NewError(ErrInternal, fmt.Sprintf("Failed reading file header %q: %v", fh.Filename, err))
			}

			detectedMime := http.DetectContentType(sniffBuf[:n])
			declaredMime := fh.Header.Get("Content-Type")
			if declaredMime == "" {
				declaredMime = "application/octet-stream"
			}

			// Write sniffed header to temp file
			if n > 0 {
				if _, err := tempFile.Write(sniffBuf[:n]); err != nil {
					src.Close()
					tempFile.Close()
					_ = os.Remove(tempFile.Name())
					return nil, NewError(ErrInternal, fmt.Sprintf("Failed writing to temp file for %q: %v", fh.Filename, err))
				}
			}

			// Stream the remainder directly to disk
			written, err := io.Copy(tempFile, src)
			src.Close()
			tempFile.Close()
			if err != nil {
				_ = os.Remove(tempFile.Name())
				return nil, NewError(ErrInternal, fmt.Sprintf("Failed streaming uploaded file %q: %v", fh.Filename, err))
			}

			totalSize := int64(n) + written
			uploaded := NewUploadedFileDetailed(fh.Filename, declaredMime, detectedMime, totalSize, nil, tempFile.Name())
			files = append(files, uploaded)
			fieldFiles = append(fieldFiles, uploaded)
		}

		if len(fieldFiles) == 1 {
			data[fieldName] = fieldFiles[0]
		} else if len(fieldFiles) > 1 {
			data[fieldName] = fieldFiles
		}
	}

	return &ParsedMultipartData{
		Data:  data,
		Files: files,
	}, nil
}
