package rpc

import (
	"strings"
	"testing"

	"github.com/CellsWeb/crpc-go/pkg/schema"
	"github.com/valyala/fasthttp"
)

type NestedAddress struct {
	Street string `json:"street" doc:"Street line"`
	City   string `json:"city" doc:"City name"`
}

type ComplexUserDocInput struct {
	ID      string        `json:"id" doc:"User ID"`
	Address NestedAddress `json:"address" doc:"User address"`
	Tags    []string      `json:"tags,omitempty" doc:"Optional user tags"`
}

type ComplexUserDocOutput struct {
	Success bool   `json:"success" doc:"Operation status"`
	Message string `json:"message" doc:"Confirmation message"`
}

func TestDocsRendererComplete(t *testing.T) {
	app, err := New(Config{
		DocsEnabled: true,
		DocsPath:    "/docs",
		RpcPath:     "/rpc",
		CORS:        &CorsConfig{AllowOrigins: []string{"*"}},
	})
	if err != nil {
		t.Fatalf("failed to create CoreRPC: %v", err)
	}

	// Register auth middleware with options and headers schema
	authOptions := schema.Object(map[string]*schema.SchemaRule{
		"roles": schema.Array(schema.String()).Describe("Allowed user roles for this endpoint"),
	})
	authHeaders := schema.Object(map[string]*schema.SchemaRule{
		"authorization": schema.String().Describe("Bearer JWT token for authentication"),
	})
	RegisterMiddleware("docAuth", "Validates bearer token and enforces roles", func(ctx *Context, headers map[string]string, options map[string]any) error {
		return nil
	}, authOptions, authHeaders)

	// Register a test function with middlewares, metadata, and nested schemas
	Register(FunctionConfig[ComplexUserDocInput, ComplexUserDocOutput]{
		Namespace:    "user",
		FunctionName: "createProfile",
		Metadata: FunctionMetadata{
			Description: "Creates user profile with address and tags",
			IsPublic:    true,
			Tags:        []string{"users", "profile"},
		},
		Middlewares: DefineMiddlewares(
			M("docAuth", map[string]any{"roles": []string{"admin", "user"}}),
		),
		Handler: func(ctx *Context, input ComplexUserDocInput) (*ComplexUserDocOutput, error) {
			return &ComplexUserDocOutput{Success: true, Message: "Profile created"}, nil
		},
	})

	app.UseGlobalRegistry()

	// Helper to perform FastHTTP requests to the app handler
	performRequest := func(uri string) (int, string) {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.SetRequestURI(uri)
		ctx.Request.Header.SetMethod("GET")
		ctx.Request.Header.SetHost("localhost:4000")
		app.Handler()(ctx)
		return ctx.Response.StatusCode(), string(ctx.Response.Body())
	}

	// 1. Test /docs Index
	t.Run("Docs Index Page", func(t *testing.T) {
		status, body := performRequest("/docs")
		if status != 200 {
			t.Fatalf("expected 200, got %d", status)
		}

		// Verify Sidebar
		if !strings.Contains(body, `<nav class="sidebar">`) {
			t.Errorf("missing sidebar navigation in docs index")
		}
		if !strings.Contains(body, `CellsWeb RPC`) || !strings.Contains(body, `DOCS`) {
			t.Errorf("missing brand logo or badge in sidebar")
		}
		if !strings.Contains(body, `<div class="sidebar-ns">user</div>`) {
			t.Errorf("missing user namespace group in sidebar")
		}
		if !strings.Contains(body, `href="/docs/user/createProfile"`) {
			t.Errorf("missing createProfile link in sidebar")
		}

		// Verify Search Bar and Central Middlewares
		if !strings.Contains(body, `id="search-input"`) {
			t.Errorf("missing search bar on index page")
		}
		if !strings.Contains(body, `Central Middlewares Catalog`) {
			t.Errorf("missing central middlewares catalog section")
		}
		if !strings.Contains(body, `⚡ docAuth`) {
			t.Errorf("missing docAuth middleware in central catalog")
		}
		if !strings.Contains(body, `Properties &amp; Controls:`) && !strings.Contains(body, `Properties & Controls:`) {
			t.Errorf("missing properties & controls in middleware card")
		}
		if !strings.Contains(body, `Required Headers:`) {
			t.Errorf("missing required headers in middleware card")
		}

		// Verify Error Reference Guide
		if !strings.Contains(body, `Error Reference`) || !strings.Contains(body, `validation_error`) {
			t.Errorf("missing error reference section")
		}
	})

	// 2. Test Function Detail View: /docs/user/createProfile
	t.Run("Function Detail View", func(t *testing.T) {
		status, body := performRequest("/docs/user/createProfile")
		if status != 200 {
			t.Fatalf("expected 200, got %d", status)
		}

		// Check Active Sidebar Item
		if !strings.Contains(body, `sidebar-fn active`) {
			t.Errorf("expected active sidebar item for current function")
		}

		// Check Breadcrumb and Title
		if !strings.Contains(body, `createProfile`) {
			t.Errorf("missing function title createProfile")
		}
		if !strings.Contains(body, `Creates user profile with address and tags`) {
			t.Errorf("missing function description")
		}
		if !strings.Contains(body, `PUBLIC`) {
			t.Errorf("missing PUBLIC badge")
		}

		// Check Middleware Pipeline & Applied Controls
		if !strings.Contains(body, `Middleware Pipeline &amp; Applied Controls (1)`) &&
			!strings.Contains(body, `Middleware Pipeline & Applied Controls (1)`) {
			t.Errorf("missing middleware pipeline section")
		}
		if !strings.Contains(body, `roles:`) || !strings.Contains(body, `admin`) {
			t.Errorf("missing configured options for middleware auth")
		}

		// Check Aggregated Required Headers Table
		if !strings.Contains(body, `Required HTTP Headers`) {
			t.Errorf("missing required HTTP headers table")
		}
		if !strings.Contains(body, `authorization`) {
			t.Errorf("missing authorization header in required headers table")
		}

		// Check cURL Command
		if !strings.Contains(body, `Terminal cURL Command`) {
			t.Errorf("missing terminal cURL snippet")
		}
		if !strings.Contains(body, `curl -X POST`) || !strings.Contains(body, `createProfile`) {
			t.Errorf("cURL snippet does not contain correct endpoint")
		}

		// Check Request and Response Payload Examples
		if !strings.Contains(body, `Request Payload Example`) || !strings.Contains(body, `Response Payload Example`) {
			t.Errorf("missing request or response payload examples")
		}
		if !strings.Contains(body, `clx123abc456`) {
			t.Errorf("response example missing meta.requestId")
		}

		// Check 2-Column Side-by-Side Schema View
		if !strings.Contains(body, `Input Data Contract`) || !strings.Contains(body, `Output Data Contract`) {
			t.Errorf("missing input or output contract sections")
		}
		if !strings.Contains(body, `split-grid`) || !strings.Contains(body, `split-panel`) {
			t.Errorf("missing 2-column split-grid layout")
		}

		// Check Recursive Fields List (nested struct address)
		if !strings.Contains(body, `Required Data Fields`) {
			t.Errorf("missing left column Required Data Fields")
		}
		if !strings.Contains(body, `street`) || !strings.Contains(body, `city`) {
			t.Errorf("missing nested struct properties (street, city) in recursive fields list")
		}
		if !strings.Contains(body, `Street line`) || !strings.Contains(body, `City name`) {
			t.Errorf("missing descriptions on nested properties")
		}

		// Check JSON Schema Code Block with Copy Button
		if !strings.Contains(body, `JSON Schema`) {
			t.Errorf("missing JSON Schema right column")
		}
		if !strings.Contains(body, `copyCode(this)`) {
			t.Errorf("missing Copy button handler")
		}
	})

	// 3. Test Middleware Detail View: /docs/middlewares/docAuth
	t.Run("Middleware Detail View", func(t *testing.T) {
		status, body := performRequest("/docs/middlewares/docAuth")
		if status != 200 {
			t.Fatalf("expected 200, got %d", status)
		}

		if !strings.Contains(body, `⚡ docAuth`) {
			t.Errorf("missing middleware title")
		}
		if !strings.Contains(body, `Options JSON Schema`) {
			t.Errorf("missing options json schema section")
		}
		if !strings.Contains(body, `Required Headers JSON Schema`) {
			t.Errorf("missing headers json schema section")
		}
	})

	// 4. Test 404 on Unknown Doc Path
	t.Run("Docs 404", func(t *testing.T) {
		status, body := performRequest("/docs/nonexistent/endpoint")
		if status != 404 {
			t.Fatalf("expected 404, got %d", status)
		}
		if !strings.Contains(body, `404`) {
			t.Errorf("expected 404 body content")
		}
	})
}
