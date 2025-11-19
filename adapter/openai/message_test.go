package openai

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/KennyKeni/elysia/types"
)

type unsupportedContentPart struct{}

func (*unsupportedContentPart) IsContentPart() {}

func TestToChatCompletionMessageUnsupportedRole(t *testing.T) {
	messages := []types.Message{
		{
			Role: "unknown-role",
		},
	}

	if _, err := ToChatCompletionMessage("", messages); err == nil || !errors.Is(err, ErrUnsupportedMessageRole) {
		t.Fatalf("expected ErrUnsupportedMessageRole, got %v", err)
	}
}

func TestToChatCompletionMessageUnsupportedUserContent(t *testing.T) {
	msg := types.NewUserMessage()
	msg.ContentPart = append(msg.ContentPart, &unsupportedContentPart{})

	if _, err := ToChatCompletionMessage("", []types.Message{msg}); err == nil || !errors.Is(err, ErrUnsupportedUserContentPart) {
		t.Fatalf("expected ErrUnsupportedUserContentPart, got %v", err)
	}
}

func TestToChatCompletionMessageUnsupportedAssistantContent(t *testing.T) {
	msg := types.NewAssistantMessage(types.WithImage("image-data"))

	if _, err := ToChatCompletionMessage("", []types.Message{msg}); err == nil || !errors.Is(err, ErrUnsupportedAssistantContentPart) {
		t.Fatalf("expected ErrUnsupportedAssistantContentPart, got %v", err)
	}
}

func TestToChatCompletionMessageUnsupportedToolContent(t *testing.T) {
	msg := types.NewToolMessage(types.WithImage("image-data"), types.WithToolCallID("call-1"))

	if _, err := ToChatCompletionMessage("", []types.Message{msg}); err == nil || !errors.Is(err, ErrUnsupportedToolContentPart) {
		t.Fatalf("expected ErrUnsupportedToolContentPart, got %v", err)
	}
}

func TestToChatCompletionMessageMissingToolCallID(t *testing.T) {
	msg := types.NewToolMessage(types.WithText("result"))

	if _, err := ToChatCompletionMessage("", []types.Message{msg}); err == nil || !errors.Is(err, ErrMissingToolCallID) {
		t.Fatalf("expected ErrMissingToolCallID, got %v", err)
	}
}

func TestToChatCompletionMessageSuccess(t *testing.T) {
	toolCall := &types.ToolCall{
		ID: "call-1",
		Function: types.ToolFunction{
			Name: "lookup",
			Arguments: map[string]any{
				"city": "San Francisco",
			},
		},
	}

	messages := []types.Message{
		types.NewUserMessage(types.WithText("What's the weather?")),
		types.NewAssistantMessage(
			types.WithText("Let me check."),
			types.WithToolCalls(*toolCall),
		),
		types.NewToolMessage(
			types.WithToolCallID(toolCall.ID),
			types.WithText(`{"temperature":70}`),
		),
	}

	result, err := ToChatCompletionMessage("You are a helpful assistant.", messages)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(result) != len(messages)+1 {
		t.Fatalf("expected %d messages, got %d", len(messages)+1, len(result))
	}

	if result[0].OfSystem == nil {
		t.Fatal("expected system prompt message at index 0")
	}

	if result[1].OfUser == nil {
		t.Fatal("expected user message at index 1")
	}

	assistant := result[2].OfAssistant
	if assistant == nil {
		t.Fatal("expected assistant message at index 2")
	}

	if len(assistant.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(assistant.ToolCalls))
	}

	tool := result[3].OfTool
	if tool == nil {
		t.Fatal("expected tool message at index 3")
	}

	if tool.ToolCallID != toolCall.ID {
		t.Fatalf("expected ToolCallID %q, got %q", toolCall.ID, tool.ToolCallID)
	}

	parts := tool.Content.OfArrayOfContentParts
	if len(parts) != 1 {
		t.Fatalf("expected 1 content part, got %d", len(parts))
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(parts[0].Text), &decoded); err != nil {
		t.Fatalf("failed to unmarshal tool result: %v", err)
	}

	expected := map[string]any{"temperature": float64(70)}
	if !reflect.DeepEqual(decoded, expected) {
		t.Fatalf("unexpected tool result payload: %#v", decoded)
	}
}

func BenchmarkToChatCompletionMessage(b *testing.B) {
	toolCall := &types.ToolCall{
		ID: "call-1",
		Function: types.ToolFunction{
			Name: "lookup",
			Arguments: map[string]any{
				"city":   "San Francisco",
				"metric": true,
			},
		},
	}

	messages := []types.Message{
		types.NewUserMessage(
			types.WithText("What's the weather?"),
			types.WithImage("iVBORw0KGgoAAAANSUhEUgAAAAUA"),
		),
		types.NewAssistantMessage(
			types.WithText("Let me call the weather API."),
			types.WithToolCalls(*toolCall),
		),
		types.NewToolMessage(
			types.WithToolCallID(toolCall.ID),
			types.WithText(`{"temperature":70,"condition":"sunny"}`),
		),
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := ToChatCompletionMessage("You are a helpful assistant.", messages); err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}
