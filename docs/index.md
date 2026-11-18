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
