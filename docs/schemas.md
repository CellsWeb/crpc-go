# Native schemas

Use schema.SchemaRule builders or FromStruct for object contracts. These native
rules overlap with TypeBox but are not an implementation of every JSON Schema
keyword or union. Keep shared logical conformance tests for the contracts used.

<!-- compile-go: schemas/schemas.go -->
```go
package schemas

import "github.com/CellsWeb/crpc-go/pkg/schema"
var Profile = schema.Object(map[string]*schema.SchemaRule{
    "name": schema.String().Min(1).Max(100).Describe("Display name"),
    "age": schema.Optional(schema.Integer().MinValue(0)),
    "avatar": schema.Optional(schema.File().MaxSize(5*1024*1024).AllowedMimes("image/png")),
})
var NullableText = schema.Optional(schema.String())
var Names = schema.Array(schema.String())
```

Native Optional allows nil. This does not change HTTP omission: omitted data is
an empty object, so a nullable string schema rejects it. Explicit null is nil and
can be accepted. Object property absence/nil and output omission are separate
contracts; test typed pointers/omitempty explicitly where they matter.

FromStruct reads json names/omitempty, doc/description, min/max, min_files/
max_files, max_size and mimes tags. Use explicit builders for precise non-object
or numeric constraints. Defaults stored in schema metadata are not an automatic
application-default engine. Input/output validation remains enforced; middleware
option/header schemas are documentation metadata and do not add pipeline checks.

Go objects reject fields absent from their Properties contract. HTTP JSON v2
rejects duplicate members; IPC/in-memory use existing JSON decoding conventions.
Do not assume identical large-integer precision or every keyword across Node/Go.
File MIME detection/limits do not sanitize the content itself.
