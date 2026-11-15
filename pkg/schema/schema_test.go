package schema_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/CellsWeb/crpc-go/pkg/schema"
)

type TestUserStruct struct {
	UserID string `json:"userId" doc:"The unique user ID"`
	Email  string `json:"email" doc:"User email address"`
	Age    int    `json:"age,omitempty" doc:"User age"`
}

func TestFromStruct(t *testing.T) {
	rule := schema.FromStruct(TestUserStruct{})
	if rule.Type != schema.TypeObject {
		t.Fatalf("Expected type object, got %s", rule.Type)
	}

	if len(rule.Properties) != 3 {
		t.Fatalf("Expected 3 properties, got %d", len(rule.Properties))
	}

	userIdProp, ok := rule.Properties["userId"]
	if !ok || userIdProp.Optional {
		t.Fatalf("Expected required property 'userId'")
	}
	if userIdProp.Description != "The unique user ID" {
		t.Fatalf("Expected description 'The unique user ID', got %q", userIdProp.Description)
	}

	ageProp, ok := rule.Properties["age"]
	if !ok || !ageProp.Optional {
		t.Fatalf("Expected optional property 'age'")
	}
}

func TestValidateStrictUnknownFields(t *testing.T) {
	rule := schema.Object(map[string]*schema.SchemaRule{
		"name": schema.String().Min(2),
	})

	// Valid input
	errs := schema.Validate(rule, map[string]any{"name": "Alice"}, "")
	if len(errs) > 0 {
		t.Fatalf("Expected 0 errors, got %d: %v", len(errs), errs)
	}

	// Unknown field should be rejected
	errs = schema.Validate(rule, map[string]any{
		"name":  "Alice",
		"extra": "malicious",
	}, "")
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error for unknown field, got %d", len(errs))
	}
	if errs[0].Path != "/extra" {
		t.Fatalf("Expected error path '/extra', got %q", errs[0].Path)
	}
}

type mockFile struct {
	name string
	mime string
	size int64
}

func (m mockFile) Filename() string { return m.name }
func (m mockFile) MimeType() string { return m.mime }
func (m mockFile) Size() int64      { return m.size }

func TestFileAndArrayValidation(t *testing.T) {
	rule := schema.Object(map[string]*schema.SchemaRule{
		"avatar": schema.File().MaxSize(1024).AllowedMimes("image/png"),
		"photos": schema.Array(schema.File().AllowedMimes("image/jpeg")).MinFiles(1).MaxFiles(3),
	})

	// 1. Valid single file and array of files
	validPayload := map[string]any{
		"avatar": mockFile{name: "a.png", mime: "image/png", size: 500},
		"photos": []any{
			mockFile{name: "p1.jpg", mime: "image/jpeg", size: 200},
			mockFile{name: "p2.jpg", mime: "image/jpeg", size: 300},
		},
	}
	errs := schema.Validate(rule, validPayload, "")
	if len(errs) > 0 {
		t.Fatalf("Expected valid, got errors: %v", errs)
	}

	// 2. Avatar exceeding size
	oversizedPayload := map[string]any{
		"avatar": mockFile{name: "a.png", mime: "image/png", size: 2048},
		"photos": []any{
			mockFile{name: "p1.jpg", mime: "image/jpeg", size: 200},
		},
	}
	errs = schema.Validate(rule, oversizedPayload, "")
	if len(errs) != 1 || errs[0].Path != "/avatar" {
		t.Fatalf("Expected /avatar size error, got: %v", errs)
	}

	// 3. Photos exceeding max file count (4 files when max is 3)
	tooManyFiles := map[string]any{
		"avatar": mockFile{name: "a.png", mime: "image/png", size: 500},
		"photos": []any{
			mockFile{name: "p1.jpg", mime: "image/jpeg", size: 100},
			mockFile{name: "p2.jpg", mime: "image/jpeg", size: 100},
			mockFile{name: "p3.jpg", mime: "image/jpeg", size: 100},
			mockFile{name: "p4.jpg", mime: "image/jpeg", size: 100},
		},
	}
	errs = schema.Validate(rule, tooManyFiles, "")
	if len(errs) != 1 || errs[0].Path != "/photos" {
		t.Fatalf("Expected /photos max files error, got: %v", errs)
	}
}

func TestIntegerFractionalAndInfinityValidation(t *testing.T) {
	rule := schema.Object(map[string]*schema.SchemaRule{
		"count": schema.Integer().Min(1).Max(100),
	})

	// Valid integer
	errs := schema.Validate(rule, map[string]any{"count": 42}, "")
	if len(errs) > 0 {
		t.Fatalf("Expected valid integer, got %v", errs)
	}

	// Fractional float passed to TypeInteger -> must be rejected
	errs = schema.Validate(rule, map[string]any{"count": 2.75}, "")
	if len(errs) == 0 {
		t.Fatalf("Expected fractional float 2.75 to be rejected by TypeInteger")
	}

	// NaN and Inf checks
	errs = schema.Validate(rule, map[string]any{"count": math.Inf(1)}, "")
	if len(errs) == 0 {
		t.Fatalf("Expected infinity to be rejected")
	}
}

func TestUnicodeStringLength(t *testing.T) {
	rule := schema.Object(map[string]*schema.SchemaRule{
		"name": schema.String().Min(2).Max(5),
	})

	// 3-rune unicode string (multi-byte runes like emojis or Bengali/Japanese)
	// "বাংলা" is 5 runes, but 15 bytes in UTF-8
	errs := schema.Validate(rule, map[string]any{"name": "বাংলা"}, "")
	if len(errs) > 0 {
		t.Fatalf("Expected 5-rune unicode string to pass, got %v", errs)
	}

	// Too many runes (6 runes)
	errs = schema.Validate(rule, map[string]any{"name": "বাংলাদেশ"}, "")
	if len(errs) == 0 {
		t.Fatalf("Expected 6-rune string to be rejected by Max(5)")
	}
}

type CustomStringID struct {
	ID string
}

func (c CustomStringID) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", c.ID)), nil
}

func TestCustomJsonMarshalerWireRepresentationValidation(t *testing.T) {
	// Rule expects a String
	stringRule := schema.String().Min(3)

	customID := CustomStringID{ID: "usr_123"}
	errs := schema.Validate(stringRule, customID, "")
	if len(errs) > 0 {
		t.Fatalf("Expected CustomStringID to validate as string on wire, got errors: %v", errs)
	}

	// Rule expects an Object: CustomStringID emits a string on the wire, so it MUST fail Object validation
	objectRule := schema.Object(map[string]*schema.SchemaRule{
		"ID": schema.String(),
	})
	errs = schema.Validate(objectRule, customID, "")
	if len(errs) == 0 {
		t.Fatalf("Expected CustomStringID emitting string on wire to be rejected by Object schema")
	}
}
