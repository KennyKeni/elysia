package types

import (
	"context"
	"encoding/json"

	"github.com/google/jsonschema-go/jsonschema"
)

type ToolHandler func(ctx context.Context, args map[string]any) (*ToolResult, error)

type Tool interface {
	Name() string
	Description() string
	InputSchema() any
	OutputSchema() any
	Execute(ctx context.Context, args map[string]any) (*ToolResult, error)
}

type ToolResult struct {
	ContentPart       []ContentPart
	StructuredContent any
	IsError           bool
}

type ToolOption func(*ToolResult)

func WithToolText(text string) ToolOption {
	return func(t *ToolResult) {
		t.ContentPart = append(t.ContentPart, &ContentPartText{Text: text})
	}
}

func WithToolImage(data string) ToolOption {
	return func(t *ToolResult) {
		t.ContentPart = append(t.ContentPart, &ContentPartImage{Data: data})
	}
}

func WithStructuredContent(content any) ToolOption {
	return func(t *ToolResult) {
		t.StructuredContent = content
	}
}

func NewToolResult(opts ...ToolOption) *ToolResult {
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

type NativeTool struct {
	name         string
	description  string
	inputSchema  any
	outputSchema any
	handler      ToolHandler
}

func (n *NativeTool) Name() string {
	return n.name
}

func (n *NativeTool) Description() string {
	return n.description
}

func (n *NativeTool) InputSchema() any {
	return n.inputSchema
}

func (n *NativeTool) OutputSchema() any {
	return n.outputSchema
}

func (n *NativeTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	return n.handler(ctx, args)
}

// NewNativeTool creates a new native tool with automatic schema generation from Go types
func NewNativeTool[TIn, TOut any](
	name, description string,
	handler func(ctx context.Context, args TIn) (TOut, error),
) (*NativeTool, error) {
	// Generate input schema using jsonschema.For
	inputSchema, err := jsonschema.For[TIn](nil)
	if err != nil {
		return nil, err
	}

	// Generate output schema using jsonschema.For
	outputSchema, err := jsonschema.For[TOut](nil)
	if err != nil {
		return nil, err
	}

	// Resolve schemas for validation
	resolvedInput, err := inputSchema.Resolve(nil)
	if err != nil {
		return nil, err
	}
	resolvedOutput, err := outputSchema.Resolve(nil)
	if err != nil {
		return nil, err
	}

	// Wrap the typed handler with validation
	wrappedHandler := func(ctx context.Context, args map[string]any) (*ToolResult, error) {
		// Validate input against schema
		if err := resolvedInput.Validate(args); err != nil {
			return nil, err
		}

		// Convert map to typed input
		argsJSON, err := json.Marshal(args)
		if err != nil {
			return nil, err
		}
		var typedArgs TIn
		if err := json.Unmarshal(argsJSON, &typedArgs); err != nil {
			return nil, err
		}

		// Execute handler
		result, err := handler(ctx, typedArgs)
		if err != nil {
			return &ToolResult{
				ContentPart:       []ContentPart{&ContentPartText{Text: err.Error()}},
				StructuredContent: nil,
				IsError:           true,
			}, nil
		}

		resultJSON, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}

		// Validate output
		var resultValue any
		if err := json.Unmarshal(resultJSON, &resultValue); err != nil {
			return nil, err
		}
		if err := resolvedOutput.Validate(resultValue); err != nil {
			return nil, err
		}

		return &ToolResult{
			ContentPart:       []ContentPart{&ContentPartText{Text: string(resultJSON)}},
			StructuredContent: result,
			IsError:           false,
		}, nil

	}

	return &NativeTool{
		name:         name,
		description:  description,
		inputSchema:  inputSchema,
		outputSchema: outputSchema,
		handler:      wrappedHandler,
	}, nil
}
