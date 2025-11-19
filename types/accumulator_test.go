package types

import "testing"

func TestMessageAccumulatorBuildsMessage(t *testing.T) {
	acc := NewMessageAccumulator()

	acc.Update(&MessageDelta{
		Role:    RoleAssistant,
		Content: "Hel",
		ToolCalls: []ToolCallDelta{
			{
				Index:        0,
				ID:           "call_1",
				FunctionName: "do_something",
				Arguments:    `{"arg": "val`,
			},
		},
	})

	acc.Update(&MessageDelta{
		Content: "lo",
		Refusal: "",
		ToolCalls: []ToolCallDelta{
			{
				Index:     0,
				Arguments: `ue"}`,
			},
		},
	})

	msg, err := acc.Message()
	if err != nil {
		t.Fatalf("Message() returned error: %v", err)
	}

	if msg.Role != RoleAssistant {
		t.Fatalf("expected role %q, got %q", RoleAssistant, msg.Role)
	}

	if len(msg.ContentPart) != 1 {
		t.Fatalf("expected 1 content part, got %d", len(msg.ContentPart))
	}

	text, ok := msg.ContentPart[0].(*ContentPartText)
	if !ok {
		t.Fatalf("expected ContentPartText, got %T", msg.ContentPart[0])
	}
	if text.Text != "Hello" {
		t.Fatalf("expected text %q, got %q", "Hello", text.Text)
	}

	if len(msg.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(msg.ToolCalls))
	}

	if msg.ToolCalls[0].ID != "call_1" {
		t.Fatalf("expected tool call ID %q, got %q", "call_1", msg.ToolCalls[0].ID)
	}

	if msg.ToolCalls[0].Function.Name != "do_something" {
		t.Fatalf("expected tool name %q, got %q", "do_something", msg.ToolCalls[0].Function.Name)
	}

	arg, ok := msg.ToolCalls[0].Function.Arguments["arg"]
	if !ok {
		t.Fatalf("expected arguments to contain key %q", "arg")
	}
	if arg != "value" {
		t.Fatalf("expected argument value %q, got %v", "value", arg)
	}
}

func TestMessageAccumulatorInvalidJSON(t *testing.T) {
	acc := NewMessageAccumulator()
	acc.Update(&MessageDelta{
		ToolCalls: []ToolCallDelta{
			{
				Index:     0,
				Arguments: `{"unterminated"`,
			},
		},
	})

	if _, err := acc.Message(); err == nil {
		t.Fatalf("expected error for invalid JSON arguments")
	}
}
