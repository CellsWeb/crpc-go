<p align="center">
  <a href="https://cellsweb.com" target="_blank" rel="noopener noreferrer">
    <img src="https://cellsweb.com/logo.svg" alt="CellsWeb Logo" width="340" style="background: rgba(255, 255, 255, 0.96); padding: 16px 24px; border-radius: 16px; box-shadow: 0 8px 32px rgba(0, 0, 0, 0.08); display: inline-block; max-width: 100%;" />
  </a>
</p>

# CellsWeb RPC Go

## Framework overview

| Capability | Go implementation |
|---|---|
| Registration | Generic typed function registration or explicit FunctionDefinition |
| Middleware | Ordered handlers with options and context Set/Get; startup-only registration |
| Schemas | Native schema builders/struct metadata, HTTP input/output checks |
| HTTP | fasthttp server and net/http adapter, optional custom routes |
| Uploads | Multipart routing, bounds, temporary-file ownership and typed file injection |
| IPC | Unix newline/length framing, bounded stdio and maintained Node client |
| Documentation | Global enable/path controls, function and middleware schema views |
| Tooling | Go CLI initialization/generators, tests, standalone SDK resolution |

[Node/Bun engine](https://github.com/CellsWeb/crpc-node),
[HTTP client/React SDK](https://www.npmjs.com/package/@cellsweb/rpc-client) and
[Go/Node IPC example](https://github.com/CellsWeb/crpc-go/tree/main/example-node) are distinct integration paths. A browser
uses HTTP, while a local Node daemon client can use Unix IPC. HTTP File/Blob
uploads do not become live file objects over JSON IPC.

Typed native functions and schema contracts behind one RPC endpoint, with
middleware, multipart handling, Unix IPC and server-rendered documentation.

Version v1.0.2 adds bounded 2 MiB stdio framing to the hardening fixes; see
[installation](getting-started.md). Go >=1.27.4 is required. Node 24 LTS is the primary sibling/client test
runtime; Bun is tested independently in the Node framework.

- [Functions](functions.md), [schemas](schemas.md), [middleware](middlewares.md)
- [IPC](node-ipc.md), [in-memory testing](testing.md), [custom routes](custom-routes.md)
- [Docs controls](interactive-docs.md), [API](api-reference.md), [deployment](deployment.md)
- [Protocol](architecture.md), [benchmark context](benchmarks.md)

IsPublic is deprecated and ignored. All authentication/authorization is enforced
by application middleware, not documentation metadata or caller-injected context.

## For contributors

Start with [contribution policy](contributing.md), [development setup](development.md)
and the [internal source map](internals/index.md). The sidebar separates end-user
usage from implementation, testing and release guides. See the [security policy](security.md)
and [Code of Conduct](code-of-conduct.md) before opening a report.

## Start an application

```sh
go install github.com/CellsWeb/crpc-go/cmd/crpc-go@v1.0.2
crpc-go init my-api
cd my-api
go mod tidy
go build ./...
go test ./...
crpc-go dev
```

The versioned CLI selects SDKv1.0.2. Register typed functions and middleware
before serving; use native schemas for input/output contracts. For direct server
setup, see the [repository README](https://github.com/CellsWeb/crpc-go#executable-typed-server).
Review [middleware](middlewares.md), [schemas](schemas.md),
[HTTP routes](custom-routes.md) and [IPC](node-ipc.md) for application integration.

Go uses native metadata rather than TypeBox. Middleware header/options schemas
describe documentation; middleware remains responsible for validating them.
fasthttp buffers transport intake; per-request bounds are not an aggregate-memory
guarantee. See [lifecycle](internals/lifecycle.md) and
[source architecture](internals/index.md) for implementation details.
