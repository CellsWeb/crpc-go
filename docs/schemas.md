# Native schemas

Use `schema.SchemaRule` builders or `FromStruct` for object contracts. These native
rules overlap with TypeBox but are not an implementation of every JSON Schema
keyword or union. Keep shared logical conformance tests for the contracts used.

<!-- compile-go: schemas/schemas.go -->
```go
package schemas

import "github.com/CellsWeb/crpc-go/pkg/schema"

var Submission = schema.Object(map[string]*schema.SchemaRule{
	"title":  schema.String().Min(1).Max(100),
	"score":  schema.Number().MinValue(0.0),
	"count":  schema.Integer().MinValue(0),
	"active": schema.Boolean(),
	"avatar": schema.Optional(schema.File().MaxSize(5 * 1024 * 1024).AllowedMimes("image/png")),
})

var Profile = schema.Object(map[string]*schema.SchemaRule{
	"name":   schema.String().Min(1).Max(100).Describe("Display name"),
	"age":    schema.Optional(schema.Integer().MinValue(0)),
	"avatar": schema.Optional(schema.File().MaxSize(5 * 1024 * 1024).AllowedMimes("image/png")),
})
var NullableText = schema.Optional(schema.String())
var Names = schema.Array(schema.String())
```

## Optional and Nullable Contracts

Native Optional allows nil. This does not change HTTP omission: omitted data is
an empty object, so a nullable string schema rejects it. Explicit null is nil and
can be accepted. Object property absence/nil and output omission are separate
contracts; test typed pointers/omitempty explicitly where they matter.

## Schema Builders and Struct Inference

`FromStruct` reads json names/omitempty, doc/description, min/max, min_files/max_files,
max_size, and mimes tags. Use explicit builders (`schema.String()`, `schema.Number()`,
`schema.Integer()`, `schema.Boolean()`, `schema.File()`) for precise non-object or
numeric constraints. Defaults stored in schema metadata are not an automatic
application-default engine. Input/output validation remains enforced; middleware
option/header schemas are documentation metadata and do not add pipeline checks.

## Schema-Directed Multipart Coercion

In multipart requests (`multipart/form-data`), all form fields arrive over HTTP framing as
raw text strings. Unlike Node (which uses explicit `FormNum()` / `FormBool()` wrapper schemas),
Go provides **automatic schema-directed multipart coercion** during transport intake
(`schema.CoerceMultipartData`).

Before input validation runs on a multipart request, the engine traverses the declared
`InputSchema` and automatically converts string form fields into native types matching the schema:

| Schema Rule | FormData Input | Coerced Value & Handler Type |
|---|---|---|
| `schema.String()` | `"42"` | string `"42"` (remains a string) |
| `schema.Number()` | `"42"` | float64 `42.0` (numeric) |
| `schema.Integer()` | `"42"` | int64 `42` (integer) |
| `schema.Boolean()` | `"true"` or `"1"` | bool `true` |
| `schema.Boolean()` | `"false"` or `"0"` | bool `false` |

### String Retention vs Numeric Coercion

Because coercion is strictly schema-directed:
- A field declared as `schema.String()` receives the raw string `"42"`.
- The exact same input `"42"` under `schema.Number()` or `schema.Integer()` is automatically parsed into a float64 or int64 before the handler receives it.
- Coercion applies recursively through nested objects and multi-dimensional arrays declared in the schema.

### Accepted Boolean Strings

For `schema.Boolean()`, multipart intake accepts:
- `"true"` and `"1"` (case-insensitive and trimmed) -> coerced to `true`
- `"false"` and `"0"` (case-insensitive and trimmed) -> coerced to `false`

Any unrecognized string (e.g. `"invalid"`, `"yes"`, `"off"`) cannot be parsed as a boolean, remains a string, and consequently fails boolean schema validation.

### Strict JSON Validation Remains Enforced

Schema-directed coercion is strictly confined to multipart transport intake. Normal JSON
requests (`Content-Type: application/json`) remain strictly validated:
- In JSON requests, passing a string `"42"` for a field declared with `schema.Number()` or `schema.Integer()` is **rejected** with a validation error.
- JSON payloads must match their declared native JSON types directly without transport coercion.

### Validation Errors and HTTP 200 Contract

When an invalid value is provided (e.g. `"abc"` for `schema.Number()`, `"invalid"` for `schema.Boolean()`, or a numeric constraint violation), validation fails before handler invocation.

Under the framework's uniform architecture, handled application errors return HTTP 200 OK by default with an RPC error envelope:

```json
{
  "status": "error",
  "code": "validation_error",
  "message": "validation failed",
  "details": [
    {
      "path": "score",
      "message": "expected number, got string"
    }
  ]
}
```

The raw HTTP status remains 200 OK by default; transport/framing failures are distinct from handled application validation errors. Applications requiring conventional HTTP error statuses can optionally enable `UseHttpStatusCodes: true` in `rpc.Config`.

## General Guidelines

Go objects reject fields absent from their Properties contract. HTTP JSON v2
rejects duplicate members; IPC/in-memory use existing JSON decoding conventions.
Do not assume identical large-integer precision or every keyword across Node/Go.
File MIME detection/limits do not sanitize the content itself.
