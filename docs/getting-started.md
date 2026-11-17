# Getting started

## Release status

Go >=1.27.4 is required. This branch prepares **v1.0.1**. The current public tag
v1.0.0 is an older SDK without the verified hardening and docs fixes. A source
binary printing 1.0.1 does not prove that module version is public.

For the unpublished candidate, build the CLI from this checkout:

```sh
go build -o /tmp/crpc-go-candidate ./cmd/crpc-go
/tmp/crpc-go-candidate init /tmp/my-crpc-app
cd /tmp/my-crpc-app
go work init . /absolute/path/to/rpc-go
go build ./...
go run ./cmd/server
```

Replace the absolute checkout path and keep this machine-specific workspace
local. It explicitly selects the candidate SDK. Generated apps contain no
parent/sibling `replace` and can build independently against a real released SDK.

After approved publication of the new tag, the normal installation is:

```sh
go install github.com/CellsWeb/crpc-go/cmd/crpc-go@v1.0.1
crpc-go init my-api
cd my-api
go mod tidy
go build ./...
crpc-go dev
```

These versioned commands are a post-publication step; do not assume an uncreated
tag resolves. Install binaries under GOBIN (or the default Go bin directory) and
put that directory on PATH. The installed CLI's module version selects the SDK
dependency. Local/VCS-stamped/development builds use the **existing older v1.0.0
fallback**, because a local Git-derived pseudo-version may be unpublished.
`GOWORK=off` selects the declared public dependency rather than your workspace.

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
