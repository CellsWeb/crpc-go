# Testing

CallInMemory and TestCaller run without an HTTP socket, through middleware and
schema checks. They are not transport tests and do not bypass authorization.
Call unwraps data/errors; CallRaw returns an envelope plus its helper error.
An application error can be in that envelope even when the Go error return is nil.

<!-- compile-go: testing/testing_test.go -->
```go
package testing_test

import (
    "testing"
    _ "docsexamples/functions/hello"
    "github.com/CellsWeb/crpc-go/pkg/rpc"
)
func TestHello(t *testing.T) {
    app, err := rpc.New(rpc.Config{RpcPath: "/rpc", CORS: &rpc.CorsConfig{AllowOrigins: []string{"*"}}})
    if err != nil { t.Fatal(err) }
    app.UseGlobalRegistry()
    caller := rpc.NewTestCaller(app)
    response, err := caller.CallRaw("public.hello", map[string]any{"name":"Test"})
    if err != nil || response == nil || response.Status != "ok" { t.Fatalf("%+v %v", response, err) }
    response, err = caller.CallRaw("public.hello", map[string]any{"name":123})
    if err != nil || response == nil || response.Code != rpc.ErrValidation { t.Fatalf("%+v %v", response, err) }
}
```

Both helpers support objects, arrays, scalars and nil subject to schemas. Return
types differ: CallInMemory populates your output target; CallRaw exposes protocol
metadata. Use Headers in CallOptions/TestOptions to supply valid test credentials;
do not inject unverified identity to make authorization tests pass.

Release checks: go build, full go test, race detector, vet, real HTTP multipart/
docs tests and Node IPC tests. Enable CRPC_STANDALONE_INTEGRATION=1 for actual
public-module scaffold resolution; enable CRPC_SCAFFOLD_RACE=1 to instrument
generated child binaries. Those integration cases require a public proxy/cache
and an unused port 4000. Node example tests use actual Go IPC daemons and optional
CRPC_IPC_TEST_RACE=1, covering disconnect/corruption/reconnect without replay.
