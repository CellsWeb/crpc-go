# ⚡ CellsWeb RPC Go (`crpc-go`) — Battle-Tested Benchmark Harness

This directory contains the public, reproducible benchmark harness, adversarial attack scripts, real-world CRUD and auth workloads, and multi-core Docker matrix for `crpc-go`.

---

## 🏗️ Architecture Under Test

- **Engine**: Fasthttp + Go 1.27 `encoding/json/v2`.
- **Endpoints**:
  - `benchmark.ping`: Zero-overhead baseline.
  - `auth.login`: Authentication flow with JWT generation and password verification.
  - `items.create`: CRUD insert with schema validation and in-memory datastore persistence.
  - `items.get`: Keyed lookup by ID.
  - `items.list`: Paginated fetch returning 20 item structures.
  - `benchmark.rateLimited`: Rate-limited endpoint (window: 60s, maxRequests: 100).

---

## 🚀 Quick Run

### 1. Run Server Locally
```bash
go run server/main.go
# Server listens on :4001
```

### 2. Run Comprehensive Load Test
```bash
./scripts/run_bench.sh 4001
```

### 3. Run Adversarial Attack & Fuzzing Suite
```bash
./scripts/attack_fuzzing.sh 4001
./scripts/attack_ratelimit.sh 4001
```

---

## 🐳 Docker Multi-Core Resource Matrix

To measure performance under strict resource isolation (1 CPU / 256MB RAM, 2 CPUs / 1GB RAM, 4 CPUs / 4GB RAM):

```bash
# Build the benchmark container
docker build -t crpc-go-bench .

# 1 CPU / 256MB RAM:
docker run --rm -d --name bench-1c --cpus="1.0" -m "256m" -p 4101:4001 crpc-go-bench
./scripts/run_bench.sh 4101
docker stop bench-1c

# 2 CPUs / 1GB RAM:
docker run --rm -d --name bench-2c --cpus="2.0" -m "1g" -p 4102:4001 crpc-go-bench
./scripts/run_bench.sh 4102
docker stop bench-2c

# 4 CPUs / 4GB RAM:
docker run --rm -d --name bench-4c --cpus="4.0" -m "4g" -p 4103:4001 crpc-go-bench
./scripts/run_bench.sh 4103
docker stop bench-4c
```

For detailed side-by-side benchmark numbers and comparison against Node.js / Bun, see [RESULTS.md](./RESULTS.md).
