# CellsWeb RPC Go

Native Go RPC functions, schemas, middleware, fasthttp/net-http adapters,
multipart intake, Unix IPC and server-rendered documentation.

## Release status

This branch prepares **v1.0.1**, including the verified hardening, documentation
parity, explicit HTTP null handling and independent scaffolding fixes. The public
**v1.0.0 tag predates those fixes**. Do not install or describe it as this candidate.
No tag or package was published by the stabilization work.

Use Go >=1.27.4. Build the candidate CLI from this checkout, then select the SDK
explicitly for local application testing:

```sh
go build -o /tmp/crpc-go-candidate ./cmd/crpc-go
/tmp/crpc-go-candidate init /tmp/my-crpc-candidate
cd /tmp/my-crpc-candidate
go work init . /absolute/path/to/rpc-go
go build ./...
go run ./cmd/server
```

Replace the checkout path. The source CLI prints its existing older v1.0.0
fallback; a workspace is how this unpublished SDK is selected intentionally.
Generated apps do not require a parent checkout or an implicit replacement.
After v1.0.1 is actually published, install that exact versioned CLI; it selects
SDK v1.0.1 without a workspace. See [installation](docs/getting-started.md).

## Contracts

HTTP POST /rpc accepts `{namespace,functionName,data}`. Objects, arrays, scalars
and explicit null retain shape through validation; omitted HTTP data defaults to
`{}`. Success/error envelopes and default HTTP 200 application-error behavior are
shared logically with Node. Native schema vocabularies/error details are not
identical. Standard HTTP mapping remains opt-in.

Middleware owns authentication and authorization for HTTP, in-memory and IPC.
Remote callers cannot bypass it or establish identity by injecting context.
Generated middleware is a customization stub, not an identity provider.
Metadata IsPublic is deprecated and ignored; it controls neither docs nor auth.
Docs are globally configured. Their middleware schema views never render live
configured option values/defaults.

## Guides and validation

- [Functions](docs/functions.md), [schemas](docs/schemas.md), [middleware](docs/middlewares.md)
- [Unix IPC and Node client](docs/node-ipc.md), [testing](docs/testing.md)
- [Custom routes](docs/custom-routes.md), [global docs](docs/interactive-docs.md)
- [Protocol](docs/architecture.md), [API reference](docs/api-reference.md)
- [Deployment](docs/deployment.md), [candidate migration](CHANGELOG.md)

```sh
go build ./...
CRPC_STANDALONE_INTEGRATION=1 go test -count=1 ./...
CRPC_STANDALONE_INTEGRATION=1 CRPC_SCAFFOLD_RACE=1 go test -race -count=1 ./...
go vet ./...
```

The Node example has a separate npm build/typecheck/IPC suite. The coordinated
release plan lives in the sibling Node repository's RELEASE.md. No zero-memory,
universal latency or security certification is implied by benchmark fixtures.
