package rpc

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/CellsWeb/crpc-go/pkg/schema"
)

// FunctionConfig provides type-safe configuration for registering an RPC function
type FunctionConfig[TInput any, TOutput any] struct {
	Namespace    string
	FunctionName string
	ContentType  string // "application/json" (default) or "multipart/form-data"
	Metadata     FunctionMetadata
	InputSchema  *schema.SchemaRule
	OutputSchema *schema.SchemaRule
	Middlewares  []MiddlewareAttachment
	Handler      func(ctx *Context, input TInput) (*TOutput, error)
}

// Register registers a type-safe RPC function into the global or local registry
func Register[TInput any, TOutput any](cfg FunctionConfig[TInput, TOutput]) *FunctionDefinition {
	var zeroInput TInput
	inputType := reflect.TypeOf(zeroInput)

	// Auto-infer input schema if not explicitly provided
	inSchema := cfg.InputSchema
	if inSchema == nil {
		inSchema = schema.FromStruct(zeroInput)
	}

	var zeroOutput TOutput
	outSchema := cfg.OutputSchema
	if outSchema == nil {
		outSchema = schema.FromStruct(zeroOutput)
	}

	cType := cfg.ContentType
	if cType == "" {
		cType = "application/json"
	}

	wrapped := func(ctx *Context, rawInput any) (any, error) {
		var input TInput

		// If input is already of type TInput (e.g. from in-process caller)
		// Otherwise marshal map -> json -> struct
		if rawInput != nil {
			if cType == "multipart/form-data" {
				if rawMap, ok := rawInput.(map[string]any); ok {
					rawInput = coerceRawMapToType(inputType, rawMap)
				}
			}
			bytes, err := json.Marshal(rawInput)
			if err != nil {
				return nil, NewError(ErrInvalidBody, fmt.Sprintf("Failed to encode input: %v", err))
			}
			if err := json.Unmarshal(bytes, &input); err != nil {
				return nil, NewError(ErrInvalidBody, fmt.Sprintf("Failed to unmarshal into input struct: %v", err))
			}
		}

		// Inject any UploadedFiles if TInput contains UploadedFile pointers
		if rawMap, ok := rawInput.(map[string]any); ok {
			injectFilesIntoStruct(&input, rawMap)
		}

		return cfg.Handler(ctx, input)
	}

	def := &FunctionDefinition{
		Namespace:    cfg.Namespace,
		FunctionName: cfg.FunctionName,
		ContentType:  cType,
		Metadata:     cfg.Metadata,
		InputSchema:  inSchema,
		OutputSchema: outSchema,
		Middlewares:  cfg.Middlewares,
		Handler:      wrapped,
		InputType:    inputType,
	}

	RegisterFunction(def)
	return def
}

// injectFilesIntoStruct copies *UploadedFile and []*UploadedFile from raw input map to struct fields matching json tags
func injectFilesIntoStruct(target any, raw map[string]any) {
	if raw == nil || target == nil {
		return
	}
	val := reflect.ValueOf(target)
	if val.Kind() == reflect.Pointer {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return
	}

	t := val.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}
		fieldName := field.Name
		if jsonTag != "" {
			parts := stringsSplit(jsonTag, ",")
			if parts[0] != "" {
				fieldName = parts[0]
			}
		}

		fieldVal := val.Field(i)
		if !fieldVal.CanSet() {
			continue
		}

		if rawVal, exists := raw[fieldName]; exists {
			// 1. Single *UploadedFile
			if file, ok := rawVal.(*UploadedFile); ok {
				if fieldVal.Type() == reflect.TypeOf((*UploadedFile)(nil)) {
					fieldVal.Set(reflect.ValueOf(file))
				}
				continue
			}

			// 2. Slice of []*UploadedFile
			if fieldVal.Type() == reflect.TypeOf(([]*UploadedFile)(nil)) {
				var fileList []*UploadedFile
				if slice, ok := rawVal.([]*UploadedFile); ok {
					fileList = slice
				} else if slice, ok := rawVal.([]any); ok {
					for _, item := range slice {
						if f, ok := item.(*UploadedFile); ok {
							fileList = append(fileList, f)
						}
					}
				}
				if len(fileList) > 0 {
					fieldVal.Set(reflect.ValueOf(fileList))
				}
				continue
			}

			// 2b. Mixed slice []any containing *UploadedFile
			if fieldVal.Type() == reflect.TypeOf(([]any)(nil)) {
				if slice, ok := rawVal.([]any); ok {
					hasFile := false
					for _, item := range slice {
						if _, ok := item.(*UploadedFile); ok {
							hasFile = true
							break
						}
					}
					if hasFile {
						fieldVal.Set(reflect.ValueOf(slice))
					}
				} else if file, ok := rawVal.(*UploadedFile); ok {
					fieldVal.Set(reflect.ValueOf([]any{file}))
				}
				continue
			}

			// 2c. Interface / any field holding an UploadedFile
			if fieldVal.Kind() == reflect.Interface {
				if file, ok := rawVal.(*UploadedFile); ok {
					fieldVal.Set(reflect.ValueOf(file))
					continue
				}
			}

			// 3. Nested struct or pointer to struct
			if nestedMap, ok := rawVal.(map[string]any); ok {
				if fieldVal.Kind() == reflect.Struct {
					injectFilesIntoStruct(fieldVal.Addr().Interface(), nestedMap)
				} else if fieldVal.Kind() == reflect.Pointer && !fieldVal.IsNil() && fieldVal.Elem().Kind() == reflect.Struct {
					injectFilesIntoStruct(fieldVal.Interface(), nestedMap)
				}
			}
		}
	}
}

// coerceRawMapToType aligns raw map primitive strings (from multipart or loose JSON) to match target struct field types
func coerceRawMapToType(t reflect.Type, raw map[string]any) map[string]any {
	if raw == nil || t == nil {
		return raw
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return raw
	}

	result := make(map[string]any, len(raw))
	for k, v := range raw {
		result[k] = v
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}
		fieldName := field.Name
		if jsonTag != "" {
			parts := stringsSplit(jsonTag, ",")
			if parts[0] != "" {
				fieldName = parts[0]
			}
		}

		val, exists := result[fieldName]
		if !exists || val == nil {
			continue
		}

		ft := field.Type
		if ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}

		switch ft.Kind() {
		case reflect.Bool:
			if s, ok := val.(string); ok {
				trimmed := strings.ToLower(strings.TrimSpace(s))
				if trimmed == "true" || trimmed == "1" {
					result[fieldName] = true
				} else if trimmed == "false" || trimmed == "0" {
					result[fieldName] = false
				}
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if s, ok := val.(string); ok {
				if iv, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
					result[fieldName] = iv
				}
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if s, ok := val.(string); ok {
				if uv, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64); err == nil {
					result[fieldName] = uv
				}
			}
		case reflect.Float32, reflect.Float64:
			if s, ok := val.(string); ok {
				if fv, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
					result[fieldName] = fv
				}
			}
		case reflect.Struct:
			if nestedMap, ok := val.(map[string]any); ok {
				result[fieldName] = coerceRawMapToType(ft, nestedMap)
			}
		}
	}
	return result
}

func stringsSplit(s, sep string) []string {
	var parts []string
	start := 0
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			parts = append(parts, s[start:i])
			start = i + len(sep)
			i = start - 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}
