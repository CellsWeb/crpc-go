---
layout: default
title: Getting Started
nav_order: 2
---

# Getting Started

This guide walks you through installing CellsWeb RPC Go, scaffolding a new project, and running your first RPC endpoint.

---

## 🛠️ Prerequisites

- **Go**: Version `1.27.4` or higher ([Download Go](https://go.dev/dl/))
- **Operating System**: Linux, macOS, or Windows
- **Git**

---

## 🛠️ Step 1: Install the CLI Tool (`crpc-go`)

Install the global CLI scaffolder anywhere from your terminal:

```bash
go install github.com/CellsWeb/crpc-go/cmd/crpc-go@latest
```

Verify your installation:

```bash
crpc-go --version
```

---

## 🚀 Step 2: Creating a New Project

Use `crpc-go` to bootstrap a complete project with pre-configured directory layout and `go.mod`:

```bash
crpc-go init my-rpc-api
cd my-rpc-api
go mod tidy
go build ./...
```

### SDK dependency and unreleased development

Generated applications require `github.com/CellsWeb/crpc-go` as an ordinary
versioned module. They do not contain a local `replace` directive or require a
parent/sibling SDK checkout. A CLI installed with `go install ...@<version>`
uses that installed module version for the generated SDK requirement.

An unpublished source build cannot assume its Git commit is remotely available.
It uses the existing published `v1.0.0` fallback, which `init` prints explicitly.
This older release does **not** contain unreleased hardening fixes. No future tag
is assumed or created. To test a candidate CLI and SDK checkout together, opt in
to a local Go workspace after generating the application:

```bash
cd /path/to/my-rpc-api
go work init . /absolute/path/to/rpc-go
go build ./...
go run ./cmd/server
```

Keep this machine-specific workspace local. `GOWORK=off go build ./...` uses the
application's declared released dependency instead. Once the reviewed SDK and
CLI are published together, install that exact CLI version and generate against
the matching SDK version; no workspace is needed. The generated Go directive
matches the current SDK minimum, `1.27.4`.

> **Adding to an Existing Project**:
> If adding to an existing Go project that already has a `go.mod`, install the library inside your project folder:
> ```bash
> go get github.com/CellsWeb/crpc-go@latest
> ```

### Generated Project Layout

The CLI generates a clean, scalable directory structure:

```text
my-rpc-api/
├── cmd/
│   ├── server/
│   │   └── main.go          # Application entrypoint & HTTP server bootstrap
│   └── codegen/
│       └── main.go          # TypeScript declaration exporter
├── functions/               # RPC Functions grouped by namespace
│   └── user/
│       └── getProfile/
│           ├── contracts.go   # Types, Schemas & Metadata
│           ├── middlewares.go # Middleware attachments
│           └── run.go         # Handler execution logic & init() hook
├── middlewares/             # Reusable middleware modules
│   └── auth/
│       ├── constants.go     # Options & Headers schemas
│       └── run.go           # Middleware execution logic
├── utils/                   # Helpers, storage engines, database drivers
├── go.mod
├── Makefile
└── .env.example
```

---

## ⚡ Scaffolding Functions & Middlewares

### Create a New Function

```bash
crpc-go create:function --ns user --name updateEmail
```

For multipart file upload endpoints (e.g. image or document uploads):

```bash
crpc-go create:function --ns user --name uploadAvatar --multipart
```

### Create a Reusable Middleware

```bash
crpc-go create:middleware --name rateLimit --desc "Per-IP rate limiter"
```

---

## 🏃 Running the Application

Generated servers read exported process environment variables. Neither the
executable nor `crpc-go dev` automatically loads `.env`. To use the generated,
trusted shell-compatible file in a POSIX shell, export it before starting:

```bash
set -a
. ./.env
set +a
go run ./cmd/server
```

`DOCS_ENABLED=true` enables documentation and `false` disables it. Values are
trimmed and case-insensitive; `1`/`0` and Go's `t`/`f` aliases also work. An unset
or blank value preserves the existing default: enabled unless `NODE_ENV` is
exactly `production`. An invalid nonblank value stops startup with a configuration
error, including in production. `DOCS_PATH` selects the absolute docs endpoint,
for example `/internal/docs`; an unset or blank value defaults to `/docs`.

The executable uses the final exported values. A sourced file replaces earlier
shell assignments; exports after sourcing replace the file's values. A `.env`
file that has not been exported has no effect. Existing programmatic
`rpc.Config.DocsEnabled` / `DocsPath` behavior is unchanged. The legacy `IsPublic`
field is deprecated and ignored; it controls neither documentation nor authorization.

Start the development server with live reload:

```bash
go run ./cmd/server
```

You will see the startup banner:

```text
⚡ CellsWeb RPC Go Engine running at http://0.0.0.0:4001/rpc
📖 Interactive Docs available at http://0.0.0.0:4001/docs
🔌 Node.js IPC Socket listening at /tmp/crpc.sock
```

---

## 🧪 Testing Your First RPC Call

All RPC calls are dispatched via HTTP `POST /rpc`:

```bash
curl -X POST http://localhost:4001/rpc \
  -H "Content-Type: application/json" \
  -d '{
    "namespace": "public",
    "functionName": "hello",
    "data": {
      "name": "Alex"
    }
  }'
```

### Success Response (HTTP 200)

```json
{
  "status": "ok",
  "data": {
    "greeting": "Hello, Alex! Welcome to CellsWeb RPC Go."
  },
  "meta": {
    "requestId": "req_c7f8e3a1",
    "durationMs": 0.42
  }
}
```

### Interactive Documentation

Open your browser and navigate to:

👉 **`http://localhost:4001/docs`**

You will see the interactive dark-themed API portal, complete with 2-column input/output schema views, middleware controls, and copyable cURL commands!
