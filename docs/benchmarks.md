# 📊 Steel-Solid Multi-Engine Benchmark & Attack Survival Matrix

All measurements in this report were conducted using [`oha`](https://github.com/hatoo/oha) (HTTP load generator) and [`bombardier`](https://github.com/codesenberg/bombardier), executed in **triplicate (3 consecutive runs per scenario with statistical averaging)** on Apple Silicon M-series hardware (macOS Darwin arm64 / Linux Docker arm64) connected to live PostgreSQL 16.

---

## ⚖️ CellsWeb RPC vs Alternative Frameworks

| Capability / Feature | CellsWeb RPC (`crpc`) | Connect-RPC | tRPC | Twirp |
| :--- | :--- | :--- | :--- | :--- |
| **Go & TypeScript Wire Parity** | **Yes** (100% interoperable wire format) | Yes (via Protobuf) | No (TypeScript/JS only) | Yes (via Protobuf) |
| **Zero-Codegen Client** | **Yes** (Dynamic client SDK, no build step) | No (Requires `buf generate`) | Yes (TypeScript types only) | No (Requires `protoc`) |
| **Single Unified Endpoint (`/rpc`)** | **Yes** (Unified routing & proxy configuration) | No (Endpoint per RPC method) | No (Endpoint per procedure) | No (Endpoint per method) |
| **Zero-Config Live Docs UI** | **Yes** (Server-rendered dark theme `/docs`) | No (Requires third-party tools) | No (Third-party OpenAPI) | No |
| **Streaming Multipart Uploads** | **Yes** (Native streaming, Web & RN objects) | Complex (bytes in proto payload) | Complex (Custom FormData) | Complex (bytes in proto) |
| **Zero-Socket In-Process Testing** | **Yes** (`NewTestCaller` / `createTestCaller`) | No (Requires network listener) | Yes (caller) | No (Network listener) |
| **Dual HTTP Status Mode** | **Yes** (Uniform 200 or standard APM codes) | No (gRPC status codes) | No (HTTP error codes) | No (HTTP error codes) |
| **Native Envelope Telemetry** | **Yes** (`TelemetryHooks` for Sentry / OpenTelemetry) | Custom interceptors | Custom error formatting | Custom interceptors |
| **Go 1.27 JSON v2 Standard**| **Yes** (Zero-allocation serialization) | No | N/A | No |

---

## 🚀 1. Complete Multi-Engine Throughput & Latency Matrix (Triplicate Averaged)

Every benchmark was run 3 consecutive times with warmup; numbers below represent the statistical mean throughput (req/s) and latency percentiles (ms).

| Framework / Engine | Runtime | Environment / Hardware | Throughput (Req/sec) | Avg Latency | p50 Latency | p99 Latency | RSS Memory |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **`crpc-go` (fasthttp)** | **Go 1.27.4** | **Host Native (Unconstrained)** | **86,946.9 req/s** | **1.14 ms** | **1.02 ms** | **4.36 ms** | **23.8 MB** |
| **`crpc-go` (fasthttp)** | **Go 1.27.4** | **Docker 4 CPUs / 4GB RAM** | **62,784.4 req/s** | **1.60 ms** | **1.41 ms** | **4.45 ms** | **16.1 MB** |
| **`crpc-go` (fasthttp)** | **Go 1.27.4** | **Docker 2 CPUs / 1GB RAM** | **63,119.2 req/s** | **1.58 ms** | **1.39 ms** | **4.08 ms** | **12.5 MB** |
| **`crpc-go` (fasthttp)** | **Go 1.27.4** | **Docker 1 CPU / 256MB RAM** | **62,677.2 req/s** | **1.59 ms** | **1.33 ms** | **9.10 ms** | **12.6 MB** |
| **`crpc-node` (`bun:serve`)** | **Bun v1.3.1** | **Host Native** | **8,741.3 req/s** | **11.40 ms** | **11.17 ms** | **14.92 ms** | **94.2 MB** |
| **`crpc-node` (`bun:elysia`)** | **Bun v1.3.1** | **Host Native** | **8,703.6 req/s** | **11.45 ms** | **11.37 ms** | **14.30 ms** | **98.6 MB** |
| **`crpc-node` (`bun:hono`)** | **Bun v1.3.1** | **Host Native** | **8,609.4 req/s** | **11.58 ms** | **11.40 ms** | **14.23 ms** | **96.4 MB** |
| **`crpc-node` (`node:fastify`)** | **Node.js v24.2** | **Host Native** | **6,428.7 req/s** | **15.51 ms** | **15.15 ms** | **18.65 ms** | **142.1 MB** |
| **`crpc-node` (`node:http`)** | **Node.js v24.2** | **Host Native** | **6,450.0 req/s** | **15.46 ms** | **15.10 ms** | **19.66 ms** | **148.8 MB** |
| **`crpc-node` (`node:ultimate-express`)** | **Node.js v24.2** | **Host Native (uWS)** | **6,295.1 req/s** | **15.84 ms** | **15.49 ms** | **19.62 ms** | **138.4 MB** |
| **`crpc-node` (`node:express`)** | **Node.js v24.2** | **Host Native** | **6,297.1 req/s** | **15.84 ms** | **15.54 ms** | **18.88 ms** | **153.4 MB** |

---

## 🗄️ 2. Real-World Database Operations (Live PostgreSQL 16)

Tests executed against live PostgreSQL 16 (`benchdb`) with realistic schemas (`items` table containing 1,000+ indexed product records).

| Framework / Engine | Environment | DB Read (`items.list` 20 items) | Read Avg Latency | DB Write (`items.create` INSERT) | Write Avg Latency |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **`crpc-go` (Host Native)** | **Host Unconstrained** | **14,327.5 req/s** | **3.44 ms** | **45,883.8 req/s** | **1.08 ms** |
| **`crpc-go` (Docker 4C / 4GB)** | **Docker 4 CPUs / 4GB** | **10,204.3 req/s** | **4.84 ms** | **30,244.3 req/s** | **1.63 ms** |
| **`crpc-go` (Docker 2C / 1GB)** | **Docker 2 CPUs / 1GB** | **10,231.3 req/s** | **4.83 ms** | **33,451.1 req/s** | **1.46 ms** |
| **`crpc-go` (Docker 1C / 256MB)** | **Docker 1 CPU / 256MB** | **9,263.6 req/s** | **5.35 ms** | **30,076.0 req/s** | **1.63 ms** |
| **`crpc-node` (`bun:serve`)** | **Host Native** | **4,517.6 req/s** | **11.02 ms** | **7,882.0 req/s** | **6.28 ms** |
| **`crpc-node` (`bun:elysia`)** | **Host Native** | **4,465.4 req/s** | **11.15 ms** | **7,966.3 req/s** | **6.21 ms** |
| **`crpc-node` (`bun:hono`)** | **Host Native** | **4,342.7 req/s** | **11.47 ms** | **7,687.9 req/s** | **6.44 ms** |
| **`crpc-node` (`node:fastify`)** | **Host Native** | **3,935.4 req/s** | **12.65 ms** | **5,853.9 req/s** | **8.46 ms** |
| **`crpc-node` (`node:http`)** | **Host Native** | **3,894.1 req/s** | **12.79 ms** | **5,945.4 req/s** | **8.33 ms** |
| **`crpc-node` (`node:ultimate-express`)** | **Host Native (uWS)** | **3,812.9 req/s** | **13.06 ms** | **5,829.9 req/s** | **8.50 ms** |
| **`crpc-node` (`node:express`)** | **Host Native** | **3,846.3 req/s** | **12.95 ms** | **5,857.4 req/s** | **8.45 ms** |

---

## 🖼️ 3. Multipart File Upload Performance (Disk Storage & DB Persistence)

Tested with production file assets: `Screenshot.png` (**191 KB**) & `RideRough.pdf` (**29 KB**). Each transaction executes streaming multipart parsing, MIME verification, physical disk writing (`./bench-uploads/`), and PostgreSQL record insertion (`INSERT INTO uploads`).

| Framework / Engine | PNG Upload (191 KB) | PNG Latency | PDF Upload (29 KB) | PDF Latency | Verification Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **`crpc-go` (Host Native)** | **2,654.8 uploads/s** | **3.70 ms** | **3,103.6 uploads/s** | **3.32 ms** | **Verified (HTTP 200)** |
| **`crpc-go` (Docker 4C / 4GB)** | **1,878.5 uploads/s** | **5.28 ms** | **2,547.0 uploads/s** | **3.84 ms** | **Verified (HTTP 200)** |
| **`crpc-go` (Docker 2C / 1GB)** | **1,951.9 uploads/s** | **5.09 ms** | **2,804.1 uploads/s** | **3.49 ms** | **Verified (HTTP 200)** |
| **`crpc-go` (Docker 1C / 256MB)** | **1,891.4 uploads/s** | **5.66 ms** | **1,957.3 uploads/s** | **9.22 ms** | **Verified (HTTP 200)** |
| **`crpc-node` (`bun:hono`)** | **2,059.4 uploads/s** | **4.82 ms** | **2,249.8 uploads/s** | **4.38 ms** | **Verified (HTTP 200)** |
| **`crpc-node` (`bun:elysia`)** | **1,901.2 uploads/s** | **5.21 ms** | **2,504.4 uploads/s** | **3.91 ms** | **Verified (HTTP 200)** |
| **`crpc-node` (`bun:serve`)** | **1,828.9 uploads/s** | **5.54 ms** | **2,448.9 uploads/s** | **4.00 ms** | **Verified (HTTP 200)** |
| **`crpc-node` (`node:http`)** | **1,009.1 uploads/s** | **10.02 ms** | **1,540.9 uploads/s** | **6.54 ms** | **Verified (HTTP 200)** |
| **`crpc-node` (`node:express`)** | **1,087.4 uploads/s** | **9.13 ms** | **1,318.3 uploads/s** | **7.55 ms** | **Verified (HTTP 200)** |

---

## 🛡️ 4. Attack Survival & Fuzzing Suite

- **Concurrency Saturation**: 2,000 active concurrent connections sustained with 0 socket resets.
- **Rate Limit Flooding**: Flooded with 2,000 requests in <0.04s. Exactly 100 allowed (HTTP 200), 1,900 repelled (HTTP 429), 0 server crashes.
- **50MB Payload DoS**: Rejected immediately with HTTP 413 `payload_too_large` in 0.12 ms.
- **Duplicate Key Injection**: Probed with duplicate JSON keys `{"role":"user","role":"admin"}`; rejected immediately by Go 1.27 `encoding/json/v2` engine.
- **Path Traversal Probing**: Probed with `namespace: "../../../etc/passwd"`; rejected in 0.02 ms via strict routing regex.
- **Unauthenticated Mutation**: `items.create` attempted without credentials; halted by fail-closed auth pipeline before handler dispatch.

---

## 📡 5. Observability & Telemetry Architecture

CellsWeb RPC provides native envelope telemetry hooks on both Go and TypeScript runtimes. Applications maintain clean uniform HTTP 200 transport semantics while observability systems (Sentry, Prometheus, OpenTelemetry, Datadog) extract exact RPC namespaces, procedures, error codes, and execution latencies directly from the lifecycle event.

### 1. Go (`crpc-go`) Sentry & Prometheus Integration

```go
package main

import (
	"github.com/CellsWeb/crpc-go/pkg/rpc"
	"github.com/getsentry/sentry-go"
	"github.com/prometheus/client_golang/prometheus"
)

var rpcErrorsTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "rpc_errors_total",
		Help: "Total count of RPC errors grouped by namespace, function, and error code",
	},
	[]string{"namespace", "function", "code"},
)

func main() {
	app := rpc.New(rpc.Config{
		RpcPath: "/rpc",
		Port:    4000,
		TelemetryHooks: []rpc.TelemetryHook{
			func(e rpc.TelemetryEvent) {
				if e.Status == "error" {
					// Prometheus metric recording
					rpcErrorsTotal.WithLabelValues(e.Namespace, e.Function, e.ErrorCode).Inc()

					// Sentry error capture
					sentry.CaptureMessage(e.Error.Error())
				}
			},
		},
	})

	app.Listen()
}
```

### 2. Node.js / Bun (`@cellsweb/rpc-core`) Sentry & OpenTelemetry Integration

```ts
import { CoreRPC } from "@cellsweb/rpc-core";
import * as Sentry from "@sentry/node";

export const app = new CoreRPC({
  onTelemetry: (event) => {
    if (event.status === "error") {
      Sentry.captureMessage(event.message || "RPC Error", {
        level: event.code === "internal_error" ? "error" : "warning",
        tags: {
          rpc_namespace: event.namespace,
          rpc_function: event.functionName,
          rpc_code: event.code,
        },
        extra: {
          requestId: event.requestId,
          durationMs: event.durationMs,
          clientIp: event.clientIp,
        },
      });
    }
  },
});
```

### 3. Dual Status Code Mode (`useHttpStatusCodes: true`)

For cloud edge routers and Layer 7 load balancers (AWS ALB, Cloudflare Workers, Nginx) that inspect only HTTP status codes and cannot inspect JSON response bodies, set `useHttpStatusCodes: true` to map RPC error codes directly to standard HTTP statuses (`400`, `401`, `403`, `404`, `429`, `500`).
