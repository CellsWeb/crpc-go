package rpc

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/CellsWeb/crpc-go/pkg/schema"
	"github.com/valyala/fasthttp"
)

var docsVisibilityCases = []struct {
	name, metadata string
	hidden         bool
}{
	{"omitted", `{}`, false},
	{"public", `{"isPublic":true}`, false},
	{"legacyPrivate", `{"isPublic":false}`, false},
	{"explicitVisible", `{"hideFromDocs":false}`, false},
	{"legacyPrivateVisible", `{"isPublic":false,"hideFromDocs":false}`, false},
	{"concealed", `{"hideFromDocs":true}`, true},
	{"concealedPublic", `{"isPublic":true,"hideFromDocs":true}`, true},
	{"concealedPrivate", `{"isPublic":false,"hideFromDocs":true}`, true},
}

func docsVisibilityApp(t *testing.T, docsPath string) (*CoreRPC, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	app, err := New(Config{RpcPath: "/rpc", DocsEnabled: true, DocsPath: docsPath, CORS: &CorsConfig{AllowOrigins: []string{"*"}}})
	if err != nil {
		t.Fatal(err)
	}
	middlewareCalls, handlerCalls := new(atomic.Int32), new(atomic.Int32)
	app.RegisterMiddleware("guard", "Auth middleware", func(ctx *Context, headers map[string]string, _ map[string]any) error {
		middlewareCalls.Add(1)
		if headers["authorization"] != "Bearer valid" {
			return NewError(ErrUnauthorized, "Invalid token")
		}
		ctx.Set("actor", "reader")
		return nil
	}, nil, nil)
	for _, tc := range docsVisibilityCases {
		// JSON keeps the regression compilable before the additive metadata field exists.
		var metadata FunctionMetadata
		if err := json.Unmarshal([]byte(tc.metadata), &metadata); err != nil {
			t.Fatal(err)
		}
		metadata.Description = "marker_" + tc.name
		metadata.Tags = []string{"tag_" + tc.name}
		namespace := "catalog"
		if tc.hidden {
			namespace = "vault.internal"
		}
		app.RegisterFunction(&FunctionDefinition{
			Namespace: namespace, FunctionName: tc.name, Metadata: metadata,
			InputSchema:  schema.Object(map[string]*schema.SchemaRule{}).Describe("input_schema_" + tc.name),
			OutputSchema: schema.Object(map[string]*schema.SchemaRule{"echo": schema.String()}).Describe("output_schema_" + tc.name),
			Middlewares:  DefineMiddlewares(M("guard", nil)),
			Handler: func(ctx *Context, _ any) (any, error) {
				handlerCalls.Add(1)
				if actor, _ := ctx.Get("actor"); actor != "reader" {
					return nil, fmt.Errorf("middleware context was lost")
				}
				return map[string]any{"echo": "ok"}, nil
			},
		})
	}
	return app, middlewareCalls, handlerCalls
}

func expectNoHiddenDocs(t *testing.T, body string) {
	t.Helper()
	for _, tc := range docsVisibilityCases {
		if !tc.hidden {
			continue
		}
		for _, token := range []string{tc.name, "marker_" + tc.name, "tag_" + tc.name, "input_schema_" + tc.name, "output_schema_" + tc.name} {
			if strings.Contains(body, token) {
				t.Errorf("hidden documentation token leaked: %s", token)
			}
		}
	}
	if strings.Contains(body, "vault.internal") || strings.Contains(body, "vault/internal") {
		t.Error("hidden-only namespace leaked")
	}
}

func visibilityDocsRequest(app *CoreRPC, uri string) (int, string) {
	var ctx fasthttp.RequestCtx
	ctx.Request.SetRequestURI(uri)
	ctx.Request.Header.SetMethod("GET")
	app.Handler()(&ctx)
	return ctx.Response.StatusCode(), string(ctx.Response.Body())
}

