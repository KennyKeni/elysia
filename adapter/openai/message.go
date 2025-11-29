package openai

import (
	json "encoding/json/v2"
	"fmt"

	"github.com/KennyKeni/elysia/types"
	"github.com/openai/openai-go/v3"
)

// ToChatCompletionMessage converts unified messages to OpenAI chat completion message parameters
func ToChatCompletionMessage(systemPrompt string, messages []types.Message) ([]openai.ChatCompletionMessageParamUnion, error) {
	result := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages)+1)

	if systemPrompt != "" {
		result = append(result, openai.SystemMessage(systemPrompt))
	}

	for _, message := range messages {
		switch message.Role {
		case types.RoleUser:
			userMessage, err := toUserMessage(&message)
			if err != nil {
				return nil, fmt.Errorf("error converting message to UserMessage: %w", err)
			}
			result = append(result, userMessage)
		case types.RoleAssistant:
			assistantMessage, err := toAssistantMessage(&message)
			if err != nil {
				return nil, fmt.Errorf("error converting message to AssistantMessage: %w", err)
			}
			result = append(result, assistantMessage)
		case types.RoleTool:
			toolResultMessage, err := toToolResultMessage(&message)
			if err != nil {
				return nil, fmt.Errorf("error converting message to ToolResultMessage: %w", err)
			}
			result = append(result, toolResultMessage)
		default:
			return nil, fmt.Errorf("%w: %s", ErrUnsupportedMessageRole, message.Role)
		}
	}

	return result, nil
}

// toUserMessage converts a user message to OpenAI user message parameters
func toUserMessage(message *types.Message) (openai.ChatCompletionMessageParamUnion, error) {
	content := make([]openai.ChatCompletionContentPartUnionParam, 0, len(message.ContentPart))

	for _, contentPart := range message.ContentPart {
		switch part := contentPart.(type) {
		case *types.ContentPartText:
			content = append(content, toUserTextPart(part))
		case *types.ContentPartImage:
			content = append(content, toUserImageDataPart(part))
		case *types.ContentPartImageURL:
			content = append(content, toUserImageURLPart(part))
		default:
			return openai.ChatCompletionMessageParamUnion{}, fmt.Errorf("%w: %T", ErrUnsupportedUserContentPart, part)
		}
	}

	return openai.ChatCompletionMessageParamUnion{
		OfUser: &openai.ChatCompletionUserMessageParam{
			Content: openai.ChatCompletionUserMessageParamContentUnion{
				OfArrayOfContentParts: content,
			},
		},
	}, nil
}

// toAssistantMessage converts an assistant message with content and tool calls to OpenAI assistant message parameters
func toAssistantMessage(message *types.Message) (openai.ChatCompletionMessageParamUnion, error) {
	content := make([]openai.ChatCompletionAssistantMessageParamContentArrayOfContentPartUnion, 0, len(message.ContentPart))

	for _, contentPart := range message.ContentPart {
		switch part := contentPart.(type) {
		case *types.ContentPartText:
			content = append(content, toAssistantTextPart(part))
		case *types.ContentPartRefusal:
			content = append(content, toAssistantRefusalPart(part))
		default:
			return openai.ChatCompletionMessageParamUnion{}, fmt.Errorf("%w: %T", ErrUnsupportedAssistantContentPart, part)
		}
	}

	var toolCalls []openai.ChatCompletionMessageToolCallUnionParam
	if len(message.ToolCalls) > 0 {
		toolCalls = make([]openai.ChatCompletionMessageToolCallUnionParam, 0, len(message.ToolCalls))
		for i := range message.ToolCalls {
			tc, err := toToolCallParam(&message.ToolCalls[i])
			if err != nil {
				return openai.ChatCompletionMessageParamUnion{}, fmt.Errorf("error converting tool call param for assistant message: %w", err)
			}
			toolCalls = append(toolCalls, tc)
		}
	}

	return openai.ChatCompletionMessageParamUnion{
		OfAssistant: &openai.ChatCompletionAssistantMessageParam{
			Content: openai.ChatCompletionAssistantMessageParamContentUnion{
				OfArrayOfContentParts: content,
			},
			ToolCalls: toolCalls,
		},
	}, nil
}

// toToolResultMessage converts a tool result message to OpenAI tool message parameters
func toToolResultMessage(message *types.Message) (openai.ChatCompletionMessageParamUnion, error) {
	content := make([]openai.ChatCompletionContentPartTextParam, 0, len(message.ContentPart))

	for _, contentPart := range message.ContentPart {
		switch part := contentPart.(type) {
		case *types.ContentPartText:
			content = append(content, openai.ChatCompletionContentPartTextParam{
				Text: part.Text,
			})
		default:
			return openai.ChatCompletionMessageParamUnion{}, fmt.Errorf("%w: %T", ErrUnsupportedToolContentPart, part)
		}
	}

	if message.ToolCallID == nil {
		return openai.ChatCompletionMessageParamUnion{}, ErrMissingToolCallID
	}

	return openai.ChatCompletionMessageParamUnion{
		OfTool: &openai.ChatCompletionToolMessageParam{
			Content: openai.ChatCompletionToolMessageParamContentUnion{
				OfArrayOfContentParts: content,
			},
			ToolCallID: *message.ToolCallID,
		},
	}, nil
}

