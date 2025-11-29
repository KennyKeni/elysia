package types

import (
	"errors"
	"testing"
)

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		// Clean JSON
		{
			name:  "valid object as-is",
			input: `{"city": "NYC", "temp": 72}`,
			want:  `{"city": "NYC", "temp": 72}`,
		},
		{
			name:  "valid array as-is",
			input: `[1, 2, 3]`,
			want:  `[1, 2, 3]`,
		},
		{
			name:  "valid object with whitespace",
			input: `  {"city": "NYC"}  `,
			want:  `{"city": "NYC"}`,
		},

		// Markdown code blocks
		{
			name:  "markdown json block",
			input: "```json\n{\"city\": \"NYC\"}\n```",
			want:  `{"city": "NYC"}`,
		},
		{
			name:  "markdown block without language",
			input: "```\n{\"city\": \"NYC\"}\n```",
			want:  `{"city": "NYC"}`,
		},
		{
			name:  "markdown block with extra whitespace",
			input: "```json\n  {\"city\": \"NYC\"}  \n```",
			want:  `{"city": "NYC"}`,
		},

		// Prose around JSON
		{
			name:  "prose before object",
			input: `Here is the result: {"city": "NYC"}`,
			want:  `{"city": "NYC"}`,
		},
		{
			name:  "prose after object",
			input: `{"city": "NYC"} That's the weather data.`,
			want:  `{"city": "NYC"}`,
		},
		{
			name:  "prose around object",
			input: `Here's what I found: {"city": "NYC", "temp": 72} Hope that helps!`,
			want:  `{"city": "NYC", "temp": 72}`,
		},
		{
			name:  "prose before array",
			input: `The results are: [1, 2, 3]`,
			want:  `[1, 2, 3]`,
		},

		// Nested structures
		{
			name:  "nested objects",
			input: `{"outer": {"inner": "value"}}`,
			want:  `{"outer": {"inner": "value"}}`,
		},
		{
			name:  "nested arrays",
			input: `[[1, 2], [3, 4]]`,
			want:  `[[1, 2], [3, 4]]`,
		},
		{
			name:  "mixed nesting",
			input: `{"items": [{"id": 1}, {"id": 2}]}`,
			want:  `{"items": [{"id": 1}, {"id": 2}]}`,
		},

		// Braces in strings (edge cases)
		{
			name:  "brace in string value",
			input: `{"message": "use { and } carefully"}`,
			want:  `{"message": "use { and } carefully"}`,
		},
		{
			name:  "bracket in string value",
			input: `{"message": "array looks like [1,2]"}`,
			want:  `{"message": "array looks like [1,2]"}`,
		},
		{
			name:  "escaped quotes in string",
			input: `{"message": "he said \"hello\""}`,
			want:  `{"message": "he said \"hello\""}`,
		},
		{
			name:  "prose with brace in string",
			input: `Result: {"msg": "contains { brace"}`,
			want:  `{"msg": "contains { brace"}`,
		},

		// Error cases
		{
			name:    "no JSON at all",
			input:   "This is just plain text",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "incomplete object",
			input:   `{"city": "NYC"`,
			wantErr: true,
		},
		{
			name:    "incomplete array",
			input:   `[1, 2, 3`,
			wantErr: true,
		},
		{
			name:    "invalid JSON syntax",
			input:   `{city: NYC}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractJSON(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ExtractJSON() expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Errorf("ExtractJSON() unexpected error: %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("ExtractJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildOutputToolDefinition(t *testing.T) {
	rf := ResponseFormat{
		Name:        "WeatherResponse",
		Description: "Weather data for a city",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"city": map[string]any{"type": "string"},
				"temp": map[string]any{"type": "number"},
			},
		},
	}

	tool := BuildOutputToolDefinition(rf)

	if tool.Name != OutputToolName {
		t.Errorf("tool.Name = %q, want %q", tool.Name, OutputToolName)
	}
	if tool.InputSchema == nil {
		t.Error("tool.InputSchema is nil")
	}
	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}
}

func TestBuildPromptedSuffix(t *testing.T) {
	rf := ResponseFormat{
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"city": map[string]any{"type": "string"},
			},
		},
	}

	suffix := BuildPromptedSuffix(rf)

	if suffix == "" {
		t.Error("BuildPromptedSuffix() returned empty string")
	}
	if !contains(suffix, "JSON") {
		t.Error("suffix should mention JSON")
	}
	if !contains(suffix, "schema") || !contains(suffix, "Schema") {
		t.Error("suffix should mention schema")
	}
}

func TestResponseFormatFor(t *testing.T) {
	type TestOutput struct {
		City string `json:"city"`
		Temp int    `json:"temp"`
	}

	rf, err := ResponseFormatFor[TestOutput](ResponseFormatModeNative, "test", "test output")
	if err != nil {
		t.Fatalf("ResponseFormatFor() error: %v", err)
	}

	if rf.Mode != ResponseFormatModeNative {
		t.Errorf("rf.Mode = %v, want %v", rf.Mode, ResponseFormatModeNative)
	}
	if rf.Name != "test" {
		t.Errorf("rf.Name = %q, want %q", rf.Name, "test")
	}
	if rf.Schema == nil {
		t.Error("rf.Schema is nil")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Test schema for structured output tests
func testSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"city": map[string]any{"type": "string"},
			"temp": map[string]any{"type": "number"},
		},
		"required": []any{"city", "temp"},
	}
}

func TestApplyResponseFormat_NoSchema(t *testing.T) {
	params := &ChatParams{
		Model:        "test-model",
		SystemPrompt: "You are helpful.",
	}

	ApplyResponseFormat(params)

	// Should not modify anything when no schema
	if len(params.Tools) != 0 {
		t.Errorf("expected no tools, got %d", len(params.Tools))
	}
	if params.SystemPrompt != "You are helpful." {
		t.Errorf("system prompt was modified: %q", params.SystemPrompt)
	}
}

func TestApplyResponseFormat_ToolMode(t *testing.T) {
	params := &ChatParams{
		Model:        "test-model",
		SystemPrompt: "You are helpful.",
		Tools: []ToolDefinition{
			{Name: "get_weather", Description: "Get weather"},
		},
		ResponseFormat: ResponseFormat{
			Mode:        ResponseFormatModeTool,
			Name:        "WeatherResponse",
			Description: "Weather data",
			Schema:      testSchema(),
		},
	}

	ApplyResponseFormat(params)

	// Should add _output tool
	if len(params.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(params.Tools))
	}

	// Find the _output tool
	var outputTool *ToolDefinition
	for i := range params.Tools {
		if params.Tools[i].Name == OutputToolName {
			outputTool = &params.Tools[i]
			break
		}
	}

	if outputTool == nil {
		t.Fatal("_output tool not found")
	}
	if outputTool.InputSchema == nil {
		t.Error("_output tool has no input schema")
	}
	if !contains(outputTool.Description, "WeatherResponse") {
		t.Errorf("description should contain name, got: %s", outputTool.Description)
	}
}

func TestApplyResponseFormat_PromptedMode(t *testing.T) {
	params := &ChatParams{
		Model:        "test-model",
		SystemPrompt: "You are helpful.",
		ResponseFormat: ResponseFormat{
			Mode:   ResponseFormatModePrompted,
			Schema: testSchema(),
		},
	}

	originalPrompt := params.SystemPrompt
	ApplyResponseFormat(params)

	// Should append to system prompt
	if params.SystemPrompt == originalPrompt {
		t.Error("system prompt was not modified")
	}
	if !contains(params.SystemPrompt, "JSON") {
		t.Error("system prompt should mention JSON")
	}
	if !contains(params.SystemPrompt, originalPrompt) {
		t.Error("original prompt should be preserved")
	}
}

func TestApplyResponseFormat_NativeMode(t *testing.T) {
	params := &ChatParams{
		Model:        "test-model",
		SystemPrompt: "You are helpful.",
		ResponseFormat: ResponseFormat{
			Mode:   ResponseFormatModeNative,
			Schema: testSchema(),
		},
	}

	originalPrompt := params.SystemPrompt
	toolCount := len(params.Tools)

	ApplyResponseFormat(params)

	// Native mode should not modify params (adapter handles it)
	if params.SystemPrompt != originalPrompt {
		t.Error("system prompt should not be modified in native mode")
	}
	if len(params.Tools) != toolCount {
		t.Error("tools should not be modified in native mode")
	}
}

func TestExtractStructuredContent_NoSchema(t *testing.T) {
	rf := ResponseFormat{} // No schema
	msg := &Message{
		Role:        RoleAssistant,
		ContentPart: []ContentPart{&ContentPartText{Text: "Hello world"}},
	}

	content, err := ExtractStructuredContent(rf, msg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if content != "" {
		t.Errorf("expected empty content, got %q", content)
	}
}

func TestExtractStructuredContent_NativeMode_ValidJSON(t *testing.T) {
	rf := ResponseFormat{
		Mode:   ResponseFormatModeNative,
		Schema: testSchema(),
	}
	msg := &Message{
		Role:        RoleAssistant,
		ContentPart: []ContentPart{&ContentPartText{Text: `{"city": "NYC", "temp": 72}`}},
	}

	content, err := ExtractStructuredContent(rf, msg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if content != `{"city": "NYC", "temp": 72}` {
		t.Errorf("got %q, want %q", content, `{"city": "NYC", "temp": 72}`)
	}
}

func TestExtractStructuredContent_NativeMode_InvalidSchema(t *testing.T) {
	rf := ResponseFormat{
		Mode:   ResponseFormatModeNative,
		Schema: testSchema(),
	}
	msg := &Message{
		Role:        RoleAssistant,
		ContentPart: []ContentPart{&ContentPartText{Text: `{"city": "NYC"}`}}, // Missing required "temp"
	}

	_, err := ExtractStructuredContent(rf, msg)
	if err == nil {
		t.Error("expected schema validation error")
	}

	var schemaErr *SchemaValidationError
	if !errors.As(err, &schemaErr) {
		t.Errorf("expected SchemaValidationError, got %T: %v", err, err)
	}
}

func TestExtractStructuredContent_ToolMode_Success(t *testing.T) {
	rf := ResponseFormat{
		Mode:   ResponseFormatModeTool,
		Schema: testSchema(),
	}
	msg := &Message{
		Role:        RoleAssistant,
		ContentPart: []ContentPart{},
		ToolCalls: []ToolCall{
			{
				ID: "call_123",
				Function: ToolFunction{
					Name:      OutputToolName,
					Arguments: map[string]any{"city": "NYC", "temp": float64(72)},
				},
			},
		},
	}

	content, err := ExtractStructuredContent(rf, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should extract content
	if content == "" {
		t.Error("expected content to be extracted")
	}
	if !contains(content, "NYC") || !contains(content, "72") {
		t.Errorf("content should contain city and temp: %q", content)
	}

	// Role should remain RoleAssistant
	if msg.Role != RoleAssistant {
		t.Errorf("message role = %v, want %v", msg.Role, RoleAssistant)
	}

	// Should transform message: remove tool call, add as text
	if len(msg.ToolCalls) != 0 {
		t.Errorf("expected tool calls to be removed, got %d", len(msg.ToolCalls))
	}
	if len(msg.ContentPart) != 1 {
		t.Errorf("expected 1 content part, got %d", len(msg.ContentPart))
	}
	textPart, ok := msg.ContentPart[0].(*ContentPartText)
	if !ok {
		t.Errorf("expected ContentPartText, got %T", msg.ContentPart[0])
	} else if textPart.Text != content {
		t.Errorf("text content should match extracted content")
	}
}

func TestExtractStructuredContent_ToolMode_OutputWithOtherTools(t *testing.T) {
	rf := ResponseFormat{
		Mode:   ResponseFormatModeTool,
		Schema: testSchema(),
	}
	msg := &Message{
		Role:        RoleAssistant,
		ContentPart: []ContentPart{},
		ToolCalls: []ToolCall{
			{
				ID: "call_123",
				Function: ToolFunction{
					Name:      OutputToolName,
					Arguments: map[string]any{"city": "NYC", "temp": float64(72)},
				},
			},
			{
				ID: "call_456",
				Function: ToolFunction{
					Name:      "get_weather",
					Arguments: map[string]any{"location": "NYC"},
				},
			},
		},
	}

	_, err := ExtractStructuredContent(rf, msg)
	if err == nil {
		t.Error("expected OutputToolMisuseError")
	}

	var misuseErr *OutputToolMisuseError
	if !errors.As(err, &misuseErr) {
		t.Errorf("expected OutputToolMisuseError, got %T: %v", err, err)
	} else {
		if len(misuseErr.OtherTools) != 1 || misuseErr.OtherTools[0] != "get_weather" {
			t.Errorf("OtherTools = %v, want [get_weather]", misuseErr.OtherTools)
		}
	}
}

func TestExtractStructuredContent_ToolMode_NoToolCalls(t *testing.T) {
	rf := ResponseFormat{
		Mode:   ResponseFormatModeTool,
		Schema: testSchema(),
	}
	msg := &Message{
		Role:        RoleAssistant,
		ContentPart: []ContentPart{&ContentPartText{Text: "I'll help you with the weather."}},
		ToolCalls:   nil,
	}

	_, err := ExtractStructuredContent(rf, msg)
	if err == nil {
		t.Error("expected ToolNotCalledError")
	}

	var notCalledErr *ToolNotCalledError
	if !errors.As(err, &notCalledErr) {
		t.Errorf("expected ToolNotCalledError, got %T: %v", err, err)
	} else {
		if notCalledErr.ExpectedTool != OutputToolName {
			t.Errorf("ExpectedTool = %q, want %q", notCalledErr.ExpectedTool, OutputToolName)
		}
	}
}

func TestExtractStructuredContent_ToolMode_OtherToolsOnly(t *testing.T) {
	rf := ResponseFormat{
		Mode:   ResponseFormatModeTool,
		Schema: testSchema(),
	}
	msg := &Message{
		Role:        RoleAssistant,
		ContentPart: []ContentPart{},
		ToolCalls: []ToolCall{
			{
				ID: "call_456",
				Function: ToolFunction{
					Name:      "get_weather",
					Arguments: map[string]any{"location": "NYC"},
				},
			},
		},
	}

	// When other tools are called (not _output), content is empty and agent loop continues
	content, err := ExtractStructuredContent(rf, msg)
	if err != nil {
		t.Errorf("unexpected error when other tools called: %v", err)
	}
	if content != "" {
		t.Errorf("expected empty content when other tools called, got %q", content)
	}

	// Message should not be modified
	if len(msg.ToolCalls) != 1 {
		t.Error("tool calls should not be modified")
	}
}

func TestExtractStructuredContent_ToolMode_InvalidSchema(t *testing.T) {
	rf := ResponseFormat{
		Mode:   ResponseFormatModeTool,
		Schema: testSchema(),
	}
	msg := &Message{
		Role:        RoleAssistant,
		ContentPart: []ContentPart{},
		ToolCalls: []ToolCall{
			{
				ID: "call_123",
				Function: ToolFunction{
					Name:      OutputToolName,
					Arguments: map[string]any{"city": "NYC"}, // Missing required "temp"
				},
			},
		},
	}

	_, err := ExtractStructuredContent(rf, msg)
	if err == nil {
		t.Error("expected schema validation error")
	}

	var schemaErr *SchemaValidationError
	if !errors.As(err, &schemaErr) {
		t.Errorf("expected SchemaValidationError, got %T: %v", err, err)
	}
}

func TestExtractStructuredContent_PromptedMode_ValidJSON(t *testing.T) {
	rf := ResponseFormat{
		Mode:   ResponseFormatModePrompted,
		Schema: testSchema(),
	}
	msg := &Message{
		Role:        RoleAssistant,
		ContentPart: []ContentPart{&ContentPartText{Text: `{"city": "NYC", "temp": 72}`}},
	}

	content, err := ExtractStructuredContent(rf, msg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if content != `{"city": "NYC", "temp": 72}` {
		t.Errorf("got %q, want %q", content, `{"city": "NYC", "temp": 72}`)
	}
}

func TestExtractStructuredContent_PromptedMode_JSONInProse(t *testing.T) {
	rf := ResponseFormat{
		Mode:   ResponseFormatModePrompted,
		Schema: testSchema(),
	}
	msg := &Message{
		Role: RoleAssistant,
		ContentPart: []ContentPart{
			&ContentPartText{Text: `Here's the weather: {"city": "NYC", "temp": 72} Hope that helps!`},
		},
	}

	content, err := ExtractStructuredContent(rf, msg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if content != `{"city": "NYC", "temp": 72}` {
		t.Errorf("got %q, want %q", content, `{"city": "NYC", "temp": 72}`)
	}
}

