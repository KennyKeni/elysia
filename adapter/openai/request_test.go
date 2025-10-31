package openai

import (
	"testing"

	"github.com/KennyKeni/elysia/types"
)

func TestToChatCompletionParamsStreamOptionsIncludeUsage(t *testing.T) {
	params := &types.ChatParams{
		Model: "gpt-4o-mini",
		StreamOptions: &types.StreamOptions{
			IncludeUsage: true,
		},
	}

	openaiParams, err := ToChatCompletionParams(params)
	if err != nil {
		t.Fatalf("ToChatCompletionParams returned error: %v", err)
	}

	if !openaiParams.StreamOptions.IncludeUsage.Valid() {
		t.Fatalf("expected include_usage to be set")
	}

	if !openaiParams.StreamOptions.IncludeUsage.Or(false) {
		t.Fatalf("expected include_usage to be true")
	}
}

func TestToChatCompletionParamsStreamOptionsOmittedWhenFalse(t *testing.T) {
	params := &types.ChatParams{
		Model:         "gpt-4o-mini",
		StreamOptions: &types.StreamOptions{},
	}

	openaiParams, err := ToChatCompletionParams(params)
	if err != nil {
		t.Fatalf("ToChatCompletionParams returned error: %v", err)
	}

	if openaiParams.StreamOptions.IncludeUsage.Valid() {
		t.Fatalf("expected include_usage to be omitted when false")
	}
}
