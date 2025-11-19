package openai

import (
	"errors"

	"github.com/KennyKeni/elysia/types"
	"github.com/openai/openai-go/v3"
)

func ToEmbeddingParams(embeddingParams *types.EmbeddingParams) (openai.EmbeddingNewParams, error) {
	if embeddingParams == nil {
		return openai.EmbeddingNewParams{}, errors.New("nil chatParams")
	}

	openaiInput := openai.EmbeddingNewParamsInputUnion{
		OfArrayOfStrings: embeddingParams.Input,
	}

	request := openai.EmbeddingNewParams{
		Input: openaiInput,
		Model: openai.EmbeddingModel(embeddingParams.Model),
	}

	if embeddingParams.Dimensions != nil {
		request.Dimensions = openai.Int(int64(*embeddingParams.Dimensions))
	}

	if embeddingParams.EncodingFormat != nil {
		request.EncodingFormat = openai.EmbeddingNewParamsEncodingFormat(string(*embeddingParams.EncodingFormat))
	}

	return request, nil
}
