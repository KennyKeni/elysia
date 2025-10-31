package types

import (
	"context"
	"log/slog"
)

// StreamWithHandler streams a chat response using the provided client, invokes
// onChunk for every chunk, and returns a fully assembled ChatResponse when the
// stream finishes. The helper assumes a single choice (n == 1). Clients that
// request multiple choices should extend the accumulator logic accordingly.
func StreamWithHandler(
	ctx context.Context,
	client Client,
	params *ChatParams,
	onChunk func(*StreamChunk),
) (*ChatResponse, error) {
	stream, err := client.ChatStream(ctx, params)
	if err != nil {
		return nil, err
	}
	defer func(stream *Stream) {
		err := stream.Close()
		if err != nil {
			slog.Warn("chat stream close failed", "err", err)
		}
	}(stream)

	acc := NewMessageAccumulator()

	var (
		finalUsage       *Usage
		lastFinishReason string
	)

	for stream.Next() {
		chunk := stream.Chunk()
		if chunk == nil {
			continue
		}

		if onChunk != nil {
			onChunk(chunk)
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]
		if choice.Delta != nil {
			acc.Update(choice.Delta)
		}

		if choice.FinishReason != "" {
			lastFinishReason = choice.FinishReason
		}

		if chunk.Usage != nil {
			finalUsage = chunk.Usage
		}
	}

	if err := stream.Err(); err != nil {
		return nil, err
	}

	message, err := acc.Message()
	if err != nil {
		return nil, err
	}

	return &ChatResponse{
		Model: params.Model,
		Choices: []*Choice{
			{
				Index:        0,
				Message:      message,
				FinishReason: lastFinishReason,
			},
		},
		Usage: finalUsage,
	}, nil
}