func TestExtractStructuredContent_PromptedMode_MarkdownBlock(t *testing.T) {
	rf := ResponseFormat{
		Mode:   ResponseFormatModePrompted,
		Schema: testSchema(),
	}
	msg := &Message{
		Role: RoleAssistant,
		ContentPart: []ContentPart{
			&ContentPartText{Text: "```json\n{\"city\": \"NYC\", \"temp\": 72}\n```"},
		},
	}

	content, err := ExtractStructuredContent(rf, msg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if content != `{"city": "NYC", "temp": 72}` {
		t.Errorf("got %q, want %q", content, `{"city": "NYC", "temp": 72}`)
	}
}

func TestExtractStructuredContent_PromptedMode_NoJSON(t *testing.T) {
	rf := ResponseFormat{
		Mode:   ResponseFormatModePrompted,
		Schema: testSchema(),
	}
	msg := &Message{
		Role:        RoleAssistant,
		ContentPart: []ContentPart{&ContentPartText{Text: "I don't know the weather."}},
	}

	_, err := ExtractStructuredContent(rf, msg)
	if err == nil {
		t.Error("expected error when no JSON found")
	}
}

func TestExtractStructuredContent_PromptedMode_InvalidSchema(t *testing.T) {
	rf := ResponseFormat{
		Mode:   ResponseFormatModePrompted,
		Schema: testSchema(),
	}
	msg := &Message{
		Role:        RoleAssistant,
		ContentPart: []ContentPart{&ContentPartText{Text: `{"city": "NYC"}`}}, // Missing required "temp"
	}

	_, err := ExtractStructuredContent(rf, msg)
	if err == nil {
		t.Error("expected schema validation error")
	}

	var schemaErr *SchemaValidationError
	if !errors.As(err, &schemaErr) {
		t.Errorf("expected SchemaValidationError, got %T: %v", err, err)
	}
}

func TestExtractStructuredContent_UnsupportedMode(t *testing.T) {
	rf := ResponseFormat{
		Mode:   ResponseFormatMode("unsupported"),
		Schema: testSchema(),
	}
	msg := &Message{
		Role:        RoleAssistant,
		ContentPart: []ContentPart{&ContentPartText{Text: `{"city": "NYC", "temp": 72}`}},
	}

	_, err := ExtractStructuredContent(rf, msg)
	if err == nil {
		t.Error("expected error for unsupported mode")
	}
	if !errors.Is(err, ErrUnsupportedResponseMode) {
		t.Errorf("expected ErrUnsupportedResponseMode, got %v", err)
	}
}

func TestExtractStructuredContent_ToolMode_PreservesExistingContent(t *testing.T) {
	rf := ResponseFormat{
		Mode:   ResponseFormatModeTool,
		Schema: testSchema(),
	}
	msg := &Message{
		Role: RoleAssistant,
		ContentPart: []ContentPart{
			&ContentPartText{Text: "Let me get that weather for you."},
		},
		ToolCalls: []ToolCall{
			{
				ID: "call_123",
				Function: ToolFunction{
					Name:      OutputToolName,
					Arguments: map[string]any{"city": "NYC", "temp": float64(72)},
				},
			},
		},
	}

	content, err := ExtractStructuredContent(rf, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 2 content parts: original text + extracted JSON
	if len(msg.ContentPart) != 2 {
		t.Errorf("expected 2 content parts, got %d", len(msg.ContentPart))
	}

	// First should be original text
	if textPart, ok := msg.ContentPart[0].(*ContentPartText); !ok {
		t.Errorf("first part should be text, got %T", msg.ContentPart[0])
	} else if textPart.Text != "Let me get that weather for you." {
		t.Errorf("first part text = %q", textPart.Text)
	}

	// Second should be extracted JSON
	if textPart, ok := msg.ContentPart[1].(*ContentPartText); !ok {
		t.Errorf("second part should be text, got %T", msg.ContentPart[1])
	} else if textPart.Text != content {
		t.Errorf("second part should match extracted content")
	}
}

func TestBuildOutputToolDefinition_CustomDescription(t *testing.T) {
	rf := ResponseFormat{
		Name:        "MyOutput",
		Description: "Custom description here",
		Schema:      testSchema(),
	}

	tool := BuildOutputToolDefinition(rf)

	if !contains(tool.Description, "MyOutput") {
		t.Errorf("description should contain name: %s", tool.Description)
	}
	if !contains(tool.Description, "Custom description here") {
		t.Errorf("description should contain custom description: %s", tool.Description)
	}
}

func TestBuildOutputToolDefinition_DefaultDescription(t *testing.T) {
	rf := ResponseFormat{
		Schema: testSchema(),
	}

	tool := BuildOutputToolDefinition(rf)

	if !contains(tool.Description, "Structured output") {
		t.Errorf("should have default description: %s", tool.Description)
	}
	if !contains(tool.Description, "NEVER call other tools") {
		t.Errorf("should warn about not calling other tools: %s", tool.Description)
	}
}

func TestExtractStructuredContent_ToolMode_MultipleOutputCalls(t *testing.T) {
	// Edge case: _output called multiple times (should fail as misuse)
	rf := ResponseFormat{
		Mode:   ResponseFormatModeTool,
		Schema: testSchema(),
	}
	msg := &Message{
		Role:        RoleAssistant,
		ContentPart: []ContentPart{},
		ToolCalls: []ToolCall{
			{
				ID: "call_123",
				Function: ToolFunction{
					Name:      OutputToolName,
					Arguments: map[string]any{"city": "NYC", "temp": float64(72)},
				},
			},
			{
				ID: "call_456",
				Function: ToolFunction{
					Name:      OutputToolName,
					Arguments: map[string]any{"city": "LA", "temp": float64(85)},
				},
			},
		},
	}

	// Multiple _output calls is also a misuse (len > 1 with _output)
	_, err := ExtractStructuredContent(rf, msg)
	if err == nil {
		t.Error("expected error for multiple _output calls")
	}

	// This hits the "len(msg.ToolCalls) > 1" check
	var misuseErr *OutputToolMisuseError
	if !errors.As(err, &misuseErr) {
		t.Errorf("expected OutputToolMisuseError, got %T: %v", err, err)
	}
}
