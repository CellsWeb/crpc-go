# 📊 Benchmark & Security Audit Results

Comprehensive performance metrics, Docker multi-core resource scaling tests, Go 1.27 JSON v2 security enforcement, and side-by-side comparisons against Node.js / Bun engines.

All tests executed in triplicate (3 consecutive runs per scenario with statistical averaging) on Apple Silicon M-series (macOS Darwin arm64 / Linux Docker arm64) connected to live PostgreSQL 16.

---

## 🎯 1. Multi-Engine Throughput Comparison (50,000 Requests, 100 Concurrency)

Tested using `oha` with HTTP POST payload `{"namespace":"benchmark","functionName":"ping","data":{}}`:

| Framework / Engine | Runtime | Environment / Hardware | Ping Baseline (RPS) | Ping Avg (ms) | Ping p50 (ms) | Ping p99 (ms) | RSS Memory (MB) | Error Rate |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **`crpc-go` (fasthttp)** | **Go 1.27.4** | **Host Native (Unconstrained)** | **86,946.9** | **1.14** | **1.02** | **4.36** | **23.8 MB** | **0.00%** |
| **`crpc-go` (fasthttp)** | **Go 1.27.4** | **Docker 4 CPUs / 4GB RAM** | **62,784.4** | **1.60** | **1.41** | **4.45** | **16.1 MB** | **0.00%** |
| **`crpc-go` (fasthttp)** | **Go 1.27.4** | **Docker 2 CPUs / 1GB RAM** | **63,119.2** | **1.58** | **1.39** | **4.08** | **12.5 MB** | **0.00%** |
| **`crpc-go` (fasthttp)** | **Go 1.27.4** | **Docker 1 CPU / 256MB RAM** | **62,677.2** | **1.59** | **1.33** | **9.10** | **12.6 MB** | **0.00%** |
| **`crpc-node` (`bun:serve`)** | **Bun v1.3.1** | **Host Native** | **8,741.3** | **11.40** | **11.17** | **14.92** | **94.2 MB** | **0.00%** |
| **`crpc-node` (`bun:elysia`)** | **Bun v1.3.1** | **Host Native** | **8,703.6** | **11.45** | **11.37** | **14.30** | **98.6 MB** | **0.00%** |
| **`crpc-node` (`bun:hono`)** | **Bun v1.3.1** | **Host Native** | **8,609.4** | **11.58** | **11.40** | **14.23** | **96.4 MB** | **0.00%** |
| **`crpc-node` (`node:fastify`)** | **Node.js v24.2** | **Host Native** | **6,428.7** | **15.51** | **15.15** | **18.65** | **142.1 MB** | **0.00%** |
| **`crpc-node` (`node:http`)** | **Node.js v24.2** | **Host Native** | **6,450.0** | **15.46** | **15.10** | **19.66** | **148.8 MB** | **0.00%** |
| **`crpc-node` (`node:ultimate-express`)** | **Node.js v24.2** | **Host Native (uWS)** | **6,295.1** | **15.84** | **15.49** | **19.62** | **138.4 MB** | **0.00%** |
| **`crpc-node` (`node:express`)** | **Node.js v24.2** | **Host Native** | **6,297.1** | **15.84** | **15.54** | **18.88** | **153.4 MB** | **0.00%** |

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

## 🖼️ 3. Real Multipart File Upload Benchmarks

Tested with real production files: `Screenshot 2026-11-11 at 22.26.21.png` (191 KB) & `RideRough.pdf` (29 KB):

| Framework / Engine | PNG Upload (191 KB) | PNG Latency | PDF Upload (29 KB) | PDF Latency | Success Rate |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **`crpc-go` (Host Native)** | **2,654.8 uploads/s** | **3.70 ms** | **3,103.6 uploads/s** | **3.32 ms** | **100.0% (HTTP 200)** |
| **`crpc-go` (Docker 4C / 4GB)** | **1,878.5 uploads/s** | **5.28 ms** | **2,547.0 uploads/s** | **3.84 ms** | **100.0% (HTTP 200)** |
| **`crpc-go` (Docker 2C / 1GB)** | **1,951.9 uploads/s** | **5.09 ms** | **2,804.1 uploads/s** | **3.49 ms** | **100.0% (HTTP 200)** |
| **`crpc-go` (Docker 1C / 256MB)** | **1,891.4 uploads/s** | **5.66 ms** | **1,957.3 uploads/s** | **9.22 ms** | **100.0% (HTTP 200)** |
| **`crpc-node` (`bun:hono`)** | **2,059.4 uploads/s** | **4.82 ms** | **2,249.8 uploads/s** | **4.38 ms** | **100.0% (HTTP 200)** |
| **`crpc-node` (`bun:elysia`)** | **1,901.2 uploads/s** | **5.21 ms** | **2,504.4 uploads/s** | **3.91 ms** | **100.0% (HTTP 200)** |
| **`crpc-node` (`bun:serve`)** | **1,828.9 uploads/s** | **5.54 ms** | **2,448.9 uploads/s** | **4.00 ms** | **100.0% (HTTP 200)** |
| **`crpc-node` (`node:http`)** | **1,009.1 uploads/s** | **10.02 ms** | **1,540.9 uploads/s** | **6.54 ms** | **100.0% (HTTP 200)** |
| **`crpc-node` (`node:express`)** | **1,087.4 uploads/s** | **9.13 ms** | **1,318.3 uploads/s** | **7.55 ms** | **100.0% (HTTP 200)** |

*(Note: Fastify and Ultimate-Express rejected multipart streams with HTTP 415/500 without parsing or saving files. Among all engines completing full disk and DB persistence, Go is #1).*

---

## 🛡️ 4. Attack Survival & Fuzzing Suite

- **Concurrency Saturation**: 2,000 active concurrent connections sustained with 0 socket resets.
- **Rate Limit Flooding**: Flooded with 2,000 requests in <0.04s. Exactly 100 allowed (HTTP 200), 1,900 repelled (HTTP 429), 0 server crashes.
- **50MB Payload DoS**: Rejected immediately with HTTP 413 `payload_too_large` in 0.12 ms.
- **Duplicate Key Injection**: Probed with duplicate JSON keys `{"role":"user","role":"admin"}`; rejected immediately by Go 1.27 `encoding/json/v2` engine.
- **Path Traversal Probing**: Probed with `namespace: "../../../etc/passwd"`; rejected in 0.02 ms via strict routing regex.
- **Unauthenticated Mutation**: `items.create` attempted without credentials; halted by fail-closed auth pipeline before handler dispatch.
