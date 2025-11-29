package types

import (
	"context"
	"encoding/json/v2"
	"fmt"
)

// ToolDefinition is metadata describing a tool for the LLM
// The client sends these to the LLM, but does not execute tools
// Execution is handled by the caller (agent layer or manual)
type ToolDefinition struct {
	Name         string
	Description  string
	InputSchema  map[string]any
	OutputSchema map[string]any
}

type Execute func(ctx context.Context, args map[string]any) (*ToolResult, error)

type Tool struct {
	ToolDefinition
	Execute Execute
}

func NewTool[TIn, TOut any](
	name, description string,
	handler func(context.Context, TIn) (TOut, error),
) (*Tool, error) {
	resolvedInputSchema, err := ResolveSchemaFor[TIn]()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve input schema: %w", err)
	}

	resolvedOutputSchema, err := ResolveSchemaFor[TOut]()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve output schema: %w", err)
	}

	inputSchemaMap, err := SchemaMapFor[TIn]()
	if err != nil {
		return nil, fmt.Errorf("failed to generate input schema map: %w", err)
	}

	outputSchemaMap, err := SchemaMapFor[TOut]()
	if err != nil {
		return nil, fmt.Errorf("failed to generate output schema map: %w", err)
	}

	validateAndExecute := func(ctx context.Context, args map[string]any) (*ToolResult, error) {
		// Validate input against the schema
		if err := Validate(resolvedInputSchema, args); err != nil {
			return ToolResultFromError(fmt.Errorf("input validation error: %w", err)), nil
		}

		// Unmarshal args into typed input
		typedInput, err := UnmarshalToolArgs[TIn](args)
		if err != nil {
			return ToolResultFromError(err), nil
		}

		// Run handler
		output, err := handler(ctx, typedInput)
		if err != nil {
			return ToolResultFromError(fmt.Errorf("execution error: %w", err)), nil
		}

		// Validate output against the schema
		if err := Validate(resolvedOutputSchema, output); err != nil {
			return ToolResultFromError(fmt.Errorf("output validation error: %w", err)), nil
		}

		// Marshal output to ToolResult
		outputJSON, err := json.Marshal(output)
		if err != nil {
			return ToolResultFromError(fmt.Errorf("failed to marshal output: %w", err)), nil
		}

		return &ToolResult{
			ContentPart: []ContentPart{
				NewContentPartText(string(outputJSON)),
			},
			StructuredContent: output,
			IsError:           false,
		}, nil
	}

	return &Tool{
		ToolDefinition: ToolDefinition{
			Name:         name,
			Description:  description,
			InputSchema:  inputSchemaMap,
			OutputSchema: outputSchemaMap,
		},
		Execute: validateAndExecute,
	}, nil
}

type ToolResult struct {
	ContentPart       []ContentPart
	StructuredContent any
	IsError           bool
}

type ToolResultOption func(*ToolResult)

// WithToolText Appends ContentPartText to tool
func WithToolText(text string) ToolResultOption {
	return func(t *ToolResult) {
		t.ContentPart = append(t.ContentPart, &ContentPartText{Text: text})
	}
}

func WithToolImage(data string) ToolResultOption {
	return func(t *ToolResult) {
		t.ContentPart = append(t.ContentPart, &ContentPartImage{Data: data})
	}
}

func WithStructuredContent(content any) ToolResultOption {
	return func(t *ToolResult) {
		t.StructuredContent = content
	}
}

func NewToolResult(opts ...ToolResultOption) *ToolResult {
	t := &ToolResult{ContentPart: make([]ContentPart, 0)}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// NewToolResultMessage converts a ToolResult to a tool Message
// This is a convenience helper for creating tool response messages from tool execution results
func NewToolResultMessage(toolCallID string, result *ToolResult) Message {
	return Message{
		Role:        RoleTool,
		ContentPart: result.ContentPart,
		ToolCallID:  &toolCallID,
	}
}

// ToolResultFromError converts any error to a ToolResult for LLM consumption
func ToolResultFromError(err error) *ToolResult {
	return &ToolResult{
		ContentPart: []ContentPart{NewContentPartText(err.Error())},
		IsError:     true,
	}
}

// UnmarshalToolArgs converts map[string]any args to a typed value
func UnmarshalToolArgs[T any](args map[string]any) (T, error) {
	var result T

	argsBytes, err := json.Marshal(args)
	if err != nil {
		return result, fmt.Errorf("failed to marshal args: %w", err)
	}

	if err := json.Unmarshal(argsBytes, &result); err != nil {
		return result, fmt.Errorf("failed to unmarshal args: %w", err)
	}

	return result, nil
}
