package types

import (
	"encoding/json/v2"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
)

func isValidJSON(s string) bool {
	var js any
	return json.Unmarshal([]byte(s), &js) == nil
}

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

// Validate validates a value against a resolved schema
func Validate(resolved *jsonschema.Resolved, value any) error {
	return resolved.Validate(value)
}

// ValidateJSONString parses a JSON string and validates it against a schema map
func ValidateJSONString(content string, schema map[string]any) error {
	// Parse the content as JSON
	var parsed any
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	// Convert schema map to jsonschema and resolve
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %w", err)
	}

	var schemaObj jsonschema.Schema
	if err := json.Unmarshal(schemaBytes, &schemaObj); err != nil {
		return fmt.Errorf("failed to parse schema: %w", err)
	}

	resolved, err := schemaObj.Resolve(nil)
	if err != nil {
		return fmt.Errorf("failed to resolve schema: %w", err)
	}

	// Validate
	if err := resolved.Validate(parsed); err != nil {
		return err
	}

	return nil
}
