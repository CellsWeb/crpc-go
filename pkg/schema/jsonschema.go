package schema

import "encoding/json"

// JSONSchemaNode represents standard JSON Schema Draft 7/2020-12 structure
type JSONSchemaNode struct {
	Type                 string                     `json:"type,omitempty"`
	Description          string                     `json:"description,omitempty"`
	Format               string                     `json:"format,omitempty"`
	Properties           map[string]*JSONSchemaNode `json:"properties,omitempty"`
	Required             []string                   `json:"required,omitempty"`
	Items                *JSONSchemaNode            `json:"items,omitempty"`
	AdditionalProperties *bool                      `json:"additionalProperties,omitempty"`
	Default              any                        `json:"default,omitempty"`
	MinLength            *int                       `json:"minLength,omitempty"`
	MaxLength            *int                       `json:"maxLength,omitempty"`
	Minimum              *float64                   `json:"minimum,omitempty"`
	Maximum              *float64                   `json:"maximum,omitempty"`
}

// ToJSONSchema converts a SchemaRule into a standard JSON Schema node
func ToJSONSchema(rule *SchemaRule) *JSONSchemaNode {
	f := false
	if rule == nil {
		return &JSONSchemaNode{Type: "object", AdditionalProperties: &f}
	}

	node := &JSONSchemaNode{
		Type:        string(rule.Type),
		Description: rule.Description,
		Format:      rule.Format,
		Default:     rule.Default,
		MinLength:   rule.MinLength,
		MaxLength:   rule.MaxLength,
		Minimum:     rule.Minimum,
		Maximum:     rule.Maximum,
	}

	if rule.Type == TypeFile {
		node.Type = "string"
		node.Format = "binary"
		if node.Description == "" {
			node.Description = "Uploaded file"
		}
	}

	if rule.Type == TypeObject {
		node.AdditionalProperties = &f
	}

	if rule.Properties != nil {
		node.Properties = make(map[string]*JSONSchemaNode)
		for k, v := range rule.Properties {
			node.Properties[k] = ToJSONSchema(v)
		}
		if len(rule.Required) > 0 {
			node.Required = rule.Required
		}
	}

	if rule.Items != nil {
		node.Items = ToJSONSchema(rule.Items)
	}

	return node
}

// ToJSONString converts a SchemaRule to a pretty-printed JSON Schema string
func ToJSONString(rule *SchemaRule) (string, error) {
	node := ToJSONSchema(rule)
	bytes, err := json.MarshalIndent(node, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
