# CellsWeb RPC Go

Typed native functions and schema contracts behind one RPC endpoint, with
middleware, multipart handling, Unix IPC and server-rendered documentation.

Version v1.0.1 is publicly released with the hardening fixes; see
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
