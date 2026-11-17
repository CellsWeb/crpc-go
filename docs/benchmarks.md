# Benchmark context

Historical workloads/results live under the repository's benchmarks directory.
Retain commit, Go/runtime version, hardware, CPU/memory limits, payload shape,
concurrency and warmup when reproducing them. They are not a current latency,
throughput, zero-allocation or security guarantee. No new production benchmark
was run as part of release stabilization.

Account for transport-buffered fasthttp intake and runtime-buffered Node Fetch
formData when comparing memory. HTTP JSON v2 versus IPC JSON v1 conventions also
need workload-specific tests; see [architecture](architecture.md).
