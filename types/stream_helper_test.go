package types

import (
	"context"
	"io"
	"testing"
)

type stubStreamClient struct {
	chunks []*StreamChunk
}

func (s *stubStreamClient) Chat(ctx context.Context, params *ChatParams) (*ChatResponse, error) {
	return nil, nil
}

func (s *stubStreamClient) ChatStream(ctx context.Context, params *ChatParams) (*Stream, error) {
	index := 0
	next := func() (*StreamChunk, error) {
		if index >= len(s.chunks) {
			return nil, io.EOF
		}
		chunk := s.chunks[index]
		index++
		return chunk, nil
	}
	return NewStream(next, nil), nil
}

func TestStreamWithHandlerMultipleChoices(t *testing.T) {
	client := &stubStreamClient{
		chunks: []*StreamChunk{
			{
				Choices: []*StreamChoice{
					{Index: 1, Delta: &MessageDelta{Content: "Wor"}},
					{Index: 0, Delta: &MessageDelta{Content: "Hel"}},
				},
			},
			{
				Choices: []*StreamChoice{
					{Index: 0, Delta: &MessageDelta{Content: "lo"}, FinishReason: "stop"},
					{Index: 1, Delta: &MessageDelta{Content: "ld"}, FinishReason: "length"},
				},
				Usage: &Usage{
					PromptTokens:     1,
					CompletionTokens: 2,
					TotalTokens:      3,
				},
			},
		},
	}

	chunkCount := 0
	resp, err := StreamWithHandler(
		context.Background(),
		client,
		&ChatParams{Model: "test-model"},
		func(chunk *StreamChunk) {
			chunkCount++
		},
	)
	if err != nil {
		t.Fatalf("StreamWithHandler error: %v", err)
	}

	if chunkCount != len(client.chunks) {
		t.Fatalf("expected %d chunks, got %d", len(client.chunks), chunkCount)
	}

	if resp == nil {
		t.Fatalf("expected response, got nil")
	}

	if resp.Model != "test-model" {
		t.Fatalf("expected model %q, got %q", "test-model", resp.Model)
	}

	if len(resp.Choices) != 2 {
		t.Fatalf("expected 2 choices, got %d", len(resp.Choices))
	}

	if resp.Choices[0].Index != 0 || resp.Choices[1].Index != 1 {
		t.Fatalf("expected choices in index order [0,1], got [%d,%d]", resp.Choices[0].Index, resp.Choices[1].Index)
	}

	assertText := func(choice *Choice, expectedText string) {
		if len(choice.Message.ContentPart) != 1 {
			t.Fatalf("expected 1 content part, got %d", len(choice.Message.ContentPart))
		}
		textPart, ok := choice.Message.ContentPart[0].(*ContentPartText)
		if !ok {
			t.Fatalf("expected ContentPartText, got %T", choice.Message.ContentPart[0])
		}
		if textPart.Text != expectedText {
			t.Fatalf("expected text %q, got %q", expectedText, textPart.Text)
		}
	}

	assertText(resp.Choices[0], "Hello")
	assertText(resp.Choices[1], "World")

	if resp.Choices[0].FinishReason != "stop" {
		t.Fatalf("expected finish reason 'stop' for choice 0, got %q", resp.Choices[0].FinishReason)
	}
	if resp.Choices[1].FinishReason != "length" {
		t.Fatalf("expected finish reason 'length' for choice 1, got %q", resp.Choices[1].FinishReason)
	}

	if resp.Usage == nil || resp.Usage.TotalTokens != 3 {
		t.Fatalf("expected usage total tokens 3, got %#v", resp.Usage)
	}
}