func TestDocsVisibilityOptOut(t *testing.T) {
	for _, docsPath := range []string{"/docs", "/custom/reference"} {
		t.Run(docsPath, func(t *testing.T) {
			app, middlewareCalls, handlerCalls := docsVisibilityApp(t, docsPath)
			for _, uri := range []string{docsPath, docsPath + "/catalog/omitted", docsPath + "/middlewares/guard", docsPath + "/missing/function"} {
				_, body := visibilityDocsRequest(app, uri)
				expectNoHiddenDocs(t, body)
			}
			_, index := visibilityDocsRequest(app, docsPath)
			for _, tc := range docsVisibilityCases {
				if tc.hidden {
					continue
				}
				if !strings.Contains(index, "marker_"+tc.name) {
					t.Errorf("visible/default/legacy function missing: %s", tc.name)
				}
			}
			if !strings.Contains(index, "5 functions across 1 namespaces") {
				t.Error("counts include hidden functions or namespaces")
			}
			if !strings.Contains(index, "PRIVATE") || !strings.Contains(index, "PUBLIC") {
				t.Error("legacy badge classification changed")
			}
			for _, tc := range docsVisibilityCases {
				if !tc.hidden {
					continue
				}
				for _, uri := range []string{docsPath + "/vault/internal/" + tc.name + "/?x=1", docsPath + "/fn/vault.internal." + tc.name} {
					status, body := visibilityDocsRequest(app, uri)
					if status != 404 {
						t.Errorf("hidden detail URL returned %d: %s", status, uri)
					}
					expectNoHiddenDocs(t, body)
				}
			}
			// Check renderer entrypoints as well as HTTP routing, using the unfiltered execution registry.
			functions, middlewares := app.Functions(), app.Middlewares()
			for _, body := range []string{
				renderDocsIndex(docsPath, "/rpc", "", functions, middlewares),
				renderFunctionDocs(docsPath, "/rpc", "", functions["catalog.omitted"], middlewares, functions),
				renderFunctionDocs(docsPath, "/rpc", "", functions["vault.internal.concealed"], middlewares, functions),
				renderMiddlewareDoc(docsPath, middlewares["guard"], functions, middlewares),
			} {
				expectNoHiddenDocs(t, body)
			}
			if len(app.Functions()) != 8 || middlewareCalls.Load() != 0 || handlerCalls.Load() != 0 {
				t.Fatal("docs mutated the execution registry or dispatched RPC")
			}
		})
	}
}

func TestDocsVisibilityAllHidden(t *testing.T) {
	app, _, _ := docsVisibilityApp(t, "/docs")
	functions := app.Functions()
	for key, fn := range functions {
		if fn.Namespace != "vault.internal" {
			delete(functions, key)
		}
	}
	body := renderDocsIndex("/docs", "/rpc", "", functions, app.Middlewares())
	expectNoHiddenDocs(t, body)
	if !strings.Contains(body, "0 functions across 0 namespaces") || len(functions) != 3 {
		t.Fatal("all-hidden directory did not preserve empty view/source registry")
	}
}

func TestDocsVisibilityDoesNotAuthorizeRPC(t *testing.T) {
	app, middlewareCalls, handlerCalls := docsVisibilityApp(t, "/docs")
	_, _ = visibilityDocsRequest(app, "/docs")
	for _, tc := range docsVisibilityCases {
		t.Run(tc.name, func(t *testing.T) {
			namespace := "catalog"
			if tc.hidden {
				namespace = "vault.internal"
			}
			beforeMiddleware, beforeHandler := middlewareCalls.Load(), handlerCalls.Load()
			for _, authorized := range []bool{false, true} {
				var ctx fasthttp.RequestCtx
				ctx.Request.SetRequestURI("/rpc")
				ctx.Request.Header.SetMethod("POST")
				ctx.Request.Header.SetContentType("application/json")
				if authorized {
					ctx.Request.Header.Set("Authorization", "Bearer valid")
				}
				ctx.Request.SetBodyString(fmt.Sprintf(`{"namespace":%q,"functionName":%q,"data":{}}`, namespace, tc.name))
				app.Handler()(&ctx)
				var response ResponseEnvelope
				if err := json.Unmarshal(ctx.Response.Body(), &response); err != nil {
					t.Fatal(err)
				}
				if ctx.Response.StatusCode() != 200 {
					t.Fatal("RPC HTTP status changed")
				}
				if authorized {
					if response.Status != "ok" {
						t.Fatalf("documentation metadata denied an authorized RPC: %+v", response)
					}
				} else if response.Status != "error" || response.Code != ErrUnauthorized {
					t.Fatalf("documentation metadata bypassed middleware: %+v", response)
				}
			}
			var output map[string]any
			if err := app.CallInMemory(namespace+"."+tc.name, map[string]any{}, &output, CallOptions{Headers: map[string]string{"authorization": "Bearer valid"}}); err != nil {
				t.Fatal(err)
			}
			if output["echo"] != "ok" || middlewareCalls.Load() != beforeMiddleware+3 || handlerCalls.Load() != beforeHandler+2 {
				t.Fatal("execution/middleware changed after docs rendering")
			}
		})
	}
	t.Run("hidden_open_function", func(t *testing.T) {
		fn := app.Functions()["vault.internal.concealed"]
		fn.Middlewares = nil
		fn.Handler = func(*Context, any) (any, error) { return map[string]any{"echo": "open hidden function"}, nil }
		var output map[string]any
		if err := app.CallInMemory(fn.Key, map[string]any{}, &output); err != nil || output["echo"] != "open hidden function" {
			t.Fatalf("hidden metadata denied an open function: %v", err)
		}
	})
}

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
