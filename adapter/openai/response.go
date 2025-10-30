package openai

import (
	"encoding/json"

	"github.com/KennyKeni/elysia/types"
	"github.com/openai/openai-go/v3"
)

// ToChatResponse converts an OpenAI ChatCompletion to the unified types.ChatResponse
func ToChatResponse(completion *openai.ChatCompletion) *types.ChatResponse {
	if completion == nil {
		return nil
	}

	response := &types.ChatResponse{
		ID:      completion.ID,
		Created: completion.Created,
		Model:   completion.Model,
		Choices: make([]*types.Choice, len(completion.Choices)),
		Usage:   toUsage(&completion.Usage),
		Extra:   make(map[string]any),
	}

	for i, choice := range completion.Choices {
		response.Choices[i] = toChoice(&choice)
	}

	return response
}

// toChoice converts an OpenAI ChatCompletionChoice to types.Choice
func toChoice(choice *openai.ChatCompletionChoice) *types.Choice {
	if choice == nil {
		return nil
	}

	return &types.Choice{
		Index:        int(choice.Index),
		Message:      toMessage(&choice.Message),
		FinishReason: choice.FinishReason,
	}
}

// toMessage converts an OpenAI ChatCompletionMessage to types.Message
func toMessage(msg *openai.ChatCompletionMessage) *types.Message {
	if msg == nil {
		return nil
	}

	message := &types.Message{
		Role:        types.RoleAssistant,
		ContentPart: make([]types.ContentPart, 0),
		ToolCalls:   make([]*types.ToolCall, 0),
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
		tc := toToolCall(toolCall)
		if tc != nil {
			message.ToolCalls = append(message.ToolCalls, tc)
		}
		// Skip tool calls with invalid JSON arguments
	}

	return message
}

// toToolCall converts an OpenAI tool call to types.ToolCall
// Returns nil if the arguments cannot be parsed as valid JSON
func toToolCall(toolCall openai.ChatCompletionMessageToolCallUnion) *types.ToolCall {
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

// toUsage converts OpenAI CompletionUsage to types.Usage
func toUsage(usage *openai.CompletionUsage) *types.Usage {
	if usage == nil {
		return nil
	}

	return &types.Usage{
		PromptTokens:     int(usage.PromptTokens),
		CompletionTokens: int(usage.CompletionTokens),
		TotalTokens:      int(usage.TotalTokens),
	}
}
