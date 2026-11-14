---
layout: default
title: Interactive Documentation Portal
nav_order: 9
---

# Interactive Documentation Portal

CellsWeb RPC Go features a built-in, server-rendered API documentation UI available at `/docs`. It requires **zero third-party UI dependencies** and renders in less than **1 millisecond**.

---

## 🎨 UI Features

- **Dark Theme by Default**: Clean, high-contrast dark aesthetic matching modern developer tools.
- **2-Column Split Schema View**:
  - **Left Side**: Human-readable recursive field tree (`• user (object) -> • id (string, required)`). Binary file uploads display cleanly as `File / Binary`.
  - **Right Side**: Formatted JSON Schema code block with a one-click `Copy` button.
- **Dynamic Middleware Cards**:
  - Dynamically inspects attached middlewares and TypeBox-compatible schemas.
  - **Properties & Controls**: Lists option keys, configured runtime values, schema defaults, and descriptions.
  - **Required Headers**: Highlights required/optional HTTP headers and tokens.
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
   - All attached middlewares with their resolved runtime options.
   - Ready-to-copy cURL command.

3. **Middleware Catalog (`/docs/middlewares/:name`)**:
   - Central catalog documenting all available middlewares.
   - Shows which functions currently attach this middleware.
   - Detailed header and configuration schemas.
