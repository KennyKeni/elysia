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
		Choices: make([]types.Choice, len(completion.Choices)),
		Usage:   FromUsage(&completion.Usage),
		Extra:   make(map[string]any),
	}

	for i, choice := range completion.Choices {
		response.Choices[i] = fromChoice(&choice)
	}

	return response
}

// fromChoice converts an OpenAI ChatCompletionChoice to types.Choice
func fromChoice(choice *openai.ChatCompletionChoice) types.Choice {
	if choice == nil {
		return types.Choice{}
	}

	return types.Choice{
		Index:        int(choice.Index),
		Message:      FromChatCompletionMessage(&choice.Message),
		FinishReason: choice.FinishReason,
	}
}

// FromUsage converts OpenAI CompletionUsage to types.Usage
func FromUsage(usage *openai.CompletionUsage) *types.Usage {
	if usage == nil {
		return nil
	}

	return &types.Usage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}
}
