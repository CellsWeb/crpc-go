package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/valyala/fasthttp"
)

// ParsedMultipartData holds the extracted routing parameters, fields, and file references
type ParsedMultipartData struct {
	Namespace    string
	FunctionName string
	Data         map[string]any
	Files        []*UploadedFile
}

// ParseMultipartPhase1 parses only the routing keys (namespace, functionName) from multipart stream without reading any files
func ParseMultipartPhase1(reqCtx *fasthttp.RequestCtx) (namespace, functionName string, err error) {
	boundary := string(reqCtx.Request.Header.MultipartFormBoundary())
	if boundary == "" {
		return "", "", NewError(ErrInvalidBody, "Missing multipart boundary in Content-Type")
	}

	mr := multipart.NewReader(bytes.NewReader(reqCtx.PostBody()), boundary)

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", "", NewError(ErrInvalidBody, fmt.Sprintf("Malformed multipart stream: %v", err))
		}

		// Reject file parts that appear before routing fields
		if part.FileName() != "" && (namespace == "" || functionName == "") {
			return "", "", NewError(ErrInvalidBody, "Routing fields ('namespace', 'functionName') must appear before file parts in multipart payload")
		}

		fieldName := part.FormName()
		if fieldName == "namespace" {
			if namespace != "" {
				return "", "", NewError(ErrInvalidBody, "Duplicate 'namespace' field in multipart payload")
			}
			buf := make([]byte, 129)
			n, _ := io.ReadFull(part, buf)
			if n > 128 {
				return "", "", NewError(ErrInvalidBody, "namespace field exceeds 128 characters")
			}
			namespace = string(buf[:n])
		} else if fieldName == "functionName" {
			if functionName != "" {
				return "", "", NewError(ErrInvalidBody, "Duplicate 'functionName' field in multipart payload")
			}
			buf := make([]byte, 129)
			n, _ := io.ReadFull(part, buf)
			if n > 128 {
				return "", "", NewError(ErrInvalidBody, "functionName field exceeds 128 characters")
			}
			functionName = string(buf[:n])
		}

		// Halt parsing the stream immediately once both routing keys are resolved
		// Zero file parts are read or persisted to disk
		if namespace != "" && functionName != "" {
			break
		}
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

	if len(form.Value["namespace"]) > 1 || len(form.Value["functionName"]) > 1 {
		return nil, NewError(ErrInvalidBody, "Duplicate routing fields ('namespace' or 'functionName') in multipart payload")
	}

	data := make(map[string]any)
	var bodyNs, bodyFn string
	if len(form.Value["namespace"]) > 0 {
		bodyNs = form.Value["namespace"][0]
	}
	if len(form.Value["functionName"]) > 0 {
		bodyFn = form.Value["functionName"][0]
	}

	parseJsonField := func(val string) any {
		trimmed := strings.TrimSpace(val)
		if (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
			(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")) {
			var parsed any
			if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
				return parsed
			}
		}
		return val
	}

	// Extract standard text values
	for k, vals := range form.Value {
		if k == "namespace" || k == "functionName" || k == "__proto__" || k == "constructor" || k == "prototype" {
			continue
		}
		actualKey := k
		if k == "_data_namespace" {
			actualKey = "namespace"
		} else if k == "_data_functionName" {
			actualKey = "functionName"
		}

		if len(vals) == 1 {
			data[actualKey] = parseJsonField(vals[0])
		} else {
			var parsedArr []any
			for _, v := range vals {
				parsedArr = append(parsedArr, parseJsonField(v))
			}
			data[actualKey] = parsedArr
		}
	}

	var files []*UploadedFile
	cleanupCreatedFiles := func() {
		for _, f := range files {
			if f != nil && f.TempPath() != "" {
				_ = os.Remove(f.TempPath())
			}
		}
	}

	// Ensure isolated temp upload directory exists with 0700 permissions
	uploadDir := filepath.Join(os.TempDir(), "crpc-uploads")
	if err := os.MkdirAll(uploadDir, 0700); err != nil {
		return nil, NewError(ErrInternal, fmt.Sprintf("Failed creating temp upload directory: %v", err))
	}

	// Extract binary files by streaming directly to disk temp files (zero in-memory accumulation)
	for fieldName, fileHeaders := range form.File {
		if fieldName == "__proto__" || fieldName == "constructor" || fieldName == "prototype" {
			continue
		}
		var fieldFiles []any
		for _, fh := range fileHeaders {
			if maxSizeBytes > 0 && fh.Size > maxSizeBytes {
				cleanupCreatedFiles()
				return nil, NewError(ErrPayloadTooLarge, fmt.Sprintf("File %q exceeds max allowed size of %d bytes", fh.Filename, maxSizeBytes))
			}

			src, err := fh.Open()
			if err != nil {
				cleanupCreatedFiles()
				return nil, NewError(ErrInternal, fmt.Sprintf("Failed opening uploaded file %q: %v", fh.Filename, err))
			}

			tempFile, err := os.CreateTemp(uploadDir, "upload-*")
			if err != nil {
				src.Close()
				cleanupCreatedFiles()
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
				cleanupCreatedFiles()
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
					cleanupCreatedFiles()
					return nil, NewError(ErrInternal, fmt.Sprintf("Failed writing to temp file for %q: %v", fh.Filename, err))
				}
			}

			// Stream the remainder directly to disk
			written, err := io.Copy(tempFile, src)
			src.Close()
			tempFile.Close()
			if err != nil {
				_ = os.Remove(tempFile.Name())
				cleanupCreatedFiles()
				return nil, NewError(ErrInternal, fmt.Sprintf("Failed streaming uploaded file %q: %v", fh.Filename, err))
			}

			totalSize := int64(n) + written
			uploaded := NewUploadedFileDetailed(fh.Filename, declaredMime, detectedMime, totalSize, nil, tempFile.Name())
			files = append(files, uploaded)
			fieldFiles = append(fieldFiles, uploaded)
		}

		actualKey := fieldName
		if fieldName == "_data_namespace" {
			actualKey = "namespace"
		} else if fieldName == "_data_functionName" {
			actualKey = "functionName"
		}

		existing, exists := data[actualKey]
		if !exists {
			if len(fieldFiles) == 1 {
				data[actualKey] = fieldFiles[0]
			} else {
				data[actualKey] = fieldFiles
			}
		} else {
			// Merge existing text value(s) with file(s) into a unified slice
			var merged []any
			if existingSlice, ok := existing.([]any); ok {
				merged = append(merged, existingSlice...)
			} else {
				merged = append(merged, existing)
			}
			merged = append(merged, fieldFiles...)
			data[actualKey] = merged
		}
	}

	return &ParsedMultipartData{
		Namespace:    bodyNs,
		FunctionName: bodyFn,
		Data:         data,
		Files:        files,
	}, nil
}
