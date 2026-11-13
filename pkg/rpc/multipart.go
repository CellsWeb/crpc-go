package rpc

import (
	"bytes"
	"fmt"
	"mime/multipart"

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

// ParseMultipartPhase2 extracts all remaining fields and binary files after middlewares have verified access
func ParseMultipartPhase2(reqCtx *fasthttp.RequestCtx, maxSizeBytes int64) (*ParsedMultipartData, error) {
	form, err := reqCtx.MultipartForm()
	if err != nil {
		return nil, NewError(ErrInvalidBody, fmt.Sprintf("Failed to parse multipart fields: %v", err))
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

	// Extract binary files
	for fieldName, fileHeaders := range form.File {
		for _, fh := range fileHeaders {
			if maxSizeBytes > 0 && fh.Size > maxSizeBytes {
				return nil, NewError(ErrPayloadTooLarge, fmt.Sprintf("File %q exceeds max allowed size of %d bytes", fh.Filename, maxSizeBytes))
			}

			fileContent, err := readMultipartFile(fh)
			if err != nil {
				return nil, NewError(ErrInternal, fmt.Sprintf("Failed reading uploaded file %q: %v", fh.Filename, err))
			}

			mimeType := fh.Header.Get("Content-Type")
			if mimeType == "" {
				mimeType = "application/octet-stream"
			}

			uploaded := NewUploadedFile(fh.Filename, mimeType, fileContent, "")
			files = append(files, uploaded)
			data[fieldName] = uploaded
		}
	}

	return &ParsedMultipartData{
		Data:  data,
		Files: files,
	}, nil
}

func readMultipartFile(fh *multipart.FileHeader) ([]byte, error) {
	f, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(f); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
