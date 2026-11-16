package schema

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// RuleType denotes the schema data type
type RuleType string

const (
	TypeString  RuleType = "string"
	TypeNumber  RuleType = "number"
	TypeInteger RuleType = "integer"
	TypeBoolean RuleType = "boolean"
	TypeArray   RuleType = "array"
	TypeObject  RuleType = "object"
	TypeFile    RuleType = "file"
)

// SchemaRule defines a single validation constraint & documentation node
type SchemaRule struct {
	Type        RuleType              `json:"type"`
	Optional    bool                  `json:"optional,omitempty"`
	Description string                `json:"description,omitempty"`
	MinLength   *int                  `json:"minLength,omitempty"`
	MaxLength   *int                  `json:"maxLength,omitempty"`
	Pattern     string                `json:"pattern,omitempty"`
	Format      string                `json:"format,omitempty"` // e.g. "email", "binary"
	Minimum     *float64              `json:"minimum,omitempty"`
	Maximum     *float64              `json:"maximum,omitempty"`
	Properties  map[string]*SchemaRule `json:"properties,omitempty"`
	Items       *SchemaRule           `json:"items,omitempty"`
	MaxSizeBytes int64                `json:"maxSizeBytes,omitempty"`
	MimeTypes   []string              `json:"mimeTypes,omitempty"`
	Required    []string              `json:"required,omitempty"`
	Default     any                   `json:"default,omitempty"`
}

// Fluent Builder Methods

func String() *SchemaRule {
	return &SchemaRule{Type: TypeString}
}

func Number() *SchemaRule {
	return &SchemaRule{Type: TypeNumber}
}

func Integer() *SchemaRule {
	return &SchemaRule{Type: TypeInteger}
}

func Boolean() *SchemaRule {
	return &SchemaRule{Type: TypeBoolean}
}

func File() *SchemaRule {
	return &SchemaRule{Type: TypeFile, Format: "binary"}
}

func Array(itemRule *SchemaRule) *SchemaRule {
	return &SchemaRule{
		Type:  TypeArray,
		Items: itemRule,
	}
}

func Object(props map[string]*SchemaRule) *SchemaRule {
	required := make([]string, 0)
	for k, v := range props {
		if !v.Optional {
			required = append(required, k)
		}
	}
	return &SchemaRule{
		Type:       TypeObject,
		Properties: props,
		Required:   required,
	}
}

func Optional(rule *SchemaRule) *SchemaRule {
	clone := *rule
	clone.Optional = true
	return &clone
}

func (r *SchemaRule) Describe(desc string) *SchemaRule {
	r.Description = desc
	return r
}

func (r *SchemaRule) WithDefault(val any) *SchemaRule {
	r.Default = val
	return r
}

func (r *SchemaRule) Min(min int) *SchemaRule {
	r.MinLength = &min
	return r
}

func (r *SchemaRule) Max(max int) *SchemaRule {
	r.MaxLength = &max
	return r
}

// MinFiles sets the minimum file count constraint on an array of files
func (r *SchemaRule) MinFiles(min int) *SchemaRule {
	r.MinLength = &min
	return r
}

// MaxFiles sets the maximum file count constraint on an array of files
func (r *SchemaRule) MaxFiles(max int) *SchemaRule {
	r.MaxLength = &max
	return r
}

func (r *SchemaRule) Email() *SchemaRule {
	r.Format = "email"
	return r
}

func (r *SchemaRule) MinValue(min float64) *SchemaRule {
	r.Minimum = &min
	return r
}

func (r *SchemaRule) MaxValue(max float64) *SchemaRule {
	r.Maximum = &max
	return r
}

func (r *SchemaRule) MaxSize(bytes int64) *SchemaRule {
	r.MaxSizeBytes = bytes
	return r
}

func (r *SchemaRule) AllowedMimes(mimes ...string) *SchemaRule {
	r.MimeTypes = mimes
	return r
}

