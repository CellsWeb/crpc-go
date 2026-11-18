<p align="center">
  <a href="https://cellsweb.com" target="_blank" rel="noopener noreferrer">
    <img src="https://cellsweb.com/logo.svg" alt="CellsWeb Logo" width="340" style="background: rgba(255, 255, 255, 0.96); padding: 16px 24px; border-radius: 16px; box-shadow: 0 8px 32px rgba(0, 0, 0, 0.08); display: inline-block; max-width: 100%;" />
  </a>
</p>

# CellsWeb RPC Go

Native Go RPC functions, schemas, middleware, fasthttp/net-http adapters,
multipart intake, Unix IPC and server-rendered documentation.

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
[Go/Node IPC example](example-node/) are distinct integration paths. A browser
uses HTTP, while a local Node daemon client can use Unix IPC. HTTP File/Blob
uploads do not become live file objects over JSON IPC.

## Install the versioned SDK and CLI

**github.com/CellsWeb/crpc-go@v1.0.2** includes the hardening, documentation
parity, explicit HTTP null and standalone scaffold fixes. Use Go>=1.27.4:

```sh
go install github.com/CellsWeb/crpc-go/cmd/crpc-go@v1.0.2
crpc-go version
crpc-go init my-api
cd my-api
go mod tidy
go build ./...
go test ./...
crpc-go dev
```

Put GOBIN (or `$(go env GOPATH)/bin`) on PATH. The versioned CLI generates a
matching SDK v1.0.2 dependency without a workspace or local replacement. For an
existing module: `go get github.com/CellsWeb/crpc-go@v1.0.2`.

Source/VCS-built CLIs still intentionally select the older v1.0.0 SDK fallback;
use an explicit workspace when testing local SDK changes. The CLI display
version alone does not identify its generated dependency. See
[installation](docs/getting-started.md) and [migration](docs/migration.md).

## Executable typed server

In a fresh Go module, `go get github.com/CellsWeb/crpc-go@v1.0.2`, then save this
as `main.go`. The example registers before serving and uses explicit CORS/config.
The documentation decision below is application configuration, not an implicit
Go core environment parser.

<!-- verify-go: root-go/main.go -->
```go
package main

import (
    "log"
    "os"
    "github.com/CellsWeb/crpc-go/pkg/rpc"
    "github.com/CellsWeb/crpc-go/pkg/schema"
)

type Input struct { Name string `json:"name"` }
type Output struct { Greeting string `json:"greeting"` }

func main() {
    rpc.Register(rpc.FunctionConfig[Input, Output]{
        Namespace: "public", FunctionName: "hello",
        Metadata: rpc.FunctionMetadata{Description: "Greet a named caller"},
        InputSchema: schema.Object(map[string]*schema.SchemaRule{
            "name": schema.String().Min(1),
        }),
        OutputSchema: schema.FromStruct(Output{}),
        Handler: func(_ *rpc.Context, input Input) (*Output, error) {
            return &Output{Greeting: "Hello, " + input.Name + "!"}, nil
        },
    })
    app, err := rpc.New(rpc.Config{
        RpcPath: "/rpc", Host: "127.0.0.1", Port: 4000,
        CORS: &rpc.CorsConfig{AllowOrigins: []string{"*"}},
        DocsEnabled: os.Getenv("NODE_ENV") != "production", DocsPath: "/docs",
    })
    if err != nil { log.Fatal(err) }
    app.UseGlobalRegistry()
    if err := app.ListenAndServe(); err != nil { log.Fatal(err) }
}
```

Run `go mod tidy`, `go run .`, then:

```sh
curl http://localhost:4000/rpc -H 'Content-Type: application/json' \
  --data '{"namespace":"public","functionName":"hello","data":{"name":"Ada"}}'
```

Success contains `{"status":"ok","data":{"greeting":"Hello, Ada!"}}`; a numeric
name produces the existing structured validation error with default HTTP 200.
Use `schema.Optional`, arrays and native builders for precise nullable/scalar
contracts; Go is not an implementation of every TypeBox/JSON Schema keyword.

## Middleware and application context

Register middleware through `rpc.RegisterMiddleware` or an instance's
RegisterMiddleware, then attach it with
`rpc.DefineMiddlewares(rpc.M("auth", options))` in FunctionConfig.Middlewares.
A middleware receives `(*rpc.Context, headers, options)` and returns an error to
stop execution. Verify credentials before calling ctx.Set; handlers can retrieve
that server-owned state with ctx.Get or rpc.GetContext.

Go middleware options/header schemas are documentation metadata, not automatic
runtime middleware validation. Validate credentials and configuration in the
middleware itself. Unlike remote HTTP/IPC input, local test setup is trusted
application code. There is no supported remote skip/context-injection auth path.
Register functions, middleware and custom routes before active serving.

## Multipart, documentation and IPC

Use `schema.File()` or file arrays and `*rpc.UploadedFile` fields in typed input.
Its API uses methods: Filename(), Size(), MimeType(), Bytes(), Reader(),
TempPath(), SaveTo(path). Persist to a safe application-owned location during
processing; framework temporary files are cleaned afterward. Files are supported
at top level/direct arrays. Routing headers must agree with redundant body
fields; headerless fallback is supported. fasthttp buffers transport intake
before application multipart processing, so this is not zero network buffering.

Raw `rpc.Config` controls DocsEnabled/DocsPath explicitly. The generated server
uses exported DOCS_ENABLED (trimmed/case-insensitive true/false/1/0), DOCS_PATH and
NODE_ENV; omitted visibility defaults on outside production/off in production.
Invalid nonblank booleans fail startup; generated Go servers do **not** load .env.
Disabling docs blocks function/middleware/detail routes without unregistering RPCs.
Metadata schemas/descriptions should never contain application secrets.

For a Node process, use the maintained example-node client and its matching
contract declarations. Go exposes `nodebridge.ServeUnixSocket` and `ServeStdio`.
Unix supports JSON-Lines or a four-byte big-endian length prefix; stdio is
newline-only. The maximum is2,097,152 bytes, counting newline delimiters for line
frames. Socket-close, anonymous connection errors and malformed response JSON
reject pending Node calls promptly; later calls can reconnect without replay.
See [IPC guide](docs/node-ipc.md) for framing and daemon lifecycle details.

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

## Contribute

Read [CONTRIBUTING.md](CONTRIBUTING.md) for setup, focused changes and review.
The [development guide](docs/development.md) and
[source map](docs/internals/index.md) explain internals and subsystem tests.
Please follow [the Code of Conduct](CODE_OF_CONDUCT.md); report vulnerabilities
through [the private reporting policy](SECURITY.md). Licensed under
[MIT](LICENSE).
