package nodebridge

import (
	"fmt"
	"strings"

	"github.com/CellsWeb/crpc-go/pkg/rpc"
	"github.com/CellsWeb/crpc-go/pkg/schema"
)

// GenerateTypeScriptDeclarations produces a strictly-typed TypeScript declaration file (.d.ts)
func GenerateTypeScriptDeclarations(functions map[string]*rpc.FunctionDefinition) string {
	var sb strings.Builder

	sb.WriteString(`/**
 * This file is generated using nodebridge.GenerateTypeScriptDeclarations.
 * DO NOT EDIT MANUALLY.
 */

`)

	// Generate individual input and output types
	for key, fn := range functions {
		safeKey := toTypeIdentifier(key)
		inType := ruleToTSType(fn.InputSchema, 0)
		outType := ruleToTSType(fn.OutputSchema, 0)

		sb.WriteString(fmt.Sprintf("export type %sInput = %s;\n\n", safeKey, inType))
		sb.WriteString(fmt.Sprintf("export type %sOutput = %s;\n\n", safeKey, outType))
	}

	// Generate the aggregate RpcContract map
	sb.WriteString("export interface RpcContractMap {\n")
	for key := range functions {
		safeKey := toTypeIdentifier(key)
		sb.WriteString(fmt.Sprintf("  %q: {\n    input: %sInput;\n    output: %sOutput;\n  };\n", key, safeKey, safeKey))
	}
	sb.WriteString("}\n")

	return sb.String()
}

func ruleToTSType(rule *schema.SchemaRule, indentLevel int) string {
	if rule == nil {
		return "Record<string, unknown>"
	}

	indent := strings.Repeat("  ", indentLevel)
	nextIndent := strings.Repeat("  ", indentLevel+1)

	switch rule.Type {
	case schema.TypeString:
		return "string"
	case schema.TypeInteger, schema.TypeNumber:
		return "number"
	case schema.TypeBoolean:
		return "boolean"
	case schema.TypeFile:
		return "File | Blob | { uri: string; name: string; type: string }"
	case schema.TypeArray:
		elemType := "unknown"
		if rule.Items != nil {
			elemType = ruleToTSType(rule.Items, indentLevel)
		}
		return fmt.Sprintf("Array<%s>", elemType)
	case schema.TypeObject:
		if rule.Properties == nil || len(rule.Properties) == 0 {
			return "Record<string, unknown>"
		}
		var fields strings.Builder
		fields.WriteString("{\n")
		for name, prop := range rule.Properties {
			optMark := ""
			if prop.Optional {
				optMark = "?"
			}
			fieldType := ruleToTSType(prop, indentLevel+1)
			docComment := ""
			if prop.Description != "" {
				docComment = fmt.Sprintf("%s/** %s */\n", nextIndent, prop.Description)
			}
			fields.WriteString(fmt.Sprintf("%s%s%s%s: %s;\n", docComment, nextIndent, name, optMark, fieldType))
		}
		fields.WriteString(indent + "}")
		return fields.String()
	default:
		return "unknown"
	}
}

func toTypeIdentifier(key string) string {
	parts := strings.FieldsFunc(key, func(r rune) bool {
		return r == '.' || r == '_' || r == '-'
	})
	var sb strings.Builder
	for _, p := range parts {
		if len(p) > 0 {
			sb.WriteString(strings.ToUpper(p[:1]) + p[1:])
		}
	}
	return sb.String()
}
