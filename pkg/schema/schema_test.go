package schema_test

import (
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
