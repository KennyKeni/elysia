package openai

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/KennyKeni/elysia/types"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/ssestream"
)

const sampleChunkJSON = `{
	"id": "chunk_1",
	"object": "chat.completion.chunk",
	"created": 123,
	"model": "gpt-4o-mini",
	"choices": [
		{
			"index": 0,
			"delta": {
				"role": "assistant",
				"content": "Hello",
				"tool_calls": [
					{
						"index": 0,
						"id": "call_1",
						"type": "function",
						"function": {
							"name": "do_something",
							"arguments": "{\"arg\": \"value\"}"
						}
					}
				]
			},
			"finish_reason": "stop",
			"logprobs": null
		}
	],
	"service_tier": null,
	"system_fingerprint": "",
	"usage": {
		"prompt_tokens": 1,
		"completion_tokens": 2,
		"total_tokens": 3,
		"completion_tokens_details": {},
		"prompt_tokens_details": {}
	}
}`

func TestFromChatCompletionChunk(t *testing.T) {
	var chunk openai.ChatCompletionChunk
	if err := json.Unmarshal([]byte(sampleChunkJSON), &chunk); err != nil {
		t.Fatalf("failed to unmarshal chunk JSON: %v", err)
	}

	streamChunk := FromChatCompletionChunk(&chunk)
	if streamChunk == nil {
		t.Fatal("expected non-nil StreamChunk")
	}

	if streamChunk.ID != "chunk_1" {
		t.Fatalf("expected chunk ID %q, got %q", "chunk_1", streamChunk.ID)
	}

	if streamChunk.Usage == nil || streamChunk.Usage.TotalTokens != 3 {
		t.Fatalf("expected usage total tokens to be 3, got %#v", streamChunk.Usage)
	}

	if len(streamChunk.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(streamChunk.Choices))
	}

	delta := streamChunk.Choices[0].Delta
	if delta == nil {
		t.Fatal("expected non-nil delta")
	}
	if delta.Role != types.RoleAssistant {
		t.Fatalf("expected role %q, got %q", types.RoleAssistant, delta.Role)
	}
	if delta.Content != "Hello" {
		t.Fatalf("expected content %q, got %q", "Hello", delta.Content)
	}
	if len(delta.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call delta, got %d", len(delta.ToolCalls))
	}
	if delta.ToolCalls[0].ID != "call_1" {
		t.Fatalf("expected tool call ID %q, got %q", "call_1", delta.ToolCalls[0].ID)
	}
	if delta.ToolCalls[0].Arguments != `{"arg": "value"}` {
		t.Fatalf("expected tool call arguments %q, got %q", `{"arg": "value"}`, delta.ToolCalls[0].Arguments)
	}
}

func TestChatStreamWrapper(t *testing.T) {
	decoder := &fakeDecoder{
		events: []ssestream.Event{
			{Type: "", Data: []byte(sampleChunkJSON)},
			{Type: "", Data: []byte("[DONE]")},
		},
	}

	stream := ssestream.NewStream[openai.ChatCompletionChunk](decoder, nil)
	typesStream := newChatStream(stream)
	defer func() {
		if err := typesStream.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
		if !decoder.closed {
			t.Fatal("expected decoder to be closed")
		}
	}()

	if !typesStream.Next() {
		t.Fatal("expected Next to return true for first chunk")
	}

	chunk := typesStream.Chunk()
	if chunk == nil {
		t.Fatal("expected chunk to be non-nil")
	}
	if chunk.ID != "chunk_1" {
		t.Fatalf("expected chunk ID %q, got %q", "chunk_1", chunk.ID)
	}

	if typesStream.Next() {
		t.Fatal("expected Next to return false after final chunk")
	}

	if err := typesStream.Err(); err != nil {
		t.Fatalf("expected nil error after consuming stream, got %v", err)
	}
}

func TestChatStreamWrapperPropagatesError(t *testing.T) {
	expectedErr := errors.New("stream error")
	decoder := &fakeDecoder{
		err: expectedErr,
	}

	stream := ssestream.NewStream[openai.ChatCompletionChunk](decoder, expectedErr)
	typesStream := newChatStream(stream)
	defer func() {
		if err := typesStream.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	if typesStream.Next() {
		t.Fatal("expected Next to return false when stream has error")
	}

	if err := typesStream.Err(); !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
}

type fakeDecoder struct {
	events  []ssestream.Event
	index   int
	current ssestream.Event
	err     error
	closed  bool
}

func (f *fakeDecoder) Next() bool {
	if f.index >= len(f.events) {
		return false
	}
	f.current = f.events[f.index]
	f.index++
	return true
}

func (f *fakeDecoder) Event() ssestream.Event {
	return f.current
}

func (f *fakeDecoder) Close() error {
	f.closed = true
	return nil
}

func (f *fakeDecoder) Err() error {
	return f.err
}
