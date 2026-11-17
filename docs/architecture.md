# Architecture and protocol

The configured HTTP endpoint accepts `{namespace,functionName,data}`. Functions
register explicitly, then dispatch through middleware, input validation, the
handler and output validation. Authentication/authorization are application
middleware responsibilities, also enforced for in-memory/IPC execution.

Success uses `{status:"ok",data,meta?}`. Handled RPC application errors default to
HTTP 200 with `{status:"error",code,message,details?}`. UseHttpStatusCodes is an
explicit opt-in; transport/framing failures, custom routes and docs404 differ.
This is not a proxy caching/security guarantee.

JSON objects, arrays, scalars and explicit null preserve shape. Omitted HTTP data
defaults to {}; null reaches schema validation/handlers as nil. Other transports'
omission/output conventions remain existing behavior. Native schemas are not a
complete TypeBox/JSON Schema implementation; compare the actual shared contracts.

HTTP uses encoding/json/v2 for its envelope/serialization. Other in-memory/IPC
paths use existing encoding/json conventions. Do not claim duplicate-member,
nil slice/map, number or allocation behavior is identical across every transport.
Output validation remains active; application custom marshalers need real tests.

fasthttp buffers transport intake before application multipart streaming. The
net/http adapter is not a guarantee of end-to-end streaming. Header/body routing
must agree and duplicates fail before dispatch; headerless routing is supported.
Only top-level/direct-array file transport is supported. Per-request bounds and
cleanup are not a concurrent aggregate-memory guarantee or content sanitization.

Unix IPC frames are bounded 2MiB, with deadlines and cleanup. The Node example
client rejects connection-level failures without replay. A credential/identity
cannot bypass middleware by arriving as a caller-supplied context object.

Docs are controlled globally. IsPublic is retained/deprecated/ignored, not a
visibility/auth flag. Safe structural middleware docs omit configured values and
defaults without executing handlers or changing runtime schema behavior.
