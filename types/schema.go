package types

import (
	"encoding/json/v2"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
)

// ResolveSchemaFor generates and resolves a JSON schema from a Go type
func ResolveSchemaFor[T any]() (*jsonschema.Resolved, error) {
	schema, err := jsonschema.For[T](nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate schema: %w", err)
	}

	resolved, err := schema.Resolve(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve schema: %w", err)
	}

	return resolved, nil
}

// SchemaMapFor generates a JSON schema map from a Go type
func SchemaMapFor[T any]() (map[string]any, error) {
	schema, err := jsonschema.For[T](nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate schema: %w", err)
	}

	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema: %w", err)
	}

	var schemaMap map[string]any
	if err := json.Unmarshal(schemaBytes, &schemaMap); err != nil {
		return nil, fmt.Errorf("failed to convert schema to map: %w", err)
	}

	return schemaMap, nil
}

// Validate validates any value against a resolved schema, returning a ToolResult error if invalid
func Validate(resolved *jsonschema.Resolved, value any) *ToolResult {
	if err := resolved.Validate(value); err != nil {
		return &ToolResult{
			ContentPart: []ContentPart{
				NewContentPartText(fmt.Sprintf("Validation error: %v", err)),
			},
			IsError: true,
		}
	}
	return nil
}

// UnmarshalArgs converts map[string]any args to a typed value, returning a ToolResult error if it fails
func UnmarshalArgs[T any](args map[string]any) (T, *ToolResult) {
	var result T

	argsBytes, err := json.Marshal(args)
	if err != nil {
		return result, &ToolResult{
			ContentPart: []ContentPart{
				NewContentPartText(fmt.Sprintf("Failed to marshal input: %v", err)),
			},
			IsError: true,
		}
	}

	if err := json.Unmarshal(argsBytes, &result); err != nil {
		return result, &ToolResult{
			ContentPart: []ContentPart{
				NewContentPartText(fmt.Sprintf("Failed to parse input: %v", err)),
			},
			IsError: true,
		}
	}

	return result, nil
}
