package openai

import (
	"encoding/json"
	"fmt"

	"github.com/KennyKeni/elysia/types"
	"github.com/openai/openai-go/v3"
)

// ToTools converts unified tool definitions to OpenAI tool parameters
func ToTools(tools []types.Tool) ([]openai.ChatCompletionToolUnionParam, error) {
	result := make([]openai.ChatCompletionToolUnionParam, 0, len(tools))

	for _, tool := range tools {
		toolParam, err := toToolDefinitionParam(tool)
		if err != nil {
			return nil, fmt.Errorf("error converting tool %s: %w", tool.Name(), err)
		}
		result = append(result, toolParam)
	}

	return result, nil
}

// toToolDefinitionParam converts a single tool definition to OpenAI tool parameter
func toToolDefinitionParam(tool types.Tool) (openai.ChatCompletionToolUnionParam, error) {
	// Convert the input schema to map[string]interface{}
	var parameters map[string]interface{}
	if tool.InputSchema() != nil {
		// InputSchema() returns any, which could be a schema struct or already a map
		// We need to marshal it to JSON first, then unmarshal to map[string]interface{}
		schemaJSON, err := json.Marshal(tool.InputSchema())
		if err != nil {
			return openai.ChatCompletionToolUnionParam{}, fmt.Errorf("failed to marshal input schema: %w", err)
		}
		if err := json.Unmarshal(schemaJSON, &parameters); err != nil {
			return openai.ChatCompletionToolUnionParam{}, fmt.Errorf("failed to unmarshal input schema: %w", err)
		}
	}

	return openai.ChatCompletionToolUnionParam{
		OfFunction: &openai.ChatCompletionFunctionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        tool.Name(),
				Description: openai.String(tool.Description()),
				Parameters:  openai.FunctionParameters(parameters),
			},
		},
	}, nil
}

// ToToolChoice converts unified ToolChoice to OpenAI tool choice parameter
func ToToolChoice(toolChoice *types.ToolChoice) openai.ChatCompletionToolChoiceOptionUnionParam {
	if toolChoice == nil {
		// Default to auto if not specified
		return openai.ChatCompletionToolChoiceOptionUnionParam{
			OfAuto: openai.String(string(openai.ChatCompletionToolChoiceOptionAutoAuto)),
		}
	}

	switch toolChoice.Mode {
	case types.ToolChoiceModeAuto:
		return openai.ChatCompletionToolChoiceOptionUnionParam{
			OfAuto: openai.String(string(openai.ChatCompletionToolChoiceOptionAutoAuto)),
		}

	case types.ToolChoiceModeNone:
		return openai.ChatCompletionToolChoiceOptionUnionParam{
			OfAuto: openai.String(string(openai.ChatCompletionToolChoiceOptionAutoNone)),
		}

	case types.ToolChoiceModeRequired:
		return openai.ChatCompletionToolChoiceOptionUnionParam{
			OfAuto: openai.String(string(openai.ChatCompletionToolChoiceOptionAutoRequired)),
		}

	case types.ToolChoiceModeTool:
		// Force a specific tool by name
		return openai.ToolChoiceOptionFunctionToolChoice(
			openai.ChatCompletionNamedToolChoiceFunctionParam{
				Name: toolChoice.Name,
			},
		)

	default:
		// Fallback to auto
		return openai.ChatCompletionToolChoiceOptionUnionParam{
			OfAuto: openai.String(string(openai.ChatCompletionToolChoiceOptionAutoAuto)),
		}
	}
}
