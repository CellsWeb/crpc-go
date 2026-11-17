# Public API reference

Module: github.com/CellsWeb/crpc-go. Candidate v1.0.1 is not public v1.0.0; see
[installation](getting-started.md). Go >=1.27.4 is required.

## rpc

New(Config) requires an explicit RpcPath and non-nil CORS policy. Configure
Host/Port, DocsEnabled/DocsPath, request/file limits, timeouts, proxy trust and
optional UseHttpStatusCodes. Core Config reads no environment variables.

| API | Purpose |
|---|---|
| Register(FunctionConfig[I,O]) | Typed function wrapper/global registration |
| app.RegisterFunction / RegisterMiddleware | Explicit instance registration |
| app.UseGlobalRegistry | Import init-registered modules |
| app.Handler / NetHTTPHandler | fasthttp / net-http handlers |
| app.ListenAndServe / Shutdown | Native server lifecycle |
| app.GET/POST/PUT/DELETE | Custom func(*HTTPContext) error handlers |
| app.CallInMemory | Populate output target through the execution pipeline |
| NewTestCaller, Call, CallRaw | In-memory test helpers |

MiddlewareFn is func(*Context,map[string]string,map[string]any) error. Store
trusted state with Context.Set/Get or GetContext[T]. Caller data/legacy bypass
fields never establish identity. Middleware metadata schemas describe contracts;
Go does not apply new automatic option/header validation.

NewError(code,message,details...) creates application errors. Default HTTP status
for handled RPC failures is200; opt-in mapping, transport errors and custom-route
statuses differ. Inspect envelope status/code, not HTTP status alone.

UploadedFile has methods OriginalFilename, Filename, Size, TempPath, MimeType,
DeclaredMimeType, DetectedMimeType, Bytes, Reader, SaveTo and Cleanup. Size and
Filename are methods, not fields. SaveTo creates a new 0600 destination without
overwriting; choose a unique application-owned path. Framework cleanup removes
the temporary input, not the copy you persist with SaveTo.

## schema and nodebridge

SchemaRule exposes native structural/constraint metadata. Use String/Number/
Integer/Boolean/Object/Array/File/Optional, FromStruct and their fluent methods;
examples are in [schemas](schemas.md). IsPublic remains an ignored deprecated
bool on FunctionMetadata; no visibility/auth replacement is introduced.

nodebridge.ServeUnixSocket(app,path) serves bounded Unix frames. GenerateTypeScriptDeclarations
takes app.Functions() and returns a declaration string; there is no export:types
CLI command. See [IPC](node-ipc.md) for the maintained Node client and framing.
