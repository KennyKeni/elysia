package openai

import (
	"github.com/KennyKeni/elysia/types"
	"github.com/openai/openai-go/v3"
)

// FromChatCompletion converts an OpenAI ChatCompletion to the unified types.ChatResponse
func FromChatCompletion(completion *openai.ChatCompletion) *types.ChatResponse {
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
		Message:      FromChatCompletionMessage(&choice.Message),
		FinishReason: choice.FinishReason,
	}
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
