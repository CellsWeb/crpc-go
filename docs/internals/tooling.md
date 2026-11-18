# Go CLI, scaffold and declaration internals

Source: [cmd/crpc-go/main.go](https://github.com/CellsWeb/crpc-go/blob/main/cmd/crpc-go/main.go),
[cmd/crpc-go/commands/init.go](https://github.com/CellsWeb/crpc-go/blob/main/cmd/crpc-go/commands/init.go),
[cmd/crpc-go/commands/scaffold.go](https://github.com/CellsWeb/crpc-go/blob/main/cmd/crpc-go/commands/scaffold.go) and
[cmd/crpc-go/commands/dev.go](https://github.com/CellsWeb/crpc-go/blob/main/cmd/crpc-go/commands/dev.go).

## Command responsibilities

main.go dispatches init/create:function/create:middleware/dev/build/version and
holds the display version. InitProject writes the generated module, server,
function/middleware examples, utilities, Makefile and .env examples. ScaffoldFunction
and ScaffoldMiddleware write native registration/contracts/handlers and tests.
Flags differ from Node; Go has --multipart and --desc but no --middlewares flag.
Review generated imports/attachments explicitly.

Go registration depends on imports/init calls, not scanning folders on each
request. The generated main imports its example functions and middleware, calls
UseGlobalRegistry and starts the configured application. New packages must be
imported/registered deliberately; adding files is not function discovery.

## SDK version selection is not the display version

scaffoldSDKVersion reads runtime/debug build information. A proper versioned module
installation selects its installed module version. Missing/wrong/replaced/devel or
VCS-stamped source builds select the real published older v1.0.0 fallback. Never
replace that with a made-up unpublished version to make a source test appear
standalone. A CLI installed from the versioned v1.0.2 module selects SDK v1.0.2. Release
preparation validates this with an isolated candidate proxy; public installation
must be checked again after the real tag is published.

For local SDK development, create a disposable app and explicitly use go.work to
select the checkout. Its generated go.mod stays portable without parent-relative
replace paths. A source-built CLI printing1.0.2 does not prove SDK v1.0.2 selection;
inspect go.mod and go list -m output. See [development](../development.md).

## Environment and dev process

The generated main parses exported DOCS_ENABLED and DOCS_PATH. Omitted/blank docs
boolean uses NODE_ENV!=production; trimmed case-insensitive Go boolean values are
parsed, and invalid nonblank values stop startup. DOCS_PATH trims/defaults to/docs.
Neither the generated program nor DevServer loads.env automatically. Dev watches
Go/env files and rebuilds/restarts go run; watching an env file is not dotenv
loading. Export/source trusted shell values yourself.

The generated listener still configures0.0.0.0:4000 and /rpc. PORT/HOST/RPC_PATH
appear in env examples but are not consumed by that main. Do not add config
semantics while fixing prose or promise a runtime setting simply because a line
exists in.env. Programmatic Config remains the source of truth.

## TypeScript declaration generation

[pkg/nodebridge/ts_gen.go](https://github.com/CellsWeb/crpc-go/blob/main/pkg/nodebridge/ts_gen.go) exports GenerateTypeScriptDeclarations,
which consumes app.Functions() and returns source text. The caller writes it to
a file. No crpc-go export:types command exists. The maintained example-node client
imports its matching crpc.d.ts contract map; regenerate it for the actual daemon.
Generated HTTP-oriented file types do not make JSON IPC transfer live files.

## Regression gates

init_module_test.go covers portable modules/version selection; init_test.go runs
actual generated server env/docs cases; init_standalone_test.go performs optional
real public-SDK/workspace integration; docs_metadata_test.go checks generated
metadata. Bridge ts_gen_test.go covers declaration output. Test the generated
program/build/dependency selection, not only a template string. Release/version
changes require the [release checklist](releasing.md), not just a display edit.
