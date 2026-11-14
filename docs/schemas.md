---
layout: default
title: Data Types & Schema Validation
nav_order: 5
---

# Data Types & Schema Validation

CellsWeb RPC Go provides two flexible approaches for defining input and output data contracts:
1. **Fluent TypeBox/Zod-Like Schema Builder (`pkg/schema`)**: Full control over runtime constraints (`Min`, `Max`, `Email`, `AllowedMimes`, `MaxSize`).
2. **Plain Go Structs (`schema.FromStruct`)**: Automatic reflection and type inference with zero boilerplate.

---

## 🏗️ 1. Fluent Schema Builder Syntax

The `pkg/schema` package provides a declarative, chaining builder API:

```go
package register

import "github.com/CellsWeb/crpc-go/pkg/schema"

var InputSchema = schema.Object(map[string]*schema.SchemaRule{
	// String constraints
	"name":     schema.String().Min(2).Max(50).Describe("Full user name"),
	"email":    schema.String().Email().Describe("Verified email address"),
	"password": schema.String().Min(8).Max(128).Describe("Secure account password"),

	// Numeric constraints
	"age":      schema.Optional(schema.Integer().MinValue(18).MaxValue(120)),
	"balance":  schema.Optional(schema.Number().MinValue(0.0)),

	// Binary file uploads with size & MIME constraints
	"avatar":   schema.Optional(schema.File().
		MaxSize(5 * 1024 * 1024).                                    // 5 MB limit
		AllowedMimes("image/png", "image/jpeg", "image/webp").        // Allowed MIME types
		Describe("User profile image file")),

	// Arrays and lists
	"tags":     schema.Optional(schema.Array(schema.String())),
})
```

---

## 🗂️ 2. Plain Go Struct Syntax (Automatic Derivation)

Alternatively, you can define contracts using standard Go structs. The schema engine automatically derives constraints from field types and struct tags:

```go
type Input struct {
	Name     string            `json:"name" doc:"Full user name"`
	Email    string            `json:"email" doc:"Verified email address"`
	Password string            `json:"password" doc:"Secure account password"`
	Age      int               `json:"age,omitempty" doc:"User age"`
	Avatar   *rpc.UploadedFile `json:"avatar,omitempty" doc:"User profile image file"`
}

// Auto-derive schema from struct:
var InputSchema = schema.FromStruct(Input{})
```

### Supported Struct Tags
- `json:"fieldName"`: Specifies JSON payload property name.
- `json:",omitempty"`: Automatically marks the field as **optional**.
- `doc:"Description"`: Supplies the human-readable description rendered on `/docs`.

---

## 📋 Complete Data Type & Validator Catalog

### String Rules (`schema.String()`)

| Method | Example | Description |
| :--- | :--- | :--- |
| `.Min(int)` | `schema.String().Min(3)` | Minimum character length |
| `.Max(int)` | `schema.String().Max(100)` | Maximum character length |
| `.Email()` | `schema.String().Email()` | Enforces valid email syntax |
| `.Pattern(regex)`| `schema.String().Pattern("^[A-Z0-9]+$")` | Regex format constraint |
| `.Describe(text)`| `schema.String().Describe("User handle")` | Documentation note |
| `.WithDefault(v)`| `schema.String().WithDefault("guest")` | Fallback default value |

---

### Numeric Rules (`schema.Integer()` & `schema.Number()`)

| Method | Example | Description |
| :--- | :--- | :--- |
| `.MinValue(float64)` | `schema.Integer().MinValue(0)` | Minimum value constraint |
| `.MaxValue(float64)` | `schema.Number().MaxValue(9999.99)` | Maximum value constraint |
| `schema.Integer()` | `schema.Integer()` | Whole numbers (int, int64) |
| `schema.Number()` | `schema.Number()` | Floating-point numbers (float64) |

---

### Binary File Rules (`schema.File()`)

CellsWeb RPC separates file upload validation directly into dedicated schema rules:

```go
var AvatarSchema = schema.File().
	MaxSize(10 * 1024 * 1024).                          // 10 Megabytes max
	AllowedMimes("image/jpeg", "image/png", "image/webp").
	Describe("High-resolution avatar graphic")
```

| Method | Example | Description |
| :--- | :--- | :--- |
| `.MaxSize(int64)` | `.MaxSize(5 * 1024 * 1024)` | Strict byte size limit (enforced during stream) |
| `.AllowedMimes(...)` | `.AllowedMimes("application/pdf")` | Whitelist of permitted MIME types |
| `schema.Optional(...)` | `schema.Optional(schema.File())` | Marks file upload as optional |

---

### Arrays & Objects

```go
// Array of primitives
var TagList = schema.Array(schema.String())

// Nested Object
var AddressSchema = schema.Object(map[string]*schema.SchemaRule{
	"street": schema.String(),
	"city":   schema.String(),
	"zip":    schema.String().Min(5).Max(10),
})
```
