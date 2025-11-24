package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KennyKeni/elysia/types"
	"github.com/google/jsonschema-go/jsonschema"
)

type RunContext[TDep any] struct {
	Deps     TDep
	Messages []types.Message
	Usage    types.Usage
}

type Tool[TDep any] struct {
	types.ToolDefinition
	Execute func(ctx context.Context, rc *RunContext[TDep], args map[string]any) (*types.ToolResult, error)
}

func NewTool[TDep, TIn, TOut any](
	name, description string,
	handler func(context.Context, *RunContext[TDep], TIn) (TOut, error),
) (*Tool[TDep], error) {
	inputSchema, err := jsonschema.For[TIn](nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate input schema: %w", err)
	}

	outputSchema, err := jsonschema.For[TOut](nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate output schema: %w", err)
	}

	resolvedInputSchema, err := inputSchema.Resolve(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve input schema: %w", err)
	}

	resolvedOutputSchema, err := outputSchema.Resolve(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve output schema: %w", err)
	}

	validateAndExecute := func(ctx context.Context, rc *RunContext[TDep], args map[string]any) (*types.ToolResult, error) {
		// Validate input against the schema
		if err := resolvedInputSchema.Validate(args); err != nil {
			return &types.ToolResult{
				ContentPart: []types.ContentPart{
					types.NewContentPartText(fmt.Sprintf("Input validation error: %v", err)),
				},
				IsError: true,
			}, nil
		}

		// Marshals into a JSON string
		argsBytes, err := json.Marshal(args)
		if err != nil {
			return &types.ToolResult{
				ContentPart: []types.ContentPart{
					types.NewContentPartText(fmt.Sprintf("Failed to marshal input: %v", err)),
				},
				IsError: true,
			}, nil
		}

		// Unmarshal into TIn
		var typedInput TIn
		if err := json.Unmarshal(argsBytes, &typedInput); err != nil {
			return &types.ToolResult{
				ContentPart: []types.ContentPart{
					types.NewContentPartText(fmt.Sprintf("Failed to parse input: %v", err)),
				},
				IsError: true,
			}, nil
		}

		// Run handler
		output, err := handler(ctx, rc, typedInput)
		if err != nil {
			return &types.ToolResult{
				ContentPart: []types.ContentPart{
					types.NewContentPartText(fmt.Sprintf("Execution error: %v", err)),
				},
				IsError: true,
			}, nil
		}

		// Validate input against the outputSchema
		if err := resolvedOutputSchema.Validate(output); err != nil {
			return &types.ToolResult{
				ContentPart: []types.ContentPart{
					types.NewContentPartText(fmt.Sprintf("Output validation error: %v", err)),
				},
				IsError: true,
			}, nil
		}

		outputJSON, err := json.Marshal(output)
		if err != nil {
			return &types.ToolResult{
				ContentPart: []types.ContentPart{
					types.NewContentPartText(fmt.Sprintf("Failed to marshal output: %v", err)),
				},
				IsError: true,
			}, nil
		}

		return &types.ToolResult{
			ContentPart: []types.ContentPart{
				types.NewContentPartText(string(outputJSON)),
			},
			StructuredContent: output,
			IsError:           false,
		}, nil
	}

	var inputSchemaMap, outputSchemaMap map[string]any

	inputSchemaBytes, err := json.Marshal(inputSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input schema: %w", err)
	}
	if err := json.Unmarshal(inputSchemaBytes, &inputSchemaMap); err != nil {
		return nil, fmt.Errorf("failed to convert input schema to map: %w", err)
	}

	outputSchemaBytes, err := json.Marshal(outputSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal output schema: %w", err)
	}
	if err := json.Unmarshal(outputSchemaBytes, &outputSchemaMap); err != nil {
		return nil, fmt.Errorf("failed to convert output schema to map: %w", err)
	}

	return &Tool[TDep]{
		ToolDefinition: types.ToolDefinition{
			Name:         name,
			Description:  description,
			InputSchema:  inputSchemaMap,
			OutputSchema: outputSchemaMap,
		},
		Execute: validateAndExecute,
	}, nil
}
