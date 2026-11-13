package schema

import (
	"fmt"
	"regexp"
	"strings"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// Validate checks a generic payload map against a SchemaRule, enforcing strict contracts
func Validate(rule *SchemaRule, data any, currentPath string) []ValidationErrorDetail {
	if rule == nil {
		return nil
	}

	if currentPath == "" {
		currentPath = "/"
	}

	var details []ValidationErrorDetail

	// If data is nil
	if data == nil {
		if !rule.Optional {
			details = append(details, ValidationErrorDetail{
				Path:    currentPath,
				Message: "Expected non-null value, got null",
			})
		}
		return details
	}

	switch rule.Type {
	case TypeObject:
		objMap, ok := data.(map[string]any)
		if !ok {
			details = append(details, ValidationErrorDetail{
				Path:    currentPath,
				Message: fmt.Sprintf("Expected object, got %T", data),
			})
			return details
		}

		// Check for unknown fields (strict mode)
		if rule.Properties != nil {
			for key := range objMap {
				if _, allowed := rule.Properties[key]; !allowed {
					childPath := appendPath(currentPath, key)
					details = append(details, ValidationErrorDetail{
						Path:    childPath,
						Message: fmt.Sprintf("Unknown field %q is not allowed by contract", key),
					})
				}
			}

			// Check all defined properties
			for propName, propRule := range rule.Properties {
				val, exists := objMap[propName]
				childPath := appendPath(currentPath, propName)

				if !exists || val == nil {
					if !propRule.Optional {
						details = append(details, ValidationErrorDetail{
							Path:    childPath,
							Message: fmt.Sprintf("Required property %q is missing", propName),
						})
					}
					continue
				}

				subErrors := Validate(propRule, val, childPath)
				details = append(details, subErrors...)
			}
		}

	case TypeString:
		str, ok := data.(string)
		if !ok {
			details = append(details, ValidationErrorDetail{
				Path:    currentPath,
				Message: fmt.Sprintf("Expected string, got %T", data),
			})
			return details
		}
		if rule.MinLength != nil && len(str) < *rule.MinLength {
			details = append(details, ValidationErrorDetail{
				Path:    currentPath,
				Message: fmt.Sprintf("String length must be at least %d characters", *rule.MinLength),
			})
		}
		if rule.MaxLength != nil && len(str) > *rule.MaxLength {
			details = append(details, ValidationErrorDetail{
				Path:    currentPath,
				Message: fmt.Sprintf("String length must be at most %d characters", *rule.MaxLength),
			})
		}
		if rule.Format == "email" && !emailRegex.MatchString(str) {
			details = append(details, ValidationErrorDetail{
				Path:    currentPath,
				Message: "Must be a valid email address",
			})
		}

	case TypeInteger, TypeNumber:
		var num float64
		switch v := data.(type) {
		case float64:
			num = v
		case float32:
			num = float64(v)
		case int:
			num = float64(v)
		case int64:
			num = float64(v)
		default:
			details = append(details, ValidationErrorDetail{
				Path:    currentPath,
				Message: fmt.Sprintf("Expected number, got %T", data),
			})
			return details
		}
		if rule.Minimum != nil && num < *rule.Minimum {
			details = append(details, ValidationErrorDetail{
				Path:    currentPath,
				Message: fmt.Sprintf("Value must be >= %v", *rule.Minimum),
			})
		}
		if rule.Maximum != nil && num > *rule.Maximum {
			details = append(details, ValidationErrorDetail{
				Path:    currentPath,
				Message: fmt.Sprintf("Value must be <= %v", *rule.Maximum),
			})
		}

	case TypeBoolean:
		if _, ok := data.(bool); !ok {
			details = append(details, ValidationErrorDetail{
				Path:    currentPath,
				Message: fmt.Sprintf("Expected boolean, got %T", data),
			})
		}

	case TypeArray:
		arr, ok := data.([]any)
		if !ok {
			details = append(details, ValidationErrorDetail{
				Path:    currentPath,
				Message: fmt.Sprintf("Expected array, got %T", data),
			})
			return details
		}
		if rule.Items != nil {
			for i, item := range arr {
				elemPath := fmt.Sprintf("%s/%d", currentPath, i)
				subErrors := Validate(rule.Items, item, elemPath)
				details = append(details, subErrors...)
			}
		}

	case TypeFile:
		uploaded, ok := data.(FileValidator)
		if !ok || uploaded == nil {
			details = append(details, ValidationErrorDetail{
				Path:    currentPath,
				Message: "Expected binary uploaded file",
			})
			return details
		}
		if rule.MaxSizeBytes > 0 && uploaded.Size() > rule.MaxSizeBytes {
			details = append(details, ValidationErrorDetail{
				Path:    currentPath,
				Message: fmt.Sprintf("File size %d bytes exceeds maximum limit of %d bytes", uploaded.Size(), rule.MaxSizeBytes),
			})
		}
		if len(rule.MimeTypes) > 0 {
			matched := false
			fileMime := strings.ToLower(uploaded.MimeType())
			for _, allowed := range rule.MimeTypes {
				if strings.EqualFold(allowed, fileMime) {
					matched = true
					break
				}
			}
			if !matched {
				details = append(details, ValidationErrorDetail{
					Path:    currentPath,
					Message: fmt.Sprintf("File MIME type %q not allowed; allowed types: %s", uploaded.MimeType(), strings.Join(rule.MimeTypes, ", ")),
				})
			}
		}
	}

	return details
}

func appendPath(base, child string) string {
	if base == "/" {
		return "/" + child
	}
	return base + "/" + child
}
