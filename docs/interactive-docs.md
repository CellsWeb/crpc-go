---
layout: default
title: Interactive Documentation Portal
nav_order: 9
---

# Interactive Documentation Portal

CellsWeb RPC Go features a built-in, server-rendered API documentation UI available at `/docs`. It requires **zero third-party UI dependencies**.

---

## 🎨 UI Features

- **Dark Theme by Default**: Clean, high-contrast dark aesthetic matching modern developer tools.
- **2-Column Split Schema View**:
  - **Left Side**: Human-readable recursive field tree (`• user (object) -> • id (string, required)`). Binary file uploads display cleanly as `File / Binary`.
  - **Right Side**: Formatted JSON Schema code block with a one-click `Copy` button.
- **Dynamic Middleware Cards**:
  - Uses existing `rpc.MiddlewareMetadata` and native `schema.SchemaRule` schemas.
  - **Properties & Controls**: Lists schema-defined option names, types, descriptions, nested fields and required/optional status. Configured values and schema defaults are never displayed.
  - **Required Headers**: Distinguishes required and optional headers. Only required headers appear in function header tables and JSON/multipart cURL placeholders.
- **Ready-to-Run cURL Snippets**: Auto-generates fully-formed cURL commands populated with mock payload data.
- **Instant Client Search**: Fast search filter across all namespaces, functions, tags, and descriptions.

---

## ⚙️ Enabling & Configuring Docs

Configure the interactive docs when initializing `CoreRPC`:

```go
app, err := rpc.New(rpc.Config{
	RpcPath:     "/rpc",
	DocsEnabled: os.Getenv("NODE_ENV") != "production", // Enable in dev / staging
	DocsPath:    "/docs",                              // Documentation portal URL
	CORS: &rpc.CorsConfig{
		AllowOrigins: []string{"*"},
	},
})
```

---

## 🧭 Navigating the Portal

1. **Dashboard (`/docs`)**:
   - Displays all registered functions grouped by namespace.
   - Shows HTTP content types (`[JSON]` vs `[FORM]`).
   - Links to the central middleware catalog.

2. **Function Detail View (`/docs/:namespace/:functionName`)**:
   - Deep inspection of a single RPC function.
   - 2-column input and output schema breakdowns.
   - All attached middleware names, descriptions and schema contracts; no resolved runtime option values.
   - Ready-to-copy cURL command.

3. **Middleware Catalog (`/docs/middlewares/:name`)**:
   - Central catalog documenting all available middlewares.
   - Detailed header and configuration schemas.


## Public documentation and private configuration

The UI never reads per-function middleware option values, including middleware
without schemas. Middleware schema defaults are omitted recursively from the
public JSON Schema projection. Types, descriptions, structural constraints and
required fields remain visible; function input/output contracts keep their
existing rendering. Keep secrets out of names, descriptions and other public
schema metadata: the renderer cannot classify secrets deliberately placed in
public prose or constraints.

Provide documentation using the existing registration API:

```go
rpc.RegisterMiddleware("auth", "Checks bearer credentials", Run,
    schema.Object(map[string]*schema.SchemaRule{
        "roles": schema.Optional(schema.Array(schema.String())).Describe("Allowed roles"),
    }),
    schema.Object(map[string]*schema.SchemaRule{
        "authorization": schema.String().Describe("Bearer credentials"),
        "x-trace": schema.Optional(schema.String()).Describe("Optional trace identifier"),
    }),
)
```

This uses the existing schema abstractions; the documentation changes do not
validate middleware options or headers at execution time, apply defaults, execute
middleware, or change application handlers. Authentication and authorization
remain the middleware's responsibility. Configured application values and the
original schema objects remain unchanged.

Docs remain globally controlled by `rpc.Config.DocsEnabled` and `DocsPath`.
Generated servers honor `DOCS_ENABLED` and `DOCS_PATH` with their existing exported
environment behavior; see [Getting Started](getting-started.md). Disabling docs
blocks their pages without changing registration or RPC execution.

## Deprecated metadata compatibility

`rpc.FunctionMetadata.IsPublic bool` is **deprecated and ignored**. It controls
neither documentation visibility nor authentication/authorization. Its type,
field order and JSON tag are retained so existing struct literals, field reads
and serialization keep working. Omitting it uses the existing zero value;
false and true have identical framework behavior. PUBLIC/PRIVATE badges have
been removed, and generators/examples no longer emit the field. No replacement
visibility flag is introduced. Removal is reserved for a future major release.
Node's optional `FunctionMetadata.isPublic` follows the same policy.
