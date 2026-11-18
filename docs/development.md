# Go contributor setup

Read [CONTRIBUTING](contributing.md) for review requirements. This repository is a
Go module; its example applications and Node IPC client have separate toolchains.

| Tool | Validated version / role |
|---|---|
| Go | 1.27.4, matching go.mod minimum; build/test/vet/race |
| Node | 24.15.0 for example-node; the sibling Node framework tests24.0.0 too |
| npm / example TypeScript | npm 11.12.1; example-node lock resolves TS5.9.3 |
| Documentation | Python 3.14.7; MkDocs 1.6.1, Material 9.8.1, PyMdown 13.1 |

```sh
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

Full Unix IPC tests need a Unix socket environment. Race detection needs a working
supported cgo/C toolchain. The release matrix was tested on macOSarm64; do not
turn cross-compilation or a skipped suite into a runtime claim.

## Source CLI versus installed CLI

The versioned v1.0.2 CLI generates SDK v1.0.2. A local/VCS-built binary still
uses its real older v1.0.0 SDK fallback, independent of display version. For local
SDK development, generate an app with the source binary, then explicitly select
this checkout with `go work init . /absolute/path/to/crpc-go`. Never commit that
machine-specific workspace or edit go.mod to pretend an unpublished tag exists.

Register imported function/middleware packages and call UseGlobalRegistry before
serving. Package init() only runs for imported packages. Changing a directory on
disk does not cause Go discovery or restart an existing binary.

## Modules and optional integration

Root `go test ./...` does not traverse nested go.mod modules. example,
example-mongo and benchmarks need their own commands; a successful build is not
proof of a live database or benchmark result. Keep external-service tests scoped.

The explicitly enabled scaffold integration uses a real public dependency and
then a local workspace, network access and port 4000. CRPC_SCAFFOLD_RACE=1 also
instruments generated child servers. example-node's CRPC_IPC_TEST_RACE=1 similarly
instruments its actual daemon. See the [testing map](internals/testing.md).

## Build the documentation

```sh
python3 -m venv /tmp/crpc-docs-venv
/tmp/crpc-docs-venv/bin/python -m pip install mkdocs-material
/tmp/crpc-docs-venv/bin/mkdocs serve
/tmp/crpc-docs-venv/bin/mkdocs build --strict --site-dir /tmp/crpc-go-site
```

The existing deployment workflow installs Material; it is preserved. The versions
above describe this validation, not a new Python dependency lock or CI job.
Render Mermaid diagrams in a browser, validate links and compile executable
examples as well as building strictly. Root policy snippets resolve from the
repository root. See [Node development](https://cellsweb.github.io/crpc-node/development/)
for its different pnpm/TypeScript workspace setup.
