package openai

import (
	"io"

	"github.com/KennyKeni/elysia/types"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/ssestream"
)

// FromChatCompletionChunk converts an OpenAI ChatCompletionChunk into the unified stream chunk.
func FromChatCompletionChunk(chunk *openai.ChatCompletionChunk) *types.StreamChunk {
	if chunk == nil {
		return nil
	}

	streamChunk := &types.StreamChunk{
		ID:      chunk.ID,
		Created: chunk.Created,
		Model:   chunk.Model,
		Choices: make([]types.StreamChoice, len(chunk.Choices)),
	}

	for i := range chunk.Choices {
		choice := chunk.Choices[i]
		streamChunk.Choices[i] = types.StreamChoice{
			Index:        int(choice.Index),
			FinishReason: choice.FinishReason,
			Delta:        toMessageDelta(&choice.Delta),
		}
	}

	if usage := toChunkUsage(chunk); usage != nil {
		streamChunk.Usage = usage
	}

	return streamChunk
}

func toChunkUsage(chunk *openai.ChatCompletionChunk) *types.Usage {
	if chunk == nil {
		return nil
	}

	// Usage is only populated when include_usage is set; check JSON metadata before converting.
	if !chunk.Usage.JSON.TotalTokens.Valid() &&
		!chunk.Usage.JSON.PromptTokens.Valid() &&
		!chunk.Usage.JSON.CompletionTokens.Valid() {
		return nil
	}

	return FromUsage(&chunk.Usage)
}

func toMessageDelta(delta *openai.ChatCompletionChunkChoiceDelta) *types.MessageDelta {
	if delta == nil {
		return nil
	}

	messageDelta := &types.MessageDelta{
		Role:    types.Role(delta.Role),
		Content: delta.Content,
		Refusal: delta.Refusal,
	}

	toolCalls := make([]types.ToolCallDelta, 0, len(delta.ToolCalls))
	for _, call := range delta.ToolCalls {
		toolCalls = append(toolCalls, types.ToolCallDelta{
			Index:        int(call.Index),
			ID:           call.ID,
			FunctionName: call.Function.Name,
			Arguments:    call.Function.Arguments,
		})
	}

	// Handle legacy function_call field by synthesising a tool call delta at index 0.
	if delta.FunctionCall.Name != "" || delta.FunctionCall.Arguments != "" {
		toolCalls = append(toolCalls, types.ToolCallDelta{
			Index:        0,
			FunctionName: delta.FunctionCall.Name,
			Arguments:    delta.FunctionCall.Arguments,
		})
	}

	if len(toolCalls) > 0 {
		messageDelta.ToolCalls = toolCalls
	}

	return messageDelta
}

type chatStreamWrapper struct {
	stream *ssestream.Stream[openai.ChatCompletionChunk]
}

func newChatStream(stream *ssestream.Stream[openai.ChatCompletionChunk]) *types.Stream {
	wrapper := &chatStreamWrapper{stream: stream}
	return types.NewStream(wrapper.next, wrapper)
}

func (w *chatStreamWrapper) next() (*types.StreamChunk, error) {
	if w.stream == nil {
		return nil, io.EOF
	}

	if !w.stream.Next() {
		if err := w.stream.Err(); err != nil {
			return nil, err
		}
		return nil, io.EOF
	}

	chunk := w.stream.Current()
	return FromChatCompletionChunk(&chunk), nil
}

func (w *chatStreamWrapper) Close() error {
	if w.stream == nil {
		return nil
	}
	return w.stream.Close()
}
