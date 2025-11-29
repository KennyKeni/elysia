package openai

import (
	"fmt"

	"github.com/KennyKeni/elysia/types"
	"github.com/openai/openai-go/v3"
)

// ToToolDefinitions converts unified tool definitions to OpenAI tool parameters
func ToToolDefinitions(toolDefinitions []types.ToolDefinition) ([]openai.ChatCompletionToolUnionParam, error) {
	result := make([]openai.ChatCompletionToolUnionParam, 0, len(toolDefinitions))

	for _, definition := range toolDefinitions {
		toolParam, err := toToolDefinitionParam(definition)
		if err != nil {
			return nil, fmt.Errorf("error converting tool %s: %w", definition.Name, err)
		}
		result = append(result, toolParam)
	}

	return result, nil
}

// toToolDefinitionParam converts a single tool definition to OpenAI tool parameter
func toToolDefinitionParam(tool types.ToolDefinition) (openai.ChatCompletionToolUnionParam, error) {
	// InputSchema is already map[string]any, just use it directly
	if tool.InputSchema == nil {
		return openai.ChatCompletionToolUnionParam{}, fmt.Errorf("tool %s has nil input schema", tool.Name)
	}

	return openai.ChatCompletionToolUnionParam{
		OfFunction: &openai.ChatCompletionFunctionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        tool.Name,
				Description: openai.String(tool.Description),
				Parameters:  openai.FunctionParameters(tool.InputSchema),
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
