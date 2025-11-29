package openai

import (
	"errors"
	"fmt"

	"github.com/KennyKeni/elysia/types"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"
)

func ToChatCompletionParams(chatParams *types.ChatParams) (openai.ChatCompletionNewParams, error) {
	if chatParams == nil {
		return openai.ChatCompletionNewParams{}, errors.New("nil chatParams")
	}

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

	// topK is ignored

	messages, err := ToChatCompletionMessage(chatParams.SystemPrompt, chatParams.Messages)
	if err != nil {
		return openai.ChatCompletionNewParams{}, fmt.Errorf("ToChatCompletionMessage failed: %w", err)
	}
	request.Messages = messages

	// Convert tools if provided
	if len(chatParams.Tools) > 0 {
		tools, err := ToToolDefinitions(chatParams.Tools)
		if err != nil {
			return openai.ChatCompletionNewParams{}, fmt.Errorf("ToToolDefinitions failed: %w", err)
		}
		request.Tools = tools

		// Convert tool choice if provided
		if chatParams.ToolChoice != nil {
			request.ToolChoice = ToToolChoice(chatParams.ToolChoice)
		}
	}

	if chatParams.StreamOptions != nil && chatParams.StreamOptions.IncludeUsage {
		request.StreamOptions = openai.ChatCompletionStreamOptionsParam{
			IncludeUsage: openai.Bool(true),
		}
	}

	// Handle Native mode ResponseFormat
	rf := chatParams.ResponseFormat
	if rf.Mode == types.ResponseFormatModeNative && rf.Schema != nil {
		name := rf.Name
		if name == "" {
			name = "response"
		}
		request.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
				JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:        name,
					Description: openai.String(rf.Description),
					Schema:      rf.Schema,
					Strict:      openai.Bool(true),
				},
			},
		}
	}

	return request, nil
}
