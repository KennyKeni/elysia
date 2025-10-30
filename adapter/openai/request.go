package openai

import (
	"fmt"
	"log/slog"

	"github.com/KennyKeni/elysia/types"
	"github.com/openai/openai-go/v3"
)

func ToChatCompletionParams(chatParams *types.ChatParams) (openai.ChatCompletionNewParams, error) {
	request := openai.ChatCompletionNewParams{
		Model: chatParams.Model,
		Stop:  openai.ChatCompletionNewParamsStopUnion{OfStringArray: chatParams.Stop},
	}

	if chatParams.MaxTokens != nil {
		request.MaxTokens = openai.Int(int64(*chatParams.MaxTokens))
	}

	if chatParams.Temperature != nil {
		request.Temperature = openai.Float(*chatParams.Temperature)
	}

	if chatParams.TopP != nil {
		request.TopP = openai.Float(*chatParams.TopP)
	}

	if chatParams.TopK != nil {
		slog.Warn("OpenAI does not support top K parameter")
	}

	messages, err := ToChatCompletionMessage(chatParams.SystemPrompt, chatParams.Messages)
	if err != nil {
		return openai.ChatCompletionNewParams{}, fmt.Errorf("ToChatCompletionMessage failed: %w", err)
	}
	request.Messages = messages

	// Convert tools if provided
	if len(chatParams.Tools) > 0 {
		tools, err := ToTools(chatParams.Tools)
		if err != nil {
			return openai.ChatCompletionNewParams{}, fmt.Errorf("ToTools failed: %w", err)
		}
		request.Tools = tools

		// Convert tool choice if provided
		if chatParams.ToolChoice != nil {
			request.ToolChoice = ToToolChoice(chatParams.ToolChoice)
		}
	}

	return request, nil
}
