package openai

import (
	"github.com/KennyKeni/elysia/types"
	"github.com/openai/openai-go/v3"
)

func FromCreateEmbeddingResponse(response *openai.CreateEmbeddingResponse) *types.EmbeddingResponse {
	if response == nil {
		return nil
	}

	embeddings := make([]types.Embedding, len(response.Data))
	for i, openaiEmbedding := range response.Data {
		embeddings[i] = fromEmbedding(openaiEmbedding)
	}

	return &types.EmbeddingResponse{
		Model:      response.Model,
		Embeddings: embeddings,
		Usage:      fromEmbeddingUsage(&response.Usage),
	}
}

func fromEmbedding(embedding openai.Embedding) types.Embedding {
	return types.Embedding{
		Index:  embedding.Index,
		Vector: embedding.Embedding,
		Object: string(embedding.Object),
	}
}

func fromEmbeddingUsage(usage *openai.CreateEmbeddingResponseUsage) *types.Usage {
	if usage == nil {
		return nil
	}

	return &types.Usage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: 0,
		TotalTokens:      usage.TotalTokens,
	}
}
