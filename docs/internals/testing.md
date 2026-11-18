# Go test architecture and safe extension

Choose the layer that owns the behavior; do not assume all invocation paths share
one implementation. The [validation matrix](validation.md) documents return/output
check differences that tests need to preserve or deliberately version.

| Subsystem | Real test file | Relevant coverage |
|---|---|---|
| HTTP engine/config | [pkg/rpc/engine_test.go](https://github.com/CellsWeb/crpc-go/blob/main/pkg/rpc/engine_test.go) | Routing headers, middleware, transport status opt-in, telemetry/CORS |
| HTTP JSON shapes | [pkg/rpc/http_json_shapes_test.go](https://github.com/CellsWeb/crpc-go/blob/main/pkg/rpc/http_json_shapes_test.go), [pkg/rpc/testdata/http-json-shapes.json](https://github.com/CellsWeb/crpc-go/blob/main/pkg/rpc/testdata/http-json-shapes.json) | Missing data versus null, arbitrary shapes, schemas, auth, both HTTP handlers |
| CallInMemory | [pkg/rpc/caller_test.go](https://github.com/CellsWeb/crpc-go/blob/main/pkg/rpc/caller_test.go) | Native/typed shapes, validation and middleware |
| CallRaw | [pkg/rpc/testing_test.go](https://github.com/CellsWeb/crpc-go/blob/main/pkg/rpc/testing_test.go) | Shape parity, typed input, validation/auth/options and Go error-versus-envelope returns |
| Multipart/files | [pkg/rpc/multipart_test.go](https://github.com/CellsWeb/crpc-go/blob/main/pkg/rpc/multipart_test.go) | File limits, primitive/array coercion, field sanitization and cleanup |
| Custom HTTP routes | [pkg/rpc/http_routes_test.go](https://github.com/CellsWeb/crpc-go/blob/main/pkg/rpc/http_routes_test.go) | Route precedence/helpers/internal calls |
| Native schemas | [pkg/schema/schema_test.go](https://github.com/CellsWeb/crpc-go/blob/main/pkg/schema/schema_test.go) | Native rule types, struct JSON representation, files/limits |
| Embedded docs | [pkg/rpc/docs_test.go](https://github.com/CellsWeb/crpc-go/blob/main/pkg/rpc/docs_test.go), [pkg/rpc/docs_parity_test.go](https://github.com/CellsWeb/crpc-go/blob/main/pkg/rpc/docs_parity_test.go), [pkg/rpc/docs_parity_http_test.go](https://github.com/CellsWeb/crpc-go/blob/main/pkg/rpc/docs_parity_http_test.go) | Actual content, safe projection, required headers, custom/global docs and no execution |
| Unix framing | [pkg/nodebridge/bridge_test.go](https://github.com/CellsWeb/crpc-go/blob/main/pkg/nodebridge/bridge_test.go) | Size allocation boundary, exact limit, no-newline/partial/malformed/deadline behavior and compatibility |
| Generated declarations | [pkg/nodebridge/ts_gen_test.go](https://github.com/CellsWeb/crpc-go/blob/main/pkg/nodebridge/ts_gen_test.go) | Native schema-to-TypeScript mapping |
| Generated apps | [cmd/crpc-go/commands/init_test.go](https://github.com/CellsWeb/crpc-go/blob/main/cmd/crpc-go/commands/init_test.go), [cmd/crpc-go/commands/init_module_test.go](https://github.com/CellsWeb/crpc-go/blob/main/cmd/crpc-go/commands/init_module_test.go), [cmd/crpc-go/commands/init_standalone_test.go](https://github.com/CellsWeb/crpc-go/blob/main/cmd/crpc-go/commands/init_standalone_test.go) | Real executable/env/docs, version selection, public resolution and explicit workspace |
| Node IPC client | [example-node/test/client.test.mjs](https://github.com/CellsWeb/crpc-go/blob/main/example-node/test/client.test.mjs), [example-node/test/corruption.test.mjs](https://github.com/CellsWeb/crpc-go/blob/main/example-node/test/corruption.test.mjs), [example-node/test/go-interop.test.mjs](https://github.com/CellsWeb/crpc-go/blob/main/example-node/test/go-interop.test.mjs) | Mock sockets plus actual daemon, close/corruption/anonymous errors, IDs/timers/reconnect/no replay |

The actual daemon fixture is
[example-node/test/testdata/ipc-daemon.go](https://github.com/CellsWeb/crpc-go/blob/main/example-node/test/testdata/ipc-daemon.go); it is testdata, not a
production-discovered main package. helpers.mjs owns fixture sockets/timers and
build/race switches. Keep test scaffolding out of production package registration.

## Focused and full commands

```sh
go test -count=1 ./pkg/rpc -run TestHTTPJSONPayloadShapes
go test -count=1 ./pkg/rpc -run TestCallRaw
go test -count=1 ./pkg/nodebridge
go build ./...
go test -count=1 ./...
go vet ./...
go test -race -count=1 ./...
CRPC_STANDALONE_INTEGRATION=1 CRPC_SCAFFOLD_RACE=1 go test -race -count=1 ./...
```

Standalone tests are opt-in, require registry/network access and an unused 4000
port, and compare the source-built CLI's actual public fallback with a deliberate
local workspace. They are not proof that every newly tagged version has reached
the public proxy; release testing installs that exact version separately.

From example-node run npm ci, npm run typecheck and CRPC_IPC_TEST_RACE=1 npm test.
It builds TypeScript and instruments the actual Go daemon, including fault-injecting
proxy tests. Ordinary root race testing does not automatically run this npm suite.
Root Go ./... also excludes nested example/benchmark modules and live databases.

## Regression and cross-runtime workflow

Pin a baseline, reproduce over the real affected transport, add a failing test,
then patch the smallest owning function. Include objects/arrays/scalars/null,
malformed input, unauthorized/missing middleware, handler non-execution, typed
wrappers, output errors and cleanup when relevant. Network tests need explicit
shutdown/deadlines so failure does not hang a suite. Record exact commands and
remaining limitations; a no-tests-matched exit0 is not regression evidence.

For shared changes, run logically equivalent Node fixtures. Use this repo's JSON
fixture set and Node union/HTTP regressions rather than assuming TypeBox syntax
exists here. No private review-kit parity runner is a contributor prerequisite.
Do not refactor duplicated pipelines, expand stdio framing or synchronize registry
mutation as a side effect of documentation. Architectural changes need explicit
scope, compatibility discussion and tests across every execution site.
