package rpc_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"testing"

	"github.com/CellsWeb/crpc-go/pkg/rpc"
	"github.com/CellsWeb/crpc-go/pkg/schema"
	"github.com/valyala/fasthttp"
)

type DocumentInput struct {
	Title string              `json:"title"`
	File  *rpc.UploadedFile   `json:"file"`
	Docs  []*rpc.UploadedFile `json:"docs,omitempty" min:"1" max:"3"`
}

type DocumentOutput struct {
	ReceivedTitle string `json:"receivedTitle"`
	FileSize      int64  `json:"fileSize"`
	DocsCount     int    `json:"docsCount"`
}

func TestMultipartMaxFilesAndCount(t *testing.T) {
	app, err := rpc.New(rpc.Config{
		RpcPath:           "/rpc",
		MaxMultipartFiles: 3, // Strict limit: max 3 files total per request
		CORS:              &rpc.CorsConfig{AllowOrigins: []string{"*"}},
	})
	if err != nil {
		t.Fatalf("Failed creating app: %v", err)
	}

	rpc.Register(rpc.FunctionConfig[DocumentInput, DocumentOutput]{
		Namespace:    "files",
		FunctionName: "upload",
		InputSchema:  schema.FromStruct(DocumentInput{}),
		Handler: func(ctx *rpc.Context, input DocumentInput) (*DocumentOutput, error) {
			return &DocumentOutput{
				ReceivedTitle: input.Title,
				FileSize:      input.File.Size(),
				DocsCount:     len(input.Docs),
			}, nil
		},
	})
	app.UseGlobalRegistry()

	handler := app.Handler()

	// Helper to send multipart request
	sendMultipart := func(fields map[string]string, fileMap map[string][][]byte) *fasthttp.Response {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)

		for k, v := range fields {
			_ = writer.WriteField(k, v)
		}
		for fieldName, fileList := range fileMap {
			for i, data := range fileList {
				part, _ := writer.CreateFormFile(fieldName, "test_file.txt")
				_, _ = part.Write(data)
				_ = i
			}
		}
		_ = writer.Close()

		var ctx fasthttp.RequestCtx
		ctx.Request.Header.SetMethod("POST")
		ctx.Request.Header.SetContentType(writer.FormDataContentType())
		ctx.Request.SetRequestURI("/rpc")
		ctx.Request.SetBody(body.Bytes())

		handler(&ctx)
		res := &fasthttp.Response{}
		ctx.Response.CopyTo(res)
		return res
	}

	// 1. Success case: 1 main file + 2 docs (total 3 files <= MaxMultipartFiles 3)
	res := sendMultipart(
		map[string]string{
			"namespace":    "files",
			"functionName": "upload",
			"title":        "Invoice Batch",
		},
		map[string][][]byte{
			"file": {[]byte("main invoice content")},
			"docs": {[]byte("receipt 1"), []byte("receipt 2")},
		},
	)

	if res.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("Expected 200, got %d. Body: %s", res.StatusCode(), string(res.Body()))
	}
	var successResp struct {
		Status string         `json:"status"`
		Data   DocumentOutput `json:"data"`
	}
	if err := json.Unmarshal(res.Body(), &successResp); err != nil {
		t.Fatalf("Failed unmarshaling success response: %v", err)
	}
	if successResp.Status != "ok" || successResp.Data.DocsCount != 2 {
		t.Fatalf("Unexpected response data: %+v", successResp)
	}

	// 2. Failure: Total files exceed MaxMultipartFiles (1 main + 3 docs = 4 files > 3)
	resExceeded := sendMultipart(
		map[string]string{
			"namespace":    "files",
			"functionName": "upload",
			"title":        "Too many files",
		},
		map[string][][]byte{
			"file": {[]byte("main file")},
			"docs": {[]byte("doc 1"), []byte("doc 2"), []byte("doc 3")},
		},
	)
	var errResp struct {
		Status  string `json:"status"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(resExceeded.Body(), &errResp)
	if errResp.Status != "error" || errResp.Code != rpc.ErrPayloadTooLarge {
		t.Fatalf("Expected payload_too_large error, got: %+v", errResp)
	}
}
