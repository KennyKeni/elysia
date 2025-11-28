package types

import (
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
