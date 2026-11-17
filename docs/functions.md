# Functions and contracts

Go uses explicit registration. Contracts, middleware attachments and handlers
can live in a co-located package; import it before UseGlobalRegistry. Examples
below are compiled together as a small application.

<!-- compile-go: functions/hello/contracts.go -->
```go
package hello

import (
    "github.com/CellsWeb/crpc-go/pkg/rpc"
    "github.com/CellsWeb/crpc-go/pkg/schema"
)
type Input struct { Name string `json:"name" doc:"Caller name"` }
type Output struct { Greeting string `json:"greeting"` }
func init() {
    rpc.Register(rpc.FunctionConfig[Input, Output]{
        Namespace: "public", FunctionName: "hello",
        Metadata: rpc.FunctionMetadata{Description: "Greet a named caller"},
        InputSchema: schema.Object(map[string]*schema.SchemaRule{"name": schema.String().Min(1)}),
        OutputSchema: schema.FromStruct(Output{}), Handler: Run,
    })
}
```

<!-- compile-go: functions/hello/run.go -->
```go
package hello

import "github.com/CellsWeb/crpc-go/pkg/rpc"
func Run(_ *rpc.Context, input Input) (*Output, error) {
    return &Output{Greeting: "Hello, " + input.Name + "!"}, nil
}
```

Attach middleware using `rpc.DefineMiddlewares(rpc.M("auth", options))` in the
FunctionConfig's Middlewares field. Actual credential verification belongs to
that middleware. Use ctx.Set/Get or rpc.GetContext to consume validated state;
remote HTTP/IPC payloads do not establish trusted context.

Register can infer object schemas from structs, but explicit native schemas are
needed for precise non-object/nullable contracts. HTTP omission defaults to {};
explicit null reaches validation as nil. Input/output checks remain active.

Uploaded files use native methods: Filename(), Size(), MimeType(), Bytes(),
Reader(), TempPath(), SaveTo(path). They are not exported struct fields. Persist
to a unique application-owned destination during processing; temporary input
files are cleaned afterward. Use native schema.File/Array rules and valid
multipart routing; nested files are not a newly supported transport format.

FunctionMetadata.IsPublic remains a deprecated ignored bool compatibility field.
It has no docs visibility, authorization, registration or execution effect. New
generators omit it; no replacement flag exists.
