# Contributing to CellsWeb RPC Go

Welcome. Small corrections, reproducible bug reports, tests and focused features
are useful contributions. Read the [Code of Conduct](https://github.com/CellsWeb/crpc-go/blob/main/CODE_OF_CONDUCT.md)
and [security policy](https://github.com/CellsWeb/crpc-go/blob/main/SECURITY.md).
Security-sensitive reports must be private. See the MIT LICENSE before including
third-party material.

## Go development setup

Use Go 1.27.4 or newer compatible tooling; Go 1.27.4 was validated. Node 24.15.0 and
npm are needed for example-node; that example's lockfile resolves TypeScript 5.9.3.
Unix IPC suites require a Unix-domain-socket environment (validated macOSarm64).
Node SDK work uses its own pnpm/TypeScript versions, not this example's lockfile.

```sh
git clone https://github.com/CellsWeb/crpc-go.git
cd crpc-go
go mod download
go build ./...
go test -count=1 ./...
go vet ./...
go test -race -count=1 ./...
cd example-node
npm ci
npm run typecheck
CRPC_IPC_TEST_RACE=1 npm test
```

Use a fork and a focused branch. Preserve unrelated local work, and use a separate
worktree for a pinned baseline. Race detection needs a working supported cgo/C
compiler toolchain; record unsupported platforms instead of claiming a pass.

## Go repository map and conventions

- pkg/rpc: CoreRPC, configuration, registration, Context, HTTP/multipart,
  in-memory callers, typed wrappers, errors and docs renderer.
- pkg/schema: native SchemaRule constructors, validation, multipart coercion and
  documentation export; this is not a complete TypeBox/JSON Schema interpreter.
- pkg/nodebridge: Unix/stdio bridge and TypeScript declaration generator.
- cmd/crpc-go: CLI dispatcher and init/dev/scaffold commands.
- pkg/utils: storage and image helpers. example, example-mongo and benchmarks have
  separate Go modules; root go test ./... does not include nested modules.
- example-node: maintained IPC client, typed contracts and actual-daemon tests.

Use gofmt, explicit errors and existing native schema abstractions. Preserve
Context ownership and cleanup. Register functions/middleware before serving;
do not assume live registry mutation is a supported concurrency model. Prefer
focused changes over pipeline/renderer refactoring in an unrelated fix.

## Create a Go function or middleware

Build the local CLI, generate a disposable app and select the local SDK deliberately:

```sh
go build -o /tmp/crpc-go-dev ./cmd/crpc-go
/tmp/crpc-go-dev init /tmp/crpc-contributor-app
cd /tmp/crpc-contributor-app
go work init . /absolute/path/to/crpc-go
/tmp/crpc-go-dev create:function --ns catalog --name get
/tmp/crpc-go-dev create:middleware --name tenantResolver --desc 'Resolve tenant'
go build ./...
```

Replace the absolute checkout path. Do not commit machine-specific workspaces.
Source/VCS-built CLIs intentionally select older SDK v1.0.0 without a workspace;
the installed public v1.0.1 CLI selects SDK v1.0.1. Check go list -m, not just the
CLI display. Generated Go functions require explicit imports/registration;
there is no Node-style filesystem discovery or --middlewares CLI flag.

## Test and regression workflow

First reproduce a bug at a recorded baseline, then add a failing regression.
Use table-driven shapes/invalid inputs and assert middleware/handler call order,
error code/details, HTTP status, resource bounds and cleanup. Important gates:

```sh
go test -count=1 ./pkg/rpc -run TestHTTPJSONPayloadShapes
go test -count=1 ./pkg/rpc -run TestCallerCallRaw
go test -count=1 ./pkg/nodebridge
CRPC_STANDALONE_INTEGRATION=1 go test -count=1 ./...
CRPC_STANDALONE_INTEGRATION=1 CRPC_SCAFFOLD_RACE=1 go test -race -count=1 ./...
```

The standalone flag resolves the actual public source-build fallback SDK and then
an explicit local workspace; it needs network access and an unused port 4000.
The race flag instruments generated child servers too. example-node has separate
npm build/typecheck/tests; CRPC_IPC_TEST_RACE=1 instruments its real daemon.
Nested examples need their own build/test commands and, for local SDK changes,
an explicit workspace. Do not claim database integration passed from a build.

## Changes involving both runtimes

Use semantic parity, not identical implementation. Read the sibling
[Node contributor guide](https://github.com/CellsWeb/crpc-node/blob/main/CONTRIBUTING.md).
Propose protocol/default/public-API changes before implementation; identify the
migration/version implications. Attach equivalent logical fixtures and actual
results for Node and Go. Go's checked-in pkg/rpc/testdata/http-json-shapes.json is
a useful fixture set, with HTTP tests in that package and Node union regressions
in rpc-core. There is no checked-in one-command cross-repository parity runner;
do not require a private review-kit path from external contributors.

Preserve these compatibility boundaries unless explicitly reviewed/versioned:

- One configurable RPC endpoint with {namespace,functionName,data}. Arbitrary
  JSON data shapes survive; omitted HTTP data defaults to {}, explicit null stays
  null for schema validation. Do not silently coerce ordinary JSON inputs.
- Successfully handled application errors use HTTP200 by default. Opt-in status
  mapping and transport/framework errors are distinct; not every malformed body
  automatically produces HTTP400. Preserve success/error envelopes and SDK parsing.
- Middleware owns authentication/authorization. Remote callers cannot skip it or
  inject trusted state. Missing configured middleware fails closed.
- Multipart routing headers come as a pair and agree with required body routing.
  Headerless fallback puts routing fields before files. No application-owned
  upload files before required authorization, and no cleanup/size-limit weakening.
- Client files are top-level fields/direct arrays; nested file objects require a
  deliberate versioned design. Native Node streams, Fetch form parsing and Go
  transport buffering have different memory boundaries; do not promise zero RAM.
- Node discovery and Go registration are intentionally different. isPublic/IsPublic
  remain deprecated/ignored compatibility fields; no replacement visibility flag.
- IPC request IDs/framing remain compatible; never replay a failed call implicitly.
  Unix framing limits do not apply automatically to stdio.

## Pull requests and review

Open an issue/discussion first for a substantial architectural/API change. Keep
one root cause per fix and include problem, before/after behavior, affected paths,
compatibility and limitations. Conventional-style subjects such as fix:, docs:
and test: help scanning; match existing style without rewriting unrelated files.

Use the PR template. Provide exact executed commands/outcomes, red/green evidence
for bugs and both-runtime results where applicable. Docs-only PRs should run both
strict MkDocs builds when shared content changes, validate links, render diagrams
and compile meaningful examples; unnecessary implementation-mirroring tests are
not required. Include screenshots for documentation UI changes. State untested
scenarios/skips honestly. Never commit secrets, generated upload files, dist,
node_modules, local workspaces or unrelated agent changes.

A maintainer reviews scope, protocol/API impact, tests, resource cleanup and docs
before merge. Do not merge/publish/tag as part of an ordinary contribution. Release
changes require separate approval and public-install verification. New public API
fields/options and defaults require explicit compatibility review.

The [contributor documentation](https://cellsweb.github.io/crpc-go/internals/)
contains source maps, request lifecycle, subsystem test maps and release guidance.
