---
layout: default
title: CellsWeb RPC Go Documentation
nav_order: 1
---

<p align="center">
  <a href="https://cellsweb.com" target="_blank" rel="noopener noreferrer">
    <img src="https://cellsweb.com/logo.svg" alt="CellsWeb Logo" width="340" style="background: rgba(255, 255, 255, 0.96); padding: 16px 24px; border-radius: 16px; box-shadow: 0 8px 32px rgba(0, 0, 0, 0.08); display: inline-block; max-width: 100%;" />
  </a>
</p>

<p align="center">
  <strong>High-Performance RPC API Framework for Go</strong><br>
  <a href="https://cellsweb.com">🌐 Official Website</a> •
  <a href="https://cellsweb.github.io/crpc-go/">📖 Go Engine Docs</a> •
  <a href="https://cellsweb.github.io/crpc-node/">📖 Node Engine Docs</a> •
  <a href="https://github.com/CellsWeb">🏛️ GitHub Organization</a>
</p>

# CellsWeb RPC (Go Engine)

> **High-performance, production-ready RPC API framework for Go, fasthttp, and TypeScript/Node.js ecosystems.**

[![Go Report Card](https://goreportcard.com/badge/github.com/CellsWeb/crpc-go)](https://goreportcard.com/report/github.com/CellsWeb/crpc-go)
[![GoDoc](https://godoc.org/github.com/CellsWeb/crpc-go?status.svg)](https://pkg.go.dev/github.com/CellsWeb/crpc-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://github.com/CellsWeb/crpc-go/blob/main/LICENSE)

---

## 🌟 Overview

**CellsWeb RPC Go (`crpc-go`)** is a wire-compatible, ultra-low-latency Go implementation of the CellsWeb RPC API architecture. Powered by **fasthttp**, it delivers **23,000+ requests per second** on a single CPU core with sub-millisecond response latency and an ultra-lean memory footprint (<15MB RAM).

It bridges the gap between Go microservices, modern frontends (Web, React Native), and Node.js/Next.js fullstack backends with:
- **Single-Endpoint Architecture**: All RPC calls dispatch through `POST /rpc` with uniform `{ namespace, functionName, data }` envelopes.
- **$O(1)$ Function Routing**: Pre-indexed hash map function discovery eliminates regex and dynamic URL parsing overhead.
- **Uniform HTTP 200 Error Protocol**: Business, validation, and authentication errors return HTTP 200 with standardized JSON envelopes.
- **Zero-Config Interactive Docs**: Server-rendered, dark-themed API documentation built directly into the engine at `/docs`.
- **Node.js Unix Domain Socket IPC**: Direct inter-process communication (<50µs latency) for Next.js Server Actions and Node backends.
- **Strict Compile-Time & Runtime Safety**: Automatic TypeBox-compatible JSON schema inference and strict field validation.

---

## ⚡ Performance Comparison

Benchmarked on Apple M-series hardware (1 CPU Core, concurrency = 100, 50,000 requests):

| Metric | Node.js (CoreRPC) | Bun (Native) | **CellsWeb RPC Go (`crpc-go`)** |
| :--- | :--- | :--- | :--- |
| **Throughput** | 7,420 req/sec | 14,280 req/sec | **23,485 req/sec** *(+216% vs Node)* |
| **P50 Latency** | 2.1 ms | 1.1 ms | **0.38 ms** *(5.5x faster)* |
| **P99 Latency** | 7.8 ms | 4.2 ms | **1.20 ms** *(6.5x faster)* |
| **Memory (Baseline)** | 64 MB | 48 MB | **14.8 MB** *(77% lower)* |
| **Memory (Peak Load)**| 182 MB | 124 MB | **24.1 MB** *(86% lower)* |

---

## ⚖️ CellsWeb RPC vs Connect-RPC vs tRPC vs Twirp

| Capability / Feature | CellsWeb RPC (`crpc`) | Connect-RPC | tRPC | Twirp |
| :--- | :--- | :--- | :--- | :--- |
| **Go & TypeScript Parity** | **Yes** (100% interoperable wire format) | Yes (via Protobuf) | No (TypeScript/JS only) | Yes (via Protobuf) |
| **Zero-Codegen Client** | **Yes** (Dynamic client SDK, no build step) | No (Requires `buf generate`) | Yes (TypeScript types only) | No (Requires `protoc`) |
| **Single Endpoint (`/rpc`)** | **Yes** (Unified routing & proxy configuration) | No (Endpoint per RPC method) | No (Endpoint per procedure) | No (Endpoint per method) |
| **Zero-Config Live Docs UI** | **Yes** (Server-rendered dark theme `/docs`) | No (Requires third-party tools) | No (Third-party OpenAPI) | No |
| **Multipart File Streaming** | **Yes** (Streaming intake, Web & RN objects) | Complex (bytes in proto payload) | Complex (Custom FormData) | Complex (bytes in proto) |
| **In-Process Testing Engine** | **Yes** (`NewTestCaller` / zero socket) | No (Requires network listener) | Yes (caller) | No (Network listener) |
| **Dual HTTP Status Mode** | **Yes** (Uniform 200 or standard APM codes) | No (gRPC status codes) | No (HTTP error codes) | No (HTTP error codes) |
| **Go 1.27 JSON v2 Standard**| **Yes** (Enforced, zero-allocation serialization) | No | N/A | No |

---

## 📚 Documentation Table of Contents

1. [Getting Started](./getting-started.md) — Installation, CLI scaffolder, and quickstart guide.
2. [Benchmarks & Attack Survival](./benchmarks.md) — Multi-engine performance matrices, Docker resource scaling, and attack results.
3. [Architecture & Lifecycle](./architecture.md) — fasthttp execution pipeline, envelopes, and dispatching.
4. [Writing Functions & Contracts](./functions.md) — Co-located `contracts.go`, `middlewares.go`, `run.go`, schemas, and file uploads.
5. [Middlewares & Context](./middlewares.md) — Header validation, options schemas, context namespacing, and fail-closed security.
6. [Node.js & Next.js IPC Integration](./node-ipc.md) — Unix domain sockets, TypeScript `.d.ts` generator, and Next.js Server Actions.
7. [Custom HTTP Routes & In-Process Calling](./custom-routes.md) — Mounting REST endpoints, webhooks, and `c.Call()` in-memory dispatch.
8. [In-Process Testing](./testing.md) — `rpc.NewTestCaller`, zero network ports, and security audit testing.
9. [Interactive Documentation UI](./interactive-docs.md) — Built-in dark documentation portal, schema split view, and cURL builder.
10. [API Reference](./api-reference.md) — Complete Go package and symbol reference.
11. [Production Deployment & Autoscaling](./deployment.md) — Docker multi-stage builds, Fly.io, and CPU/RAM autoscaler architecture.
