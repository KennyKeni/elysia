package openai

import (
	"github.com/KennyKeni/elysia/types"
	"github.com/openai/openai-go/v3"
	"log/slog"
)

func ToChatCompletionParams(chatParams *types.ChatParams) openai.ChatCompletionNewParams {
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

	return request
}
