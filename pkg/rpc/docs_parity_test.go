package rpc

import (
	"strings"
	"testing"

	"github.com/CellsWeb/crpc-go/pkg/schema"
)

func TestDocsCleanupAndMiddlewareParity(t *testing.T) {
	const configuredSecret = "CONFIGURED_SECRET_MUST_NOT_APPEAR"
	const schemaSecret = "SCHEMA_SECRET_MUST_NOT_APPEAR"
	options := schema.Object(map[string]*schema.SchemaRule{
		"secret": schema.String().WithDefault(schemaSecret).Describe("Option description marker"),
		"credentials": schema.Optional(schema.Object(map[string]*schema.SchemaRule{
			"key": schema.String().WithDefault(schemaSecret).Describe("Nested option marker"),
		})),
		"default": schema.Optional(schema.String().Describe("An option named default")),
	})
	headers := schema.Object(map[string]*schema.SchemaRule{
		"authorization": schema.String().WithDefault(schemaSecret).Describe("Required bearer marker"),
		"x-trace":       schema.Optional(schema.String().Describe("Optional trace marker")),
	})
	mws := map[string]*RegisteredMiddleware{
		"guard":   {Metadata: MiddlewareMetadata{Name: "guard", Description: "Middleware description marker", Options: options, Headers: headers}},
		"untyped": {Metadata: MiddlewareMetadata{Name: "untyped"}},
	}
	functions := map[string]*FunctionDefinition{}
	for _, name := range []string{"omitted", "legacyTrue", "legacyFalse"} {
		metadata := FunctionMetadata{Description: "Function description marker"}
		if name == "legacyTrue" {
			metadata.IsPublic = true
		}
		functions["catalog."+name] = &FunctionDefinition{Key: "catalog." + name, Namespace: "catalog", FunctionName: name,
			Metadata: metadata, ContentType: "application/json", InputSchema: schema.Object(map[string]*schema.SchemaRule{"name": schema.String().Describe("Input schema marker")}),
			Middlewares: []MiddlewareAttachment{M("guard", map[string]any{"secret": configuredSecret, "credentials": map[string]any{"key": configuredSecret}}), M("untyped", map[string]any{"untypedSecret": configuredSecret})},
		}
	}
	pages := map[string]string{"index": renderDocsIndex("/reference/api", "/rpc", "http://localhost", functions, mws)}
	for name, fn := range functions {
		pages[name] = renderFunctionDocs("/reference/api", "/rpc", "http://localhost", fn, mws, functions)
	}
	for name, mw := range mws {
		pages["middleware "+name] = renderMiddlewareDoc("/reference/api", mw, functions, mws)
	}
	for name, html := range pages {
		t.Run(name, func(t *testing.T) {
			for _, forbidden := range []string{configuredSecret, schemaSecret, ">PUBLIC</span>", ">PRIVATE</span>"} {
				if strings.Contains(html, forbidden) {
					t.Errorf("documentation exposed %q", forbidden)
				}
			}
		})
	}
	detail := pages["catalog.legacyFalse"]
	for _, marker := range []string{"Middleware description marker", "Option description marker", "Nested option marker", "An option named default", "Required bearer marker", "Optional trace marker", "required", "optional", "string", `href="/reference/api/middlewares/guard"`} {
		if !strings.Contains(detail, marker) {
			t.Errorf("missing parity field %q", marker)
		}
	}
	if !strings.Contains(detail, `<td class="header-key">authorization</td>`) {
		t.Error("missing required header")
	}
	if strings.Contains(detail, `<td class="header-key">x-trace</td>`) || strings.Contains(detail, "YOUR_X_TRACE") {
		t.Error("optional header misrepresented as required")
	}
	if !strings.Contains(pages["middleware guard"], "Headers JSON Schema") {
		t.Error("missing headers schema")
	}
	if options.Properties["secret"].Default != schemaSecret || headers.Properties["authorization"].Default != schemaSecret {
		t.Error("runtime schemas mutated")
	}
	if !functions["catalog.legacyTrue"].Metadata.IsPublic {
		t.Error("compatibility metadata mutated")
	}
	copy := *functions["catalog.legacyFalse"]
	copy.ContentType = "multipart/form-data"
	multipart := renderFunctionDocs("/reference/api", "/rpc", "http://localhost", &copy, mws, functions)
	if !strings.Contains(multipart, "YOUR_AUTHORIZATION") || strings.Contains(multipart, "YOUR_X_TRACE") {
		t.Error("multipart cURL header placeholders incorrect")
	}
}