// toUserTextPart converts text content to OpenAI user message text part
func toUserTextPart(part *types.ContentPartText) openai.ChatCompletionContentPartUnionParam {
	return openai.TextContentPart(part.Text)
}

// toUserImageDataPart converts base64 image data to OpenAI user message image part with data URL format
func toUserImageDataPart(part *types.ContentPartImage) openai.ChatCompletionContentPartUnionParam {
	dataURL := fmt.Sprintf("data:image/png;base64,%s", part.Data)
	return openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
		URL:    dataURL,
		Detail: part.Detail,
	})
}

// toUserImageURLPart converts image URL to OpenAI user message image part
func toUserImageURLPart(part *types.ContentPartImageURL) openai.ChatCompletionContentPartUnionParam {
	return openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
		URL: part.URL,
	})
}

// toAssistantTextPart converts text content to OpenAI assistant message text part
func toAssistantTextPart(part *types.ContentPartText) openai.ChatCompletionAssistantMessageParamContentArrayOfContentPartUnion {
	return openai.ChatCompletionAssistantMessageParamContentArrayOfContentPartUnion{
		OfText: &openai.ChatCompletionContentPartTextParam{
			Text: part.Text,
		},
	}
}

// toAssistantRefusalPart converts refusal content to OpenAI assistant message refusal part
func toAssistantRefusalPart(part *types.ContentPartRefusal) openai.ChatCompletionAssistantMessageParamContentArrayOfContentPartUnion {
	return openai.ChatCompletionAssistantMessageParamContentArrayOfContentPartUnion{
		OfRefusal: &openai.ChatCompletionContentPartRefusalParam{
			Refusal: part.Refusal,
		},
	}
}

// toToolCallParam converts a tool call to OpenAI tool call parameters with marshaled arguments
func toToolCallParam(toolCall *types.ToolCall) (openai.ChatCompletionMessageToolCallUnionParam, error) {
	argsJSON, err := json.Marshal(toolCall.Function.Arguments)
	if err != nil {
		return openai.ChatCompletionMessageToolCallUnionParam{}, fmt.Errorf("failed to marshal tool call arguments: %w", err)
	}

	return openai.ChatCompletionMessageToolCallUnionParam{
		OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
			ID: toolCall.ID,
			Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
				Arguments: string(argsJSON),
				Name:      toolCall.Function.Name,
			},
		},
	}, nil
}

// FromChatCompletionMessage converts an OpenAI ChatCompletionMessage to types.Message
func FromChatCompletionMessage(msg *openai.ChatCompletionMessage) *types.Message {
	if msg == nil {
		return nil
	}

	message := &types.Message{
		Role:        types.RoleAssistant,
		ContentPart: make([]types.ContentPart, 0),
		ToolCalls:   make([]types.ToolCall, 0),
	}

	// Add text content if present
	if msg.Content != "" {
		message.ContentPart = append(message.ContentPart, types.NewContentPartText(msg.Content))
	}

	// Add refusal content if present
	if msg.Refusal != "" {
		message.ContentPart = append(message.ContentPart, types.NewContentPartRefusal(msg.Refusal))
	}

	// Convert tool calls if present
	for _, toolCall := range msg.ToolCalls {
		tc := fromToolCall(toolCall)
		if tc != nil {
			message.ToolCalls = append(message.ToolCalls, *tc)
		}
		// Skip tool calls with invalid JSON arguments
	}

	return message
}

// fromToolCall converts an OpenAI tool call to types.ToolCall
// Returns nil if the arguments cannot be parsed as valid JSON
func fromToolCall(toolCall openai.ChatCompletionMessageToolCallUnion) *types.ToolCall {
	// Use AsFunction() to get the function tool call from the union
	functionCall := toolCall.AsFunction()

	args, err := parseArguments(functionCall.Function.Arguments)
	if err != nil {
		// Return nil for tool calls with invalid JSON arguments
		// Caller should handle this appropriately
		return nil
	}

	return &types.ToolCall{
		ID: functionCall.ID,
		Function: types.ToolFunction{
			Name:      functionCall.Function.Name,
			Arguments: args,
		},
	}
}

// parseArguments converts JSON string arguments to map[string]any
// The Tool interface expects []byte for Execute(), but ToolFunction stores map[string]any
// for easier inspection without reparsing
func parseArguments(args string) (map[string]any, error) {
	var result map[string]any
	if err := json.Unmarshal([]byte(args), &result); err != nil {
		return nil, err
	}
	return result, nil
}
