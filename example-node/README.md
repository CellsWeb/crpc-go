# Node client for the Go Unix daemon

Use Node 24 LTS for the tested workflow and Go >=1.27.4 for the daemon. This source
is included in public Go release v1.0.1. The historical v1.0.0 predates the
connection/parse fixes.

```sh
npm install
npm run typecheck
CRPC_IPC_TEST_RACE=1 npm test
```

Tests build the client, start actual Go test daemons and exercise framing,
correlation, concurrent calls, timeouts, corruption, reconnection/no replay and
timer/listener cleanup. Running the demonstration index against a real app may
require its example service/database; passing the test fixture is not a MongoDB
deployment test.

Copy src/client.ts with the matching crpc.d.ts generated from your registered
application. The generated file's types are contracts, not automatic function
registration or file transport. Invoke `new GoRpcClient({socketPath})`, forward
credentials in call options.headers, and handle RpcError/close the client when
done. See ../docs/node-ipc.md for the compiling usage example.

Attached Go middleware always runs; applications implement authentication and authorization there. Legacy skipMiddlewares/context
options remain for compatibility and are ignored by the Go bridge; they do not
skip verification or inject a trusted session. Namespace application state in
Go middleware with ctx.Set, then read it in handlers. Metadata IsPublic is ignored.

Connection errors reject pending calls promptly. A later call can reconnect,
but no failed request is replayed automatically. Do not add blind retries for
non-idempotent operations. Unix frame limits and protocol differences are described
in the IPC guide; no universal latency guarantee is made.
