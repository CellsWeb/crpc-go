# Go Unix IPC and Node

Use the maintained `example-node/src/client.ts` and generated contract types in
this repository rather than copying another client implementation into guides.
The client has connection-generation tracking, pending timer cleanup, concurrent
call correlation and tested failure/reconnection behavior.

HTTP middleware/auth rules remain in force over IPC. Forward credentials through
headers. Legacy CallOptions skipMiddlewares/context fields remain for source/wire
compatibility but are ignored by the Go bridge; they do not skip checks or inject
trusted identity. Do not use an SSR session object as a remote auth bypass.

<!-- compile-go: cmd/ipc/main.go -->
```go
package main

import (
    "log"
    _ "docsexamples/functions/hello"
    "github.com/CellsWeb/crpc-go/pkg/rpc"
    "github.com/CellsWeb/crpc-go/pkg/nodebridge"
)
func main() {
    app, err := rpc.New(rpc.Config{RpcPath:"/rpc", CORS:&rpc.CorsConfig{AllowOrigins:[]string{"*"}}})
    if err != nil { log.Fatal(err) }
    app.UseGlobalRegistry()
    if err := nodebridge.ServeUnixSocket(app, "/tmp/crpc-example.sock"); err != nil { log.Fatal(err) }
}
```

<!-- compile-ipc: ipc-usage.ts -->
```typescript
import { GoRpcClient, RpcError } from './src/client.js';
export async function profile(token: string) {
  const client = new GoRpcClient({ socketPath: '/tmp/crpc.sock' });
  try {
    return await client.call('user.getProfile', { id: 'user-1' },
      { headers: { authorization: `Bearer ${token}` }, timeoutMs: 5000 });
  } catch (error) {
    if (error instanceof RpcError) console.error(error.code, error.message);
    throw error;
  } finally { client.close(); }
}
```

Copy the maintained client and matching crpc.d.ts as a unit; this example's
user.getProfile contract must actually be registered in your daemon. Regenerate
types from your own app: call nodebridge.GenerateTypeScriptDeclarations(app.Functions())
and write its returned string. No `crpc-go export:types` command exists. File
uploads use HTTP multipart; the JSON IPC client does not transport live File/Blob
objects just because a generated HTTP-oriented type mentions them.

Unix requests support newline JSON or four-byte big-endian length framing. The
maximum is 2,097,152 bytes; newline counts for line frames, prefix is separate for
length frames. This Unix limit is not a promise about the separate stdio scanner.
Socket files use 0600 permissions; select an application-owned directory/path.

The Node client uses JSON-Lines responses. Socket closure, anonymous connection
errors and malformed response JSON promptly reject affected pending calls with
network_error and retire the connection. Later calls may reconnect; failed calls
are never automatically replayed and there is no automatic background backoff
loop. Matched valid application errors, ordinary timeouts and request IDs retain
their existing behavior. Some syntactically valid invalid envelopes are ignored;
do not claim a new wire-level validation convention.

IPC errors use the bridge's id/status/error envelope, not an invented HTTP error
wire format. Shared success data/validation/auth semantics do not imply every
HTTP/IPC code/details/output-omission field is identical. Test nil outputs and
generated contracts for your app. No universal 20–50µs latency guarantee applies.