// FromStruct derives a SchemaRule automatically from a standard Go struct definition
func FromStruct(s any) *SchemaRule {
	t := reflect.TypeOf(s)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return Object(map[string]*SchemaRule{})
	}

	props := make(map[string]*SchemaRule)
	required := make([]string, 0)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}

		fieldName := field.Name
		isOptional := false
		if jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" {
				fieldName = parts[0]
			}
			for _, p := range parts[1:] {
				if p == "omitempty" {
					isOptional = true
				}
			}
		}

		desc := field.Tag.Get("doc")
		if desc == "" {
			desc = field.Tag.Get("description")
		}

		rule := ruleFromType(field.Type)
		rule.Description = desc
		rule.Optional = isOptional

		if minStr := field.Tag.Get("min"); minStr != "" {
			var minVal int
			if _, err := fmt.Sscanf(minStr, "%d", &minVal); err == nil {
				rule.MinLength = &minVal
			}
		}
		if minFilesStr := field.Tag.Get("min_files"); minFilesStr != "" {
			var minFilesVal int
			if _, err := fmt.Sscanf(minFilesStr, "%d", &minFilesVal); err == nil {
				rule.MinLength = &minFilesVal
			}
		}
		if maxStr := field.Tag.Get("max"); maxStr != "" {
			var maxVal int
			if _, err := fmt.Sscanf(maxStr, "%d", &maxVal); err == nil {
				rule.MaxLength = &maxVal
			}
		}
		if maxFilesStr := field.Tag.Get("max_files"); maxFilesStr != "" {
			var maxFilesVal int
			if _, err := fmt.Sscanf(maxFilesStr, "%d", &maxFilesVal); err == nil {
				rule.MaxLength = &maxFilesVal
			}
		}
		if maxSizeStr := field.Tag.Get("max_size"); maxSizeStr != "" {
			var maxSizeVal int64
			if _, err := fmt.Sscanf(maxSizeStr, "%d", &maxSizeVal); err == nil {
				if rule.Type == TypeArray && rule.Items != nil && rule.Items.Type == TypeFile {
					rule.Items.MaxSizeBytes = maxSizeVal
				} else {
					rule.MaxSizeBytes = maxSizeVal
				}
			}
		}
		if mimesStr := field.Tag.Get("mimes"); mimesStr != "" {
			parts := strings.Split(mimesStr, ",")
			for i, p := range parts {
				parts[i] = strings.TrimSpace(p)
			}
			if rule.Type == TypeArray && rule.Items != nil && rule.Items.Type == TypeFile {
				rule.Items.MimeTypes = parts
			} else {
				rule.MimeTypes = parts
			}
		}

		if !isOptional {
			required = append(required, fieldName)
		}
		props[fieldName] = rule
	}

	return &SchemaRule{
		Type:       TypeObject,
		Properties: props,
		Required:   required,
	}
}

func ruleFromType(t reflect.Type) *SchemaRule {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	// Check if this is rpc.UploadedFile
	if t.Name() == "UploadedFile" || t.String() == "rpc.UploadedFile" {
		return File()
	}

	switch t.Kind() {
	case reflect.String:
		return String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return Integer()
	case reflect.Float32, reflect.Float64:
		return Number()
	case reflect.Bool:
		return Boolean()
	case reflect.Slice, reflect.Array:
		elemRule := ruleFromType(t.Elem())
		return Array(elemRule)
	case reflect.Struct:
		inst := reflect.New(t).Interface()
		return FromStruct(inst)
	default:
		return &SchemaRule{Type: RuleType(fmt.Sprintf("%v", t.Kind()))}
	}
}

// CoerceMultipartData coerces string values in data to numbers or booleans if the schema rule requires it
func CoerceMultipartData(rule *SchemaRule, data map[string]any) {
	if rule == nil || data == nil || rule.Properties == nil {
		return
	}

	for k, prop := range rule.Properties {
		val, exists := data[k]
		if !exists || val == nil {
			continue
		}

		s, isStr := val.(string)
		if !isStr {
			if prop.Type == TypeObject && prop.Properties != nil {
				if nestedMap, ok := val.(map[string]any); ok {
					CoerceMultipartData(prop, nestedMap)
				}
			} else if prop.Type == TypeArray && prop.Items != nil {
				if slice, ok := val.([]any); ok {
					for idx, item := range slice {
						if itemMap, ok := item.(map[string]any); ok {
							CoerceMultipartData(prop.Items, itemMap)
						} else if sItem, isStr := item.(string); isStr {
							switch prop.Items.Type {
							case TypeBoolean:
								trimmed := strings.ToLower(strings.TrimSpace(sItem))
								if trimmed == "true" || trimmed == "1" {
									slice[idx] = true
								} else if trimmed == "false" || trimmed == "0" {
									slice[idx] = false
								}
							case TypeInteger:
								if iv, err := strconv.ParseInt(strings.TrimSpace(sItem), 10, 64); err == nil {
									slice[idx] = iv
								}
							case TypeNumber:
								if fv, err := strconv.ParseFloat(strings.TrimSpace(sItem), 64); err == nil {
									slice[idx] = fv
								}
							}
						}
					}
				}
			}
			continue
		}

		switch prop.Type {
		case TypeBoolean:
			trimmed := strings.ToLower(strings.TrimSpace(s))
			if trimmed == "true" || trimmed == "1" {
				data[k] = true
			} else if trimmed == "false" || trimmed == "0" {
				data[k] = false
			}
		case TypeInteger:
			trimmed := strings.TrimSpace(s)
			if iv, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
				data[k] = iv
			}
		case TypeNumber:
			trimmed := strings.TrimSpace(s)
			if fv, err := strconv.ParseFloat(trimmed, 64); err == nil {
				data[k] = fv
			}
		}
	}
}
