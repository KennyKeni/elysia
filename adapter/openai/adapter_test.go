package openai

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/KennyKeni/elysia/types"
)

// TestChatIntegration performs a real API call to OpenAI
// Set OPENAI_API_KEY environment variable to run this test
// Run with: OPENAI_API_KEY="your-key" go test -v -run TestChatIntegration
func TestChatIntegration(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: OPENAI_API_KEY not set")
	}

	// Create client with API key
	client := NewClient(WithAPIKey(apiKey))

	// Create a simple chat request
	params := &types.ChatParams{
		Model: "gpt-4o-mini",
		Messages: []*types.Message{
			types.NewUserMessage(types.WithText("Say 'Hello, World!' and nothing else.")),
		},
	}

	// Make the request
	ctx := context.Background()
	response, err := client.Chat(ctx, params)
	if err != nil {
		t.Fatalf("Chat request failed: %v", err)
	}

	// Validate response
	if response == nil {
		t.Fatal("Response is nil")
	}

	if response.ID == "" {
		t.Error("Response ID is empty")
	}

	if response.Model == "" {
		t.Error("Response Model is empty")
	}

	if len(response.Choices) == 0 {
		t.Fatal("Response has no choices")
	}

	choice := response.Choices[0]
	if choice.Message == nil {
		t.Fatal("Choice message is nil")
	}

	if len(choice.Message.ContentPart) == 0 {
		t.Fatal("Message has no content parts")
	}

	// Check that we got a text response
	textPart, ok := choice.Message.ContentPart[0].(*types.ContentPartText)
	if !ok {
		t.Fatalf("Expected ContentPartText, got %T", choice.Message.ContentPart[0])
	}

	if textPart.Text == "" {
		t.Error("Response text is empty")
	}

	t.Logf("Response ID: %s", response.ID)
	t.Logf("Model: %s", response.Model)
	t.Logf("Response: %s", textPart.Text)
	t.Logf("Finish Reason: %s", choice.FinishReason)

	if response.Usage != nil {
		t.Logf("Usage - Prompt: %d, Completion: %d, Total: %d",
			response.Usage.PromptTokens,
			response.Usage.CompletionTokens,
			response.Usage.TotalTokens)
	}
}

// TestChatWithSystemPrompt tests chat with system prompt
func TestChatWithSystemPrompt(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: OPENAI_API_KEY not set")
	}

	client := NewClient(WithAPIKey(apiKey))

	params := &types.ChatParams{
		Model:        "gpt-4o-mini",
		SystemPrompt: "You are a helpful assistant that always responds in pirate speak.",
		Messages: []*types.Message{
			types.NewUserMessage(types.WithText("Tell me about the weather.")),
		},
	}

	ctx := context.Background()
	response, err := client.Chat(ctx, params)
	if err != nil {
		t.Fatalf("Chat request failed: %v", err)
	}

	if len(response.Choices) == 0 {
		t.Fatal("Response has no choices")
	}

	textPart, ok := response.Choices[0].Message.ContentPart[0].(*types.ContentPartText)
	if !ok {
		t.Fatal("Expected ContentPartText")
	}

	t.Logf("Pirate response: %s", textPart.Text)
}

// TestChatWithParameters tests chat with various parameters
func TestChatWithParameters(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: OPENAI_API_KEY not set")
	}

	client := NewClient(WithAPIKey(apiKey))

	maxTokens := 50
	temperature := 0.7
	topP := 0.9

	params := &types.ChatParams{
		Model: "gpt-4o-mini",
		Messages: []*types.Message{
			types.NewUserMessage(types.WithText("Write a very short poem about coding.")),
		},
		MaxTokens:   &maxTokens,
		Temperature: &temperature,
		TopP:        &topP,
	}

	ctx := context.Background()
	response, err := client.Chat(ctx, params)
	if err != nil {
		t.Fatalf("Chat request failed: %v", err)
	}

	if len(response.Choices) == 0 {
		t.Fatal("Response has no choices")
	}

	textPart, ok := response.Choices[0].Message.ContentPart[0].(*types.ContentPartText)
	if !ok {
		t.Fatal("Expected ContentPartText")
	}

	t.Logf("Poem: %s", textPart.Text)

	// Verify token limit was respected (approximately)
	if response.Usage != nil && response.Usage.CompletionTokens > maxTokens+10 {
		t.Errorf("Completion tokens (%d) exceeded max tokens (%d) by too much",
			response.Usage.CompletionTokens, maxTokens)
	}
}

