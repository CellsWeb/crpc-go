# CellsWeb RPC Go

Native Go RPC functions, schemas, middleware, fasthttp/net-http adapters,
multipart intake, Unix IPC and server-rendered documentation.

## Install the public release

**github.com/CellsWeb/crpc-go@v1.0.1** includes the hardening, documentation
parity, explicit HTTP null and standalone scaffold fixes. Use Go>=1.27.4:

```sh
go install github.com/CellsWeb/crpc-go/cmd/crpc-go@v1.0.1
crpc-go version
crpc-go init my-api
cd my-api
go mod tidy
go build ./...
go test ./...
crpc-go dev
```

Put GOBIN (or `$(go env GOPATH)/bin`) on PATH. The versioned CLI generates a
public SDK v1.0.1 dependency without a workspace or local replacement. For an
existing module: `go get github.com/CellsWeb/crpc-go@v1.0.1`.

Source/VCS-built CLIs still intentionally select the older v1.0.0 SDK fallback;
use an explicit workspace when testing local SDK changes. The CLI display
version alone does not identify its generated dependency. See
[installation](docs/getting-started.md) and [migration](docs/migration.md).

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
- [Deployment](docs/deployment.md), [release and migration notes](CHANGELOG.md)

```sh
go build ./...
CRPC_STANDALONE_INTEGRATION=1 go test -count=1 ./...
CRPC_STANDALONE_INTEGRATION=1 CRPC_SCAFFOLD_RACE=1 go test -race -count=1 ./...
go vet ./...
```

The Node example has a separate npm build/typecheck/IPC suite. The coordinated
release plan lives in the sibling Node repository's RELEASE.md. No zero-memory,
universal latency or security certification is implied by benchmark fixtures.
