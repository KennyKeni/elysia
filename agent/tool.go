package agent

import (
	"context"
	json "encoding/json/v2"
	"fmt"

	"github.com/KennyKeni/elysia/types"
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

// WrapTool wraps a types.Tool (MCP, external tools) into an agent.Tool
func WrapTool[TDep any](tool *types.Tool) *Tool[TDep] {
	return &Tool[TDep]{
		ToolDefinition: tool.ToolDefinition,
		Execute: func(ctx context.Context, rc *RunContext[TDep], args map[string]any) (*types.ToolResult, error) {
			return tool.Execute(ctx, args)
		},
	}
}

// NewTool creates an agent tool with typed input/output and RunContext access
func NewTool[TDep, TIn, TOut any](
	name, description string,
	handler func(context.Context, *RunContext[TDep], TIn) (TOut, error),
) (*Tool[TDep], error) {
	resolvedInputSchema, err := types.ResolveSchemaFor[TIn]()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve input schema: %w", err)
	}

	resolvedOutputSchema, err := types.ResolveSchemaFor[TOut]()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve output schema: %w", err)
	}

	inputSchemaMap, err := types.SchemaMapFor[TIn]()
	if err != nil {
		return nil, fmt.Errorf("failed to generate input schema map: %w", err)
	}

	outputSchemaMap, err := types.SchemaMapFor[TOut]()
	if err != nil {
		return nil, fmt.Errorf("failed to generate output schema map: %w", err)
	}

	validateAndExecute := func(ctx context.Context, rc *RunContext[TDep], args map[string]any) (*types.ToolResult, error) {
		// Validate input against the schema
		if errResult := types.ValidateToolInput(resolvedInputSchema, args); errResult != nil {
			return errResult, nil
		}

		// Unmarshal args into typed input
		typedInput, errResult := types.UnmarshalToolArgs[TIn](args)
		if errResult != nil {
			return errResult, nil
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

		// Validate output against the schema
		if errResult := types.ValidateToolInput(resolvedOutputSchema, output); errResult != nil {
			return errResult, nil
		}

		// Marshal output to ToolResult
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

func GetToolDefinitions[TDep any](agentTools []Tool[TDep]) []types.ToolDefinition {
	res := make([]types.ToolDefinition, len(agentTools))

	for i, tool := range agentTools {
		res[i] = tool.ToolDefinition
	}

	return res
}
