package openai

import "errors"

var (
	// ErrNilCompletion is returned when the OpenAI SDK yields a nil completion response.
	ErrNilCompletion = errors.New("openai chat: empty completion response")

	// ErrNoChoices is returned when the completion response contains zero choices.
	ErrNoChoices = errors.New("openai chat: response contained no choices")

	// ErrUnsupportedMessageRole indicates that a message role is not supported by the adapter.
	ErrUnsupportedMessageRole = errors.New("openai chat: unsupported message role")

	// ErrUnsupportedUserContentPart indicates that a user message includes content the adapter cannot convert.
	ErrUnsupportedUserContentPart = errors.New("openai chat: unsupported content part for user message")

	// ErrUnsupportedAssistantContentPart indicates that an assistant message includes unsupported content.
	ErrUnsupportedAssistantContentPart = errors.New("openai chat: unsupported content part for assistant message")

	// ErrUnsupportedToolContentPart indicates that a tool result message includes unsupported content.
	ErrUnsupportedToolContentPart = errors.New("openai chat: unsupported content part for tool message")

	// ErrMissingToolCallID indicates that a tool result message is missing the required ToolCallID.
	ErrMissingToolCallID = errors.New("openai chat: tool message missing ToolCallID")
)
