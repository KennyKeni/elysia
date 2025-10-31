package openai

import "github.com/openai/openai-go/v3"

func validateChatCompletion(completion *openai.ChatCompletion) error {
	if completion == nil {
		return ErrNilCompletion
	}

	if len(completion.Choices) == 0 {
		return ErrNoChoices
	}

	return nil
}
