package types

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
)

// StreamWithHandler streams a chat response using the provided client, invokes
// onChunk for every chunk, and returns a fully assembled ChatResponse when the
// stream finishes. All choices are accumulated independently so requests with
// n > 1 are supported.
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

	accumulators := make(map[int]*MessageAccumulator)
	finishReasons := make(map[int]string)
	order := make([]int, 0)
	var finalUsage *Usage

	for stream.Next() {
		chunk := stream.Chunk()
		if chunk == nil {
			continue
		}

		if onChunk != nil {
			onChunk(chunk)
		}

		for _, choice := range chunk.Choices {
			idx := choice.Index

			acc := accumulators[idx]
			if acc == nil {
				acc = NewMessageAccumulator()
				accumulators[idx] = acc
				order = append(order, idx)
			}

			if choice.Delta != nil {
				acc.Update(choice.Delta)
				if err := acc.Error(); err != nil {
					return nil, fmt.Errorf("stream accumulator (choice %d): %w", idx, err)
				}
			}

			if choice.FinishReason != "" {
				finishReasons[idx] = choice.FinishReason
			}
		}

		if chunk.Usage != nil {
			finalUsage = chunk.Usage
		}
	}

	if err := stream.Err(); err != nil {
		return nil, err
	}

	// Reconstruct choices in a stable order.
	sort.Ints(order)

	choices := make([]*Choice, 0, len(order))
	for _, idx := range order {
		acc := accumulators[idx]
		if acc == nil {
			continue
		}

		message, err := acc.Message()
		if err != nil {
			return nil, fmt.Errorf("stream accumulator (choice %d): %w", idx, err)
		}

		choices = append(choices, &Choice{
			Index:        idx,
			Message:      message,
			FinishReason: finishReasons[idx],
		})
	}

	return &ChatResponse{
		Model:   params.Model,
		Choices: choices,
		Usage:   finalUsage,
	}, nil
}
