package types

import "context"

// RawClient is implemented by adapters - just provider-specific API calls.
// Adapters should NOT be exported directly; use NewClient(raw) to wrap them.
type RawClient interface {
	RawChat(ctx context.Context, params *ChatParams) (*ChatResponse, error)
	RawChatStream(ctx context.Context, params *ChatParams) (*Stream, error)
	RawEmbed(ctx context.Context, params *EmbeddingParams) (*EmbeddingResponse, error)
}

// Client is the public interface with enforced ResponseFormat handling.
// Created by wrapping a RawClient with NewClient().
type Client interface {
	Chat(ctx context.Context, params *ChatParams) (*ChatResponse, error)
	ChatStream(ctx context.Context, params *ChatParams) (*Stream, error)
	Embed(ctx context.Context, params *EmbeddingParams) (*EmbeddingResponse, error)
}

type baseClient struct {
	raw RawClient
}

func NewClient(rc RawClient) Client {
	return &baseClient{raw: rc}
}

func (bc *baseClient) Chat(ctx context.Context, params *ChatParams) (*ChatResponse, error) {
	ApplyResponseFormat(params)

	resp, err := bc.raw.RawChat(ctx, params)
	if err != nil {
		return nil, err
	}

	if params.ResponseFormat.Schema != nil {
		for i := range resp.Choices {
			if resp.Choices[i].Message != nil {
				// Note, the reason why ANY message can set off this technically because we do not expect usage
				// of structured output with n > 1. It isn't allowed via OpenAI API and also can't be handled gracefully
				content, err := ExtractStructuredContent(params.ResponseFormat, resp.Choices[i].Message)
				if err != nil {
					return nil, err
				}
				resp.Choices[i].StructuredContent = content
			}
		}
	}

	return resp, nil
}

func (bc *baseClient) ChatStream(ctx context.Context, params *ChatParams) (*Stream, error) {
	ApplyResponseFormat(params)
	return bc.raw.RawChatStream(ctx, params)
	// Note: Streaming extraction happens in Accumulator (separate concern)
}

func (bc *baseClient) Embed(ctx context.Context, params *EmbeddingParams) (*EmbeddingResponse, error) {
	return bc.raw.RawEmbed(ctx, params)
}
