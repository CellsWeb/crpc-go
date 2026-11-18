# Native schemas, call APIs and errors

Source: [pkg/schema/rules.go](https://github.com/CellsWeb/crpc-go/blob/main/pkg/schema/rules.go), [pkg/schema/validator.go](https://github.com/CellsWeb/crpc-go/blob/main/pkg/schema/validator.go),
[pkg/schema/jsonschema.go](https://github.com/CellsWeb/crpc-go/blob/main/pkg/schema/jsonschema.go) and the four execution sites in pkg/rpc.

## SchemaRule is native metadata plus validation rules

Constructors cover string, number, integer, boolean, array, object and file.
Fluent methods add descriptions, requiredness, length/value/count/MIME/size rules.
FromStruct uses Go types/tags to infer a contract; ToMap/ToJSONString export docs
metadata. This is not a general JSON Schema evaluator or TypeBox union engine.
Do not invent Nullable/Union constructors absent from the public API.

Validate recursively gathers path/message details. A nil value is rejected unless
the rule is Optional; objects check known/required fields and nested rules.
Optional therefore has native null behavior that is not identical to TypeBox's
missing-property flag. Structs and supported JSON marshalers are normalized toward
their actual JSON representation, preserving tags/omitempty/embedded-field effects.
WithDefault records metadata; Validate does not automatically populate defaults.

File validation uses the native FileValidator interface for filename/size/MIME.
JSON-looking multipart fields and schema-directed numeric/boolean coercion are
intake behavior, not global JSON coercion. CoerceMultipartData/CoerceValue recurse
through declared structures before multipart input validation. Keep ordinary JSON
validation strict and compare logical cases with Node instead of copying its DSL.

## Execution-site matrix

| Path | Input + middleware | Output behavior | Return/error form |
|---|---|---|---|
| HTTP JSON | Parse v2 envelope, run middleware, native schema validation | Non-nil result checked if schema present | HTTP ResponseEnvelope / mapped or default200 |
| HTTP multipart | Pre-auth, parse files, consistency, coercion, input validation | Non-nil result checked if schema present | HTTP ResponseEnvelope and deferred file cleanup |
| CallInMemory | Native map shortcut or arbitrary JSON marshal/decode; middleware and input checks | Non-nil result checked; encode into supplied target | error plus output target; no HTTP status |
| TestCaller.CallRaw | Similar normalization; middleware and input checks | Raw handler result currently not output-schema validated | *ResponseEnvelope,error; local marshal failures can be Go errors |
| Unix/stdio bridge | Decode BridgeRequest, then CallInMemory | Inherits CallInMemory; response encoding has omitempty | BridgeResponse with ID/status/data/error, not HTTP envelope |

This matrix is descriptive, not a request to harmonize/refactor behavior. In
CallInMemory errors may contain less field detail than CallRaw/HTTP; not-found,
output mismatch, nil output and ordinary error formatting can differ. CallRaw's
name means an envelope result, not unchecked input or a second transport parser.

## Errors and serialization

[pkg/rpc/errors.go](https://github.com/CellsWeb/crpc-go/blob/main/pkg/rpc/errors.go) defines AppError/codes/ValidationErrorDetail.
HTTP dispatchError extracts structured application errors and sendErrorResponse
creates the envelope. UseHttpStatusCodes chooses the explicit status mapper;
otherwise handled application and parser errors return200. A library/framework
framing rejection or custom route has its own status/body boundary.

HTTP engine and native schema normalization use encoding/json/v2 in this release;
CallInMemory/CallRaw/bridge adapters also contain encoding/json v1 paths. They are
not universally identical parsers. Test duplicate members, typed maps, nil outputs,
large numbers and custom marshalers explicitly when relevant. JSON input shape
preservation does not imply identical output-null/omitempty bytes across runtimes.

## Tests for a safe change

Use schema/schema_test.go for native rules; rpc/http_json_shapes_test.go plus its
checked-in testdata for HTTP omission/null/arrays/scalars; caller_test.go for
CallInMemory and typed inputs; testing_test.go for CallRaw shape/validation/auth
and Go error-versus-envelope conventions. Assert no handler invocation on invalid
input and that middleware still runs at the expected stage. Changing typed wrappers
also needs real IPC/file injection coverage. See [testing](testing.md).
