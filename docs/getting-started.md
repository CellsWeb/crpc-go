---
layout: default
title: Getting Started
nav_order: 2
---

# Getting Started

This guide walks you through installing CellsWeb RPC Go, scaffolding a new project, and running your first RPC endpoint.

---

## 🛠️ Prerequisites

- **Go**: Version `1.22` or higher ([Download Go](https://go.dev/dl/))
- **Operating System**: Linux, macOS, or Windows
- **Git**

---

## 📦 Installation

Add the core Go module to your project:

```bash
go get github.com/CellsWeb/crpc-go@latest
```

### Install the CLI Tool (`crpc-go`)

The `crpc-go` CLI scaffolds production-ready project templates, generates functions, and scaffolds middlewares:

```bash
go install github.com/CellsWeb/crpc-go/cmd/crpc-go@latest
```

Verify your installation:

```bash
crpc-go --version
```

---

## 🚀 Creating a New Project

To bootstrap a new CellsWeb RPC Go project:

```bash
crpc-go init my-rpc-api
cd my-rpc-api
```

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