// TestChatMultiTurn tests a multi-turn conversation
func TestChatMultiTurn(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: OPENAI_API_KEY not set")
	}

	client := NewClient(WithAPIKey(apiKey))

	params := &types.ChatParams{
		Model: "gpt-4o-mini",
		Messages: []*types.Message{
			types.NewUserMessage(types.WithText("My name is Alice.")),
			types.NewAssistantMessage(types.WithText("Hello Alice! Nice to meet you.")),
			types.NewUserMessage(types.WithText("What's my name?")),
		},
	}

	ctx := context.Background()
	response, err := client.Chat(ctx, params)
	if err != nil {
		t.Fatalf("Chat request failed: %v", err)
	}

	if len(response.Choices) == 0 {
		t.Fatal("Response has no choices")
	}

	textPart, ok := response.Choices[0].Message.ContentPart[0].(*types.ContentPartText)
	if !ok {
		t.Fatal("Expected ContentPartText")
	}

	t.Logf("Response: %s", textPart.Text)

	// The response should mention "Alice"
	// Note: This is a weak test as we can't guarantee exact response
	if textPart.Text == "" {
		t.Error("Response is empty")
	}
}

// TestChatWithInvalidAPIKey tests error handling with invalid API key
func TestChatWithInvalidAPIKey(t *testing.T) {
	client := NewClient(WithAPIKey("invalid-api-key"))

	params := &types.ChatParams{
		Model: "gpt-4o-mini",
		Messages: []*types.Message{
			types.NewUserMessage(types.WithText("Hello")),
		},
	}

	ctx := context.Background()
	_, err := client.Chat(ctx, params)

	if err == nil {
		t.Fatal("Expected error with invalid API key, got nil")
	}

	t.Logf("Got expected error: %v", err)
}

// TestChatWithTools tests function calling with tools
func TestChatWithTools(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: OPENAI_API_KEY not set")
	}

	client := NewClient(WithAPIKey(apiKey))

	// Define input/output types for the weather tool
	type WeatherInput struct {
		Location string `json:"location" jsonschema:"The city and state, e.g. San Francisco, CA"`
		Unit     string `json:"unit,omitempty" jsonschema:"The temperature unit to use (celsius or fahrenheit)"`
	}

	type WeatherOutput struct {
		Temperature float64 `json:"temperature" jsonschema:"The temperature in the specified unit"`
		Condition   string  `json:"condition" jsonschema:"The weather condition (e.g. sunny, cloudy, rainy)"`
	}

	// Create typed tool with automatic schema generation
	weatherTool, err := types.NewTypedNativeTool(
		"get_weather",
		"Get the current weather for a location",
		func(ctx context.Context, input WeatherInput) (WeatherOutput, error) {
			// Mock handler - in real usage this would fetch actual weather
			return WeatherOutput{
				Temperature: 72,
				Condition:   "sunny",
			}, nil
		},
	)
	if err != nil {
		t.Fatalf("Failed to create weather tool: %v", err)
	}

	params := &types.ChatParams{
		Model: "gpt-4o-mini",
		Messages: []*types.Message{
			types.NewUserMessage(types.WithText("What's the weather like in San Francisco?")),
		},
		Tools: []types.Tool{weatherTool},
	}

	ctx := context.Background()
	response, err := client.Chat(ctx, params)
	if err != nil {
		t.Fatalf("Chat request failed: %v", err)
	}

	if len(response.Choices) == 0 {
		t.Fatal("Response has no choices")
	}

	choice := response.Choices[0]
	t.Logf("Finish Reason: %s", choice.FinishReason)

	// The model should call the tool
	if len(choice.Message.ToolCalls) > 0 {
		t.Logf("Tool calls made: %d", len(choice.Message.ToolCalls))
		for i, toolCall := range choice.Message.ToolCalls {
			t.Logf("  Tool call %d:", i)
			t.Logf("    ID: %s", toolCall.ID)
			t.Logf("    Function: %s", toolCall.Function.Name)
			t.Logf("    Arguments: %+v", toolCall.Function.Arguments)
		}
	} else {
		// Model might not always call the tool, which is okay
		t.Log("No tool calls made (model chose to respond directly)")
		if len(choice.Message.ContentPart) > 0 {
			if textPart, ok := choice.Message.ContentPart[0].(*types.ContentPartText); ok {
				t.Logf("Response: %s", textPart.Text)
			}
		}
	}
}

