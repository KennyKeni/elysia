package types

// ToolDefinition is metadata describing a tool for the LLM
// The client sends these to the LLM, but does not execute tools
// Execution is handled by the caller (agent layer or manual)
type ToolDefinition struct {
	Name         string
	Description  string
	InputSchema  map[string]any
	OutputSchema map[string]any
}

type ToolResult struct {
	ContentPart       []ContentPart
	StructuredContent any
	IsError           bool
}

type ToolOption func(*ToolResult)

// WithToolText Appends ContentPartText to tool
func WithToolText(text string) ToolOption {
	return func(t *ToolResult) {
		t.ContentPart = append(t.ContentPart, &ContentPartText{Text: text})
	}
}

func WithToolImage(data string) ToolOption {
	return func(t *ToolResult) {
		t.ContentPart = append(t.ContentPart, &ContentPartImage{Data: data})
	}
}

func WithStructuredContent(content any) ToolOption {
	return func(t *ToolResult) {
		t.StructuredContent = content
	}
}

func NewToolResult(opts ...ToolOption) *ToolResult {
	t := &ToolResult{ContentPart: make([]ContentPart, 0)}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// NewToolResultMessage converts a ToolResult to a tool Message
// This is a convenience helper for creating tool response messages from tool execution results
func NewToolResultMessage(toolCallID string, result *ToolResult) Message {
	return Message{
		Role:        RoleTool,
		ContentPart: result.ContentPart,
		ToolCallID:  &toolCallID,
	}
}
