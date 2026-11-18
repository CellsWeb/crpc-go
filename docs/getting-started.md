# Getting started

## Public installation

Go>=1.27.4 is required. Module and CLI version **v1.0.2** use the same versioned release:

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

Put GOBIN, or the default Go bin directory, on PATH. The versioned installed CLI
selects SDK v1.0.2 automatically. No parent checkout, replace directive or workspace
is required for generated applications. For an existing module, use
`go get github.com/CellsWeb/crpc-go@v1.0.2`.

Local/VCS-stamped builds retain the older v1.0.0 fallback because a local Git
revision may not identify published code. This is independent of the CLI's
human-readable version. Use `go work init . /absolute/path/to/rpc-go` only for
intentional local SDK development; keep machine-specific workspaces local.
`GOWORK=off` selects the declared released dependency. The older v1.0.0 does not
contain the v1.0.2 hardening fixes; prefer the versioned installed CLI for users.

## Generated layout and CLI

The app contains cmd/server/main.go, functions/user/getProfile and updateProfile,
middlewares/auth, utilities, go.mod, Makefile and .env examples. Go functions
register through init(); the server imports modules and calls UseGlobalRegistry.
Add new module imports/registration deliberately; there is no filesystem function
discovery or automatic TypeScript export CLI command in Go.

```sh
crpc-go create:function --ns user --name updateEmail
crpc-go create:function --ns user --name uploadAvatar --multipart
crpc-go create:middleware --name auth --desc 'Verify credentials'
crpc-go build
```

Use batch/file specs for generation where needed. Unlike Node's function CLI,
Go has no `--middlewares` flag; edit the generated middleware attachments. The
sample middleware is a no-op customization stub. Implement real verification
before relying on auth; schema metadata does not do it for you.

## Environment and docs

Neither the generated binary nor `crpc-go dev` automatically loads .env. Export
deployment values, or source the trusted shell-compatible generated file:

```sh
set -a
. ./.env
set +a
go run ./cmd/server
```

Final exported values win; sourcing replaces earlier exports, later exports
replace sourced values. DOCS_ENABLED accepts trimmed case-insensitive true/false,
1/0 and Go t/f aliases. Unset/blank uses `NODE_ENV != "production"`; invalid
nonblank values stop startup, including production. DOCS_PATH trims and defaults
to /docs. Core programmatic DocsEnabled/DocsPath are separate and read no env.

The current generated listener still hard-codes 0.0.0.0:4000 and /rpc. Although
the env example includes PORT/HOST/RPC_PATH, main does not consume them; edit
rpc.Config explicitly when deploying on another address/path. No new env parser
or listener feature is introduced by these docs.

Deprecated IsPublic is ignored; docs availability is global, independent of
middleware/auth and function execution. See [docs controls](interactive-docs.md).
