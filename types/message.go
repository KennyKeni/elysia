package types

type ContentPart interface {
	IsContentPart()
}

type Role string

const (
	// RoleSystem    Role = "system" // Removed: System prompt is a parameter in ChatParams, not a message role
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type ImageDetail string

const (
	ImageDetailLow    ImageDetail = "low"
	ImageDetailMedium ImageDetail = "medium"
	ImageDetailHigh   ImageDetail = "high"
)

type Message struct {
	Role        Role          `json:"role"`
	ContentPart []ContentPart `json:"content_part"`
	ToolCalls   []*ToolCall   `json:"tool_calls,omitempty"`
	ToolCallID  *string       `json:"tool_call_id,omitempty"` // For RoleTool messages - references which call this respond to
}

type ContentPartText struct {
	Text string `json:"text"`
}

func (*ContentPartText) IsContentPart() {}

func NewContentPartText(text string) *ContentPartText { return &ContentPartText{Text: text} }

// ContentPartImage uses Base64 data values
type ContentPartImage struct {
	Data   string `json:"data"`
	Detail string `json:"detail"`
}

func NewContentPartImage(data string) *ContentPartImage { return &ContentPartImage{Data: data} }
func NewContentPartImageWithDetail(data string, detail ImageDetail) *ContentPartImage {
	return &ContentPartImage{Data: data, Detail: string(detail)}
}

type ContentPartImageURL struct {
	URL string `json:"url"`
}

func (*ContentPartImageURL) IsContentPart() {}

func NewContentPartImageURL(url string) *ContentPartImageURL { return &ContentPartImageURL{URL: url} }

func (*ContentPartImage) IsContentPart() {}

type ContentPartRefusal struct {
	Refusal string `json:"refusal"`
}

func NewContentPartRefusal(refusal string) *ContentPartRefusal {
	return &ContentPartRefusal{Refusal: refusal}
}

func (*ContentPartRefusal) IsContentPart() {}

type ContentPartToolResult struct {
	Value any `json:"value"`
}

func NewContentPartToolResult(value any) *ContentPartToolResult {
	return &ContentPartToolResult{Value: value}
}

func (*ContentPartToolResult) IsContentPart() {}

type ToolCall struct {
	ID       string       `json:"id"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type MessageOption func(*Message)

func WithText(text string) MessageOption {
	return func(m *Message) {
		m.ContentPart = append(m.ContentPart, &ContentPartText{Text: text})
	}
}

func WithImage(data string) MessageOption {
	return func(m *Message) {
		m.ContentPart = append(m.ContentPart, &ContentPartImage{Data: data})
	}
}

func WithToolCalls(toolCalls ...*ToolCall) MessageOption {
	return func(m *Message) {
		m.ToolCalls = append(m.ToolCalls, toolCalls...)
	}
}

func WithToolCallID(toolCallID string) MessageOption {
	return func(m *Message) {
		m.ToolCallID = &toolCallID
	}
}

func WithToolResult(value any) MessageOption {
	return func(m *Message) {
		m.ContentPart = append(m.ContentPart, NewContentPartToolResult(value))
	}
}

// NewSystemMessage removed: System prompt is a parameter in ChatParams, not a message role
// func NewSystemMessage(opts ...MessageOption) *Message {
// 	m := &Message{Role: RoleSystem, ContentPart: make([]ContentPart, 0)}
// 	for _, opt := range opts {
// 		opt(m)
// 	}
// 	return m
// }

func NewUserMessage(opts ...MessageOption) *Message {
	m := &Message{Role: RoleUser, ContentPart: make([]ContentPart, 0)}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func NewAssistantMessage(opts ...MessageOption) *Message {
	m := &Message{Role: RoleAssistant, ContentPart: make([]ContentPart, 0)}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func NewToolMessage(opts ...MessageOption) *Message {
	m := &Message{Role: RoleTool, ContentPart: make([]ContentPart, 0)}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func NewToolResponse(toolCallID string, value any) *Message {
	return NewToolMessage(
		WithToolCallID(toolCallID),
		WithToolResult(value),
	)
}
