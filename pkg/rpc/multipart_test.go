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

type ProductInput struct {
	Title   string            `json:"title"`
	Price   float64           `json:"price"`
	InStock bool              `json:"inStock"`
	Count   int               `json:"count"`
	ZipCode string            `json:"zipCode"`
	Secret  string            `json:"-"`
	File    *rpc.UploadedFile `json:"file"`
	Media   []any             `json:"media"`
}

type ProductOutput struct {
	Price                float64 `json:"price"`
	InStock              bool    `json:"inStock"`
	Count                int     `json:"count"`
	ZipCode              string  `json:"zipCode"`
	Secret               string  `json:"secret"`
	HasFile              bool    `json:"hasFile"`
	MediaLen             int     `json:"mediaLen"`
	MediaHasUploadedFile bool    `json:"mediaHasUploadedFile"`
}

func TestMultipartPrimitiveCoercionAndMixedArray(t *testing.T) {
	app, err := rpc.New(rpc.Config{
		RpcPath: "/rpc",
		CORS:    &rpc.CorsConfig{AllowOrigins: []string{"*"}},
	})
	if err != nil {
		t.Fatalf("Failed creating app: %v", err)
	}

	rpc.Register(rpc.FunctionConfig[ProductInput, ProductOutput]{
		Namespace:    "products",
		FunctionName: "create",
		ContentType:  "multipart/form-data",
		Handler: func(ctx *rpc.Context, input ProductInput) (*ProductOutput, error) {
			hasFile := input.File != nil && input.File.Size() > 0
			mediaHasFile := false
			for _, m := range input.Media {
				if f, ok := m.(*rpc.UploadedFile); ok && f != nil && f.Size() > 0 {
					mediaHasFile = true
				}
			}

			return &ProductOutput{
				Price:                input.Price,
				InStock:              input.InStock,
				Count:                input.Count,
				ZipCode:              input.ZipCode,
				Secret:               input.Secret,
				HasFile:              hasFile,
				MediaLen:             len(input.Media),
				MediaHasUploadedFile: mediaHasFile,
			}, nil
		},
	})
	app.UseGlobalRegistry()

	handler := app.Handler()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Routing
	_ = writer.WriteField("namespace", "products")
	_ = writer.WriteField("functionName", "create")

	// Primitive strings in multipart
	_ = writer.WriteField("title", "Sneakers")
	_ = writer.WriteField("price", "125.5")
	_ = writer.WriteField("inStock", "true")
	_ = writer.WriteField("count", "42")
	_ = writer.WriteField("zipCode", "08540")

	// Mixed array: text URL under "media" key
	_ = writer.WriteField("media", "https://example.com/existing.png")

	// File 1: Single file under "file"
	partFile, _ := writer.CreateFormFile("file", "manual.pdf")
	_, _ = partFile.Write([]byte("manual binary pdf content"))

	// File 2: File under "media" (to form mixed string + UploadedFile slice)
	partMedia, _ := writer.CreateFormFile("media", "photo.jpg")
	_, _ = partMedia.Write([]byte("photo image binary content"))

	_ = writer.Close()

	var ctx fasthttp.RequestCtx
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.Header.SetContentType(writer.FormDataContentType())
	ctx.Request.SetRequestURI("/rpc")
	ctx.Request.SetBody(body.Bytes())

	handler(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("Expected 200 OK, got %d. Body: %s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}

	var resp struct {
		Status string        `json:"status"`
		Data   ProductOutput `json:"data"`
	}
	if err := json.Unmarshal(ctx.Response.Body(), &resp); err != nil {
		t.Fatalf("Failed unmarshaling response: %v", err)
	}

	if resp.Status != "ok" {
		t.Fatalf("Expected status ok, got %s. Full body: %s", resp.Status, string(ctx.Response.Body()))
	}

	// Assertions
	if resp.Data.Price != 125.5 {
		t.Errorf("Expected Price 125.5, got %v", resp.Data.Price)
	}
	if !resp.Data.InStock {
		t.Errorf("Expected InStock true, got %v", resp.Data.InStock)
	}
	if resp.Data.Count != 42 {
		t.Errorf("Expected Count 42, got %v", resp.Data.Count)
	}
	if resp.Data.ZipCode != "08540" {
		t.Errorf("Expected ZipCode '08540', got %q", resp.Data.ZipCode)
	}
	if resp.Data.Secret != "" {
		t.Errorf("Expected Secret to be empty (ignored via json:\"-\"), got %q", resp.Data.Secret)
	}
	if !resp.Data.HasFile {
		t.Errorf("Expected HasFile true, got %v", resp.Data.HasFile)
	}
	if resp.Data.MediaLen != 2 {
		t.Errorf("Expected MediaLen 2 (1 string + 1 file), got %d", resp.Data.MediaLen)
	}
	if !resp.Data.MediaHasUploadedFile {
		t.Errorf("Expected MediaHasUploadedFile true, got %v", resp.Data.MediaHasUploadedFile)
	}

	// Test injectFilesIntoStruct skips json:"-"
	type IgnoredFieldStruct struct {
		IgnoredFile *rpc.UploadedFile `json:"-"`
		ValidFile   *rpc.UploadedFile `json:"validFile"`
	}
	var testTarget IgnoredFieldStruct
	dummyFile := rpc.NewUploadedFile("dummy.txt", "text/plain", []byte("dummy"), "/tmp/dummy")
	rpc.InjectFilesForTest(&testTarget, map[string]any{
		"IgnoredFile": dummyFile,
		"validFile":   dummyFile,
	})
	if testTarget.IgnoredFile != nil {
		t.Errorf("Expected IgnoredFile to remain nil due to json:\"-\" tag")
	}
	if testTarget.ValidFile == nil {
		t.Errorf("Expected ValidFile to be injected")
	}
}

func TestMultipartJSONFieldPrototypeSanitization(t *testing.T) {
	app, err := rpc.New(rpc.Config{
		RpcPath: "/rpc",
		CORS:    &rpc.CorsConfig{AllowOrigins: []string{"*"}},
	})
	if err != nil {
		t.Fatalf("Failed creating app: %v", err)
	}

	type ConfigInput struct {
		Config map[string]any `json:"config"`
	}
	type ConfigOutput struct {
		HasProto bool   `json:"hasProto"`
		Name     string `json:"name"`
	}

	rpc.Register(rpc.FunctionConfig[ConfigInput, ConfigOutput]{
		Namespace:    "system",
		FunctionName: "configure",
		ContentType:  "multipart/form-data",
		Handler: func(ctx *rpc.Context, input ConfigInput) (*ConfigOutput, error) {
			_, hasProto := input.Config["__proto__"]
			name, _ := input.Config["name"].(string)
			return &ConfigOutput{
				HasProto: hasProto,
				Name:     name,
			}, nil
		},
	})
	app.UseGlobalRegistry()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("namespace", "system")
	_ = writer.WriteField("functionName", "configure")
	_ = writer.WriteField("config", `{"__proto__":{"admin":true},"name":"production"}`)
	_ = writer.Close()

	var ctx fasthttp.RequestCtx
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.Header.SetContentType(writer.FormDataContentType())
	ctx.Request.SetRequestURI("/rpc")
	ctx.Request.SetBody(body.Bytes())

	app.Handler()(&ctx)

	var resp struct {
		Status string       `json:"status"`
		Data   ConfigOutput `json:"data"`
	}
	_ = json.Unmarshal(ctx.Response.Body(), &resp)

	if resp.Status != "ok" {
		t.Fatalf("Expected ok, got %s. Body: %s", resp.Status, string(ctx.Response.Body()))
	}
	if resp.Data.HasProto {
		t.Errorf("Expected __proto__ to be stripped from JSON multipart field")
	}
	if resp.Data.Name != "production" {
		t.Errorf("Expected name 'production', got %q", resp.Data.Name)
	}
}
