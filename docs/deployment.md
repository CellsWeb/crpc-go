# Deployment

Deploy the versioned **v1.0.2** SDK with dependencies pinned in go.mod/go.sum.
Use the matching versioned CLI for scaffolding. See
[installation](getting-started.md) and retain the deployment checks below.

```sh
go build -o dist/server ./cmd/server
NODE_ENV=production DOCS_ENABLED=false ./dist/server
```

Those env controls apply to the generated main. Programmatic rpc.Config reads no
env automatically. Neither the binary nor crpc-go dev loads.env. Export trusted
values explicitly. The generated listener is currently 0.0.0.0:4000 with /rpc;
PORT/HOST/RPC_PATH in the env example are not consumed. Configure rpc.Config for
another address/path, and match container/service/load-balancer ports to it.

Set an explicit CORS policy and application authentication/authorization. Configure
request/file/time limits and proxy trust for the deployment. Disable docs globally
when needed; metadata flags do not enforce security. Health/custom routes need
their own exposure/auth policy. Inspect RPC envelope status in monitoring when
using the default HTTP 200 convention; opt-in status mapping is separate.

Go >=1.27.4 is required. A Linux static build can use CGO_ENABLED=0 with the chosen
GOOS/GOARCH; cross-compilation is not a runtime/container test. No untested Docker,
cloud autoscaling, latency, resource-bound or deployment guarantee is made here.
Use production workload/aggregate-memory measurements before choosing concurrency.
