package types

import "encoding/json"

// ChatParams represents parameters for a chat completion request.
// Supports OpenAI, Anthropic, and Google GenAI providers.
type ChatParams struct {
	// Core parameters
	Model        string     `json:"model"`
	Messages     []*Message `json:"messages"`
	SystemPrompt string     `json:"system_prompt,omitempty"`

	// Sampling parameters
	MaxTokens   *int     `json:"max_tokens,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"`
	TopP        *float64 `json:"top_p,omitempty"`
	TopK        *int     `json:"top_k,omitempty"` // Google, Anthropic

	// Control parameters
	Stop []string `json:"stop,omitempty"`

	// Tool parameters
	Tools      []Tool      `json:"tools,omitempty"`
	ToolChoice *ToolChoice `json:"tool_choice,omitempty"`

	// Provider-specific extras
	Extra map[string]any `json:"-"`
}

type ChatParamOption func(*ChatParams)

func WithMessages(messages []*Message) ChatParamOption {
	return func(p *ChatParams) {
		p.Messages = append(p.Messages, messages...)
	}
}

func WithSystemPrompt(prompt string) ChatParamOption {
	return func(p *ChatParams) {
		p.SystemPrompt = prompt
	}
}

func WithMaxTokens(maxTokens int) ChatParamOption {
	return func(p *ChatParams) {
		p.MaxTokens = &maxTokens
	}
}

func WithTemperature(temperature float64) ChatParamOption {
	return func(p *ChatParams) {
		p.Temperature = &temperature
	}
}

func WithTopP(topP float64) ChatParamOption {
	return func(p *ChatParams) {
		p.TopP = &topP
	}
}

func WithTopK(topK int) ChatParamOption {
	return func(p *ChatParams) {
		p.TopK = &topK
	}
}

func WithTools(tools []Tool) ChatParamOption {
	return func(p *ChatParams) {
		p.Tools = append(p.Tools, tools...)
	}
}

func WithToolChoice(toolChoice ToolChoice) ChatParamOption {
	return func(p *ChatParams) {
		p.ToolChoice = &toolChoice
	}
}

func WithExtras(extras map[string]any) ChatParamOption {
	return func(p *ChatParams) {
		if len(extras) == 0 {
			return
		}
		if p.Extra == nil {
			p.Extra = make(map[string]any, len(extras))
		}
		for k, v := range extras {
			p.Extra[k] = v
		}
	}
}

// ChatResponse represents the response from a chat completion request.
type ChatResponse struct {
	ID      string
	Created int64
	Model   string
	Choices []*Choice
	Usage   *Usage

	// Provider-specific extras
	Extra map[string]any `json:"-"`
}

// Choice represents a single completion choice in the response.
type Choice struct {
	Index        int
	Message      *Message
	FinishReason string
}

// Usage represents token usage statistics for the request.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// ToolChoiceMode represents the mode for tool selection.
type ToolChoiceMode string

const (
	ToolChoiceModeAuto     ToolChoiceMode = "auto"     // Model decides whether to use tools
	ToolChoiceModeRequired ToolChoiceMode = "required" // Model must use a tool
	ToolChoiceModeNone     ToolChoiceMode = "none"     // Model must not use tools
	ToolChoiceModeTool     ToolChoiceMode = "tool"     // Model must use a specific tool
)

// ToolChoice controls how the model uses tools.
type ToolChoice struct {
	Mode ToolChoiceMode `json:"-"`
	Name string         `json:"-"` // Only used when Mode == ToolChoiceModeTool
}

// ToolChoiceAuto creates a ToolChoice that lets the model decide.
func ToolChoiceAuto() *ToolChoice {
	return &ToolChoice{Mode: ToolChoiceModeAuto}
}

// ToolChoiceRequired creates a ToolChoice that requires tool usage.
func ToolChoiceRequired() *ToolChoice {
	return &ToolChoice{Mode: ToolChoiceModeRequired}
}

// ToolChoiceNone creates a ToolChoice that prevents tool usage.
func ToolChoiceNone() *ToolChoice {
	return &ToolChoice{Mode: ToolChoiceModeNone}
}

// ToolChoiceTool creates a ToolChoice that forces a specific tool.
func ToolChoiceTool(tool Tool) *ToolChoice {
	return &ToolChoice{Mode: ToolChoiceModeTool, Name: tool.Name()}
}

// ToolChoiceToolWithName creates a ToolChoice with a tool name.
func ToolChoiceToolWithName(name string) *ToolChoice {
	return &ToolChoice{Mode: ToolChoiceModeTool, Name: name}
}

// MarshalJSON implements json.Marshaler for ToolChoice.
func (tc *ToolChoice) MarshalJSON() ([]byte, error) {
	if tc.Mode == ToolChoiceModeTool {
		return json.Marshal(map[string]any{
			"type": ToolChoiceModeTool,
			"name": tc.Name,
		})
	}
	return json.Marshal(string(tc.Mode))
}

// UnmarshalJSON implements json.Unmarshaler for ToolChoice.
func (tc *ToolChoice) UnmarshalJSON(data []byte) error {
	// Try string first (auto, required, none)
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		tc.Mode = ToolChoiceMode(s)
		return nil
	}

	// Try object (tool with name)
	var obj struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	tc.Mode = ToolChoiceModeTool
	tc.Name = obj.Name
	return nil
}
