package types

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const OutputToolName = "_output"

// BuildOutputToolDefinition creates the hidden _output tool for Tool mode
func BuildOutputToolDefinition(rf ResponseFormat) ToolDefinition {
	description := rf.Description
	if description == "" {
		description = "Structured output tool"
	}
	if rf.Name != "" {
		description = rf.Name + ": " + description
	}

	return ToolDefinition{
		Name:        OutputToolName,
		Description: description,
		InputSchema: rf.Schema,
	}
}

// BuildPromptedSuffix creates the instruction suffix for Prompted mode
func BuildPromptedSuffix(rf ResponseFormat) string {
	schemaJSON, _ := json.Marshal(rf.Schema)
	return fmt.Sprintf("\n\nYou must respond with valid JSON matching this schema. Do not include any other text, only the JSON object.\n\nSchema:\n%s", schemaJSON)
}

// ResponseFormatFor creates a ResponseFormat from a Go type
func ResponseFormatFor[T any](mode ResponseFormatMode, name, description string) (ResponseFormat, error) {
	schema, err := SchemaMapFor[T]()
	if err != nil {
		return ResponseFormat{}, fmt.Errorf("failed to generate schema: %w", err)
	}

	return ResponseFormat{
		Mode:        mode,
		Name:        name,
		Description: description,
		Schema:      schema,
	}, nil
}

// ExtractJSON attempts to extract a JSON object or array from text.
// Handles cases where the model includes prose or Markdown around the JSON.
func ExtractJSON(text string) (string, error) {
	text = strings.TrimSpace(text)

	// 1. Try as-is
	if isValidJSON(text) {
		return text, nil
	}

	// 2. Try Markdown code block: ```json ... ``` or ``` ... ```
	re := regexp.MustCompile("```(?:json)?\\s*([\\s\\S]*?)```")
	if matches := re.FindStringSubmatch(text); len(matches) > 1 {
		candidate := strings.TrimSpace(matches[1])
		if isValidJSON(candidate) {
			return candidate, nil
		}
	}

	// 3. Find first { or [ and match braces
	startObj := strings.Index(text, "{")
	startArr := strings.Index(text, "[")

	start := -1
	openBrace, closeBrace := '{', '}'
	if startObj != -1 && (startArr == -1 || startObj < startArr) {
		start = startObj
	} else if startArr != -1 {
		start = startArr
		openBrace, closeBrace = '[', ']'
	}

	if start != -1 {
		end := findMatchingBrace(text[start:], openBrace, closeBrace)
		if end != -1 {
			candidate := text[start : start+end+1]
			if isValidJSON(candidate) {
				return candidate, nil
			}
		}
	}

	return "", errors.New("no valid JSON found")
}

func findMatchingBrace(s string, open, close rune) int {
	depth := 0
	inString := false
	escape := false

	for i, c := range s {
		if escape {
			escape = false
			continue
		}
		if c == '\\' && inString {
			escape = true
			continue
		}
		if c == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if c == open {
			depth++
		} else if c == close {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
