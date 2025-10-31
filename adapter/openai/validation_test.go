package openai

import (
	"testing"

	sdk "github.com/openai/openai-go/v3"
)

func TestValidateChatCompletion(t *testing.T) {
	if err := validateChatCompletion(nil); err != ErrNilCompletion {
		t.Fatalf("expected ErrNilCompletion, got %v", err)
	}

	empty := &sdk.ChatCompletion{}
	if err := validateChatCompletion(empty); err != ErrNoChoices {
		t.Fatalf("expected ErrNoChoices, got %v", err)
	}

	valid := &sdk.ChatCompletion{
		Choices: []sdk.ChatCompletionChoice{{}},
	}
	if err := validateChatCompletion(valid); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}
