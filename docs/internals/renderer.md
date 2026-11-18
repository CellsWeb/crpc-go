# Go embedded docs renderer

Source: [pkg/rpc/docs.go](https://github.com/CellsWeb/crpc-go/blob/main/pkg/rpc/docs.go) with native schema export in
[pkg/schema/jsonschema.go](https://github.com/CellsWeb/crpc-go/blob/main/pkg/schema/jsonschema.go). This is the runtime /docs feature, separate
from the MkDocs contributor site.

## Routing and views

CoreRPC.Handler checks global DocsEnabled before handleDocsRequest. DocsPath is
normalized; the request path chooses index, middleware detail or function detail.
Known details render schemas and function-level middleware. Unknown documentation
routes return a docs404 page in Go; Node's renderer currently falls back to its
index. Preserve this existing difference unless deliberately reviewed.

renderDocsIndex/buildSidebar/renderFunctionDocs/renderMiddlewareDoc use function
and middleware metadata to build descriptions, schema trees, required-header
information and placeholder request/cURL examples. DocsPath and RpcPath flow into
navigation and examples; custom paths must not become hard-coded /docs links.
Descriptions and schemas are documentation, not invocation or authorization.

## Middleware metadata projection

MiddlewareMetadata.Options/Headers are native SchemaRule trees. The renderer
recursively copies a safe structural projection using middlewareSchemaForDocs,
excluding value/default annotations and preserving types/descriptions/constraints/
requiredness. Configured per-function option maps are not read/rendered as values.
Projection must leave original schema objects and application configuration intact.
Required headers follow the schema's required list and optional rules; cURL uses
placeholders, not actual credentials. Nested options need recursive structure.

No middleware/handler executes when rendering docs. Adding metadata must not add
runtime middleware validation or defaults. Function input/output rendering is
separate from the stricter middleware projection. Keep secrets out of public
names/descriptions/constraints; no heuristic redaction can classify all prose.

## Global controls and retained metadata

Core Config.DocsEnabled/DocsPath read no environment automatically. The generated
main honors exported DOCS_ENABLED/DOCS_PATH with the documented production default
and boolean validation; it does not automatically load.env. Raw programmatic zero
DocsEnabled is false. Node generated/raw-core controls differ; preserve actual
APIs rather than inventing a common visibility flag.

FunctionMetadata.IsPublic bool and Node isPublic remain deprecated/ignored. True,
false and omitted values do not affect docs visibility, registration or execution.
There are no PUBLIC/PRIVATE badges or per-function HideFromDocs behavior.

## Test before changing a view

Use [pkg/rpc/docs_parity_test.go](https://github.com/CellsWeb/crpc-go/blob/main/pkg/rpc/docs_parity_test.go) for safe projection, ignored metadata,
required headers and function middleware. Use
[pkg/rpc/docs_parity_http_test.go](https://github.com/CellsWeb/crpc-go/blob/main/pkg/rpc/docs_parity_http_test.go) for actual fasthttp/net-http content,
custom paths, docs disabled and execution independence. docs_test.go covers the
complete renderer. Compare Node HTML/schema semantics without requiring identical
styling or overriding intentional unknown-route behavior. The
[Node renderer guide](https://cellsweb.github.io/crpc-node/internals/renderer/)
links its Fetch/native path routing tests.