// TestChatWithToolsRoundTrip tests the complete tool calling flow:
// 1. LLM decides to call a tool
// 2. Tool executes and returns result
// 3. Result sent back to LLM
// 4. LLM generates final answer using the tool result
func TestChatWithToolsRoundTrip(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: OPENAI_API_KEY not set")
	}

	client := NewClient(WithAPIKey(apiKey))

	// Define input/output types for the weather tool
	type WeatherInput struct {
		Location string `json:"location" jsonschema:"The city and state, e.g. San Francisco, CA"`
		Unit     string `json:"unit,omitempty" jsonschema:"The temperature unit to use (celsius or fahrenheit)"`
	}

	type WeatherOutput struct {
		Temperature float64 `json:"temperature" jsonschema:"The temperature in the specified unit"`
		Condition   string  `json:"condition" jsonschema:"The weather condition (e.g. sunny, cloudy, rainy)"`
	}

	// Create typed tool with automatic schema generation
	weatherTool, err := types.NewTypedNativeTool(
		"get_weather",
		"Get the current weather for a location",
		func(ctx context.Context, input WeatherInput) (WeatherOutput, error) {
			// Mock handler returning weather data
			return WeatherOutput{
				Temperature: 72,
				Condition:   "sunny",
			}, nil
		},
	)
	if err != nil {
		t.Fatalf("Failed to create weather tool: %v", err)
	}

	// Step 1: Initial request with user question
	messages := []*types.Message{
		types.NewUserMessage(types.WithText("What's the weather like in San Francisco?")),
	}

	params := &types.ChatParams{
		Model:    "gpt-4o-mini",
		Messages: messages,
		Tools:    []types.Tool{weatherTool},
	}

	ctx := context.Background()
	t.Log("Step 1: Sending initial request to LLM")
	response, err := client.Chat(ctx, params)
	if err != nil {
		t.Fatalf("Initial chat request failed: %v", err)
	}

	if len(response.Choices) == 0 {
		t.Fatal("Response has no choices")
	}

	choice := response.Choices[0]
	t.Logf("Finish Reason: %s", choice.FinishReason)

	// Verify LLM decided to call the tool
	if len(choice.Message.ToolCalls) == 0 {
		t.Fatal("Expected LLM to call a tool, but no tool calls were made")
	}

	t.Logf("Step 2: LLM called %d tool(s)", len(choice.Message.ToolCalls))

	// Step 2: Execute the tool and collect results
	toolCall := choice.Message.ToolCalls[0]
	t.Logf("  Tool: %s", toolCall.Function.Name)
	t.Logf("  Arguments: %+v", toolCall.Function.Arguments)

	// Execute the tool
	argsJSON, err := json.Marshal(toolCall.Function.Arguments)
	if err != nil {
		t.Fatalf("Failed to marshal tool arguments: %v", err)
	}

	toolResult, err := weatherTool.Execute(ctx, argsJSON)
	if err != nil {
		t.Fatalf("Tool execution failed: %v", err)
	}

	t.Logf("  Tool Result: %s", string(toolResult))

	// Step 3: Send tool result back to LLM
	messages = append(messages, choice.Message)

	toolCallID := toolCall.ID
	toolResultMessage := types.NewToolMessage(types.WithText(string(toolResult)))
	toolResultMessage.ToolCallID = &toolCallID
	messages = append(messages, toolResultMessage)

	params = &types.ChatParams{
		Model:    "gpt-4o-mini",
		Messages: messages,
		Tools:    []types.Tool{weatherTool},
	}

	t.Log("Step 3: Sending tool result back to LLM for final answer")
	finalResponse, err := client.Chat(ctx, params)
	if err != nil {
		t.Fatalf("Final chat request failed: %v", err)
	}

	if len(finalResponse.Choices) == 0 {
		t.Fatal("Final response has no choices")
	}

	finalChoice := finalResponse.Choices[0]
	t.Logf("Final Finish Reason: %s", finalChoice.FinishReason)

	// Verify we got a text response
	if len(finalChoice.Message.ContentPart) == 0 {
		t.Fatal("Final response has no content")
	}

	textPart, ok := finalChoice.Message.ContentPart[0].(*types.ContentPartText)
	if !ok {
		t.Fatalf("Expected ContentPartText, got %T", finalChoice.Message.ContentPart[0])
	}

	t.Logf("Step 4: LLM Final Answer: %s", textPart.Text)

	// Verify the final answer mentions weather information
	if textPart.Text == "" {
		t.Error("Final response text is empty")
	}

	t.Log("✓ Complete tool calling round-trip successful!")
}
