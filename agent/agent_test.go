package agent

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/KennyKeni/elysia/adapter/openai"
	"github.com/KennyKeni/elysia/client"
	"github.com/KennyKeni/elysia/types"
)

// =============================================================================
// Mock Client Infrastructure
// =============================================================================

// mockRawClient implements types.RawClient for testing
type mockRawClient struct {
	mu           sync.Mutex
	chatCalls    int
	chatResponses []chatResponse // Queue of responses to return
	chatErr      error          // Error to return (if set, overrides responses)
}

type chatResponse struct {
	response *types.ChatResponse
	err      error
}

func newMockRawClient() *mockRawClient {
	return &mockRawClient{
		chatResponses: make([]chatResponse, 0),
	}
}

// queueResponse adds a response to the queue
func (m *mockRawClient) queueResponse(resp *types.ChatResponse, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chatResponses = append(m.chatResponses, chatResponse{response: resp, err: err})
}

func (m *mockRawClient) RawChat(ctx context.Context, params *types.ChatParams) (*types.ChatResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chatCalls++

	if m.chatErr != nil {
		return nil, m.chatErr
	}

	if len(m.chatResponses) == 0 {
		return nil, fmt.Errorf("no more mock responses available (call #%d)", m.chatCalls)
	}

	resp := m.chatResponses[0]
	m.chatResponses = m.chatResponses[1:]
	return resp.response, resp.err
}

func (m *mockRawClient) RawChatStream(ctx context.Context, params *types.ChatParams) (*types.Stream, error) {
	return nil, fmt.Errorf("streaming not implemented in mock")
}

func (m *mockRawClient) RawEmbed(ctx context.Context, params *types.EmbeddingParams) (*types.EmbeddingResponse, error) {
	return nil, fmt.Errorf("embedding not implemented in mock")
}

// Helper to create a mock client wrapped with types.NewClient
func newTestClient() (*mockRawClient, types.Client) {
	raw := newMockRawClient()
	return raw, types.NewClient(raw)
}

// =============================================================================
// Response Builders
// =============================================================================

// textResponse creates a simple text response (no tool calls)
func textResponse(text string) *types.ChatResponse {
	return &types.ChatResponse{
		ID:    "test-response",
		Model: "test-model",
		Choices: []types.Choice{
			{
				Index: 0,
				Message: &types.Message{
					Role:        types.RoleAssistant,
					ContentPart: []types.ContentPart{types.NewContentPartText(text)},
				},
				FinishReason: "stop",
			},
		},
		Usage: &types.Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}
}

// toolCallResponse creates a response with tool calls
func toolCallResponse(toolCalls ...types.ToolCall) *types.ChatResponse {
	return &types.ChatResponse{
		ID:    "test-response",
		Model: "test-model",
		Choices: []types.Choice{
			{
				Index: 0,
				Message: &types.Message{
					Role:        types.RoleAssistant,
					ContentPart: []types.ContentPart{},
					ToolCalls:   toolCalls,
				},
				FinishReason: "tool_calls",
			},
		},
		Usage: &types.Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}
}

// structuredResponse creates a response with structured content (for Native mode)
func structuredResponse(jsonContent string) *types.ChatResponse {
	return &types.ChatResponse{
		ID:    "test-response",
		Model: "test-model",
		Choices: []types.Choice{
			{
				Index: 0,
				Message: &types.Message{
					Role:        types.RoleAssistant,
					ContentPart: []types.ContentPart{types.NewContentPartText(jsonContent)},
				},
				FinishReason:      "stop",
				StructuredContent: jsonContent,
			},
		},
		Usage: &types.Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}
}

// outputToolResponse creates a response with the _output tool call (for Tool mode)
func outputToolResponse(jsonContent string) *types.ChatResponse {
	// Parse the JSON to get the arguments map
	var args map[string]any
	if err := json.Unmarshal([]byte(jsonContent), &args); err != nil {
		// If parsing fails, use as raw string
		args = map[string]any{"raw": jsonContent}
	}

	return &types.ChatResponse{
		ID:    "test-response",
		Model: "test-model",
		Choices: []types.Choice{
			{
				Index: 0,
				Message: &types.Message{
					Role:        types.RoleAssistant,
					ContentPart: []types.ContentPart{},
					ToolCalls: []types.ToolCall{
						{
							ID: "output-call-1",
							Function: types.ToolFunction{
								Name:      "_output",
								Arguments: args,
							},
						},
					},
				},
				FinishReason: "tool_calls",
			},
		},
		Usage: &types.Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}
}

// outputToolWithOtherToolsResponse creates a response with _output called alongside other tools
// This should trigger OutputToolMisuseError
func outputToolWithOtherToolsResponse(jsonContent string, otherToolName string) *types.ChatResponse {
	var args map[string]any
	if err := json.Unmarshal([]byte(jsonContent), &args); err != nil {
		args = map[string]any{"raw": jsonContent}
	}

	return &types.ChatResponse{
		ID:    "test-response",
		Model: "test-model",
		Choices: []types.Choice{
			{
				Index: 0,
				Message: &types.Message{
					Role:        types.RoleAssistant,
					ContentPart: []types.ContentPart{},
					ToolCalls: []types.ToolCall{
						{
							ID: "output-call-1",
							Function: types.ToolFunction{
								Name:      "_output",
								Arguments: args,
							},
						},
						{
							ID: "other-call-1",
							Function: types.ToolFunction{
								Name:      otherToolName,
								Arguments: map[string]any{},
							},
						},
					},
				},
				FinishReason: "tool_calls",
			},
		},
		Usage: &types.Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}
}

// responseWithUsage creates a response with custom usage
func responseWithUsage(text string, prompt, completion, total int64) *types.ChatResponse {
	resp := textResponse(text)
	resp.Usage = &types.Usage{
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      total,
	}
	return resp
}

// makeToolCall creates a tool call
func makeToolCall(id, name string, args map[string]any) types.ToolCall {
	return types.ToolCall{
		ID: id,
		Function: types.ToolFunction{
			Name:      name,
			Arguments: args,
		},
	}
}

// =============================================================================
// Test Types
// =============================================================================

type testDeps struct {
	Value string
}

type testInput struct {
	Name string `json:"name"`
}

type testOutput struct {
	Result string `json:"result"`
}

type emptyOutput struct{}

// =============================================================================
// Basic Agent Tests
// =============================================================================

func TestAgent_New(t *testing.T) {
	_, client := newTestClient()

	t.Run("creates agent with defaults", func(t *testing.T) {
		agent, err := New[testDeps, testOutput](client)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if agent == nil {
			t.Fatal("expected agent to be created")
		}
	})

	t.Run("creates agent with options", func(t *testing.T) {
		agent, err := New[testDeps, testOutput](client,
			WithSystemPrompt[testDeps, testOutput]("You are a test assistant"),
			WithRetries[testDeps, testOutput](3),
			WithOutputRetries[testDeps, testOutput](5),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if agent.systemPrompt != "You are a test assistant" {
			t.Errorf("expected system prompt to be set")
		}
		if agent.retries != 3 {
			t.Errorf("expected retries to be 3, got %d", agent.retries)
		}
		if agent.outputRetries != 5 {
			t.Errorf("expected outputRetries to be 5, got %d", agent.outputRetries)
		}
	})
}

func TestAgent_WithSystemPromptFunc(t *testing.T) {
	raw, client := newTestClient()
	raw.queueResponse(textResponse("Hello!"), nil)

	agent, err := New[testDeps, emptyOutput](client,
		WithSystemPromptFunc[testDeps, emptyOutput](func(deps testDeps) string {
			return "Hello, " + deps.Value
		}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deps := testDeps{Value: "World"}
	_, err = agent.Run(context.Background(), deps, WithPrompt("test"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAgent_DuplicateToolsError(t *testing.T) {
	_, client := newTestClient()

	tool1, _ := NewTool[testDeps, testInput, testOutput](
		"duplicate_tool", "First tool",
		func(ctx context.Context, rc *RunContext[testDeps], in testInput) (testOutput, error) {
			return testOutput{Result: "ok"}, nil
		},
	)

	tool2, _ := NewTool[testDeps, testInput, testOutput](
		"duplicate_tool", "Second tool with same name",
		func(ctx context.Context, rc *RunContext[testDeps], in testInput) (testOutput, error) {
			return testOutput{Result: "ok"}, nil
		},
	)

	_, err := New[testDeps, testOutput](client,
		WithTools[testDeps, testOutput](tool1, tool2),
	)
	if err == nil {
		t.Fatal("expected error for duplicate tool names")
	}
	if err.Error() != "duplicate tool name: duplicate_tool" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// =============================================================================
// Basic Run Tests
// =============================================================================

func TestAgent_Run_SimpleTextResponse(t *testing.T) {
	raw, client := newTestClient()
	raw.queueResponse(textResponse("Hello, world!"), nil)

	agent, err := New[testDeps, emptyOutput](client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := agent.Run(context.Background(), testDeps{}, WithPrompt("Say hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Messages) != 2 {
		t.Errorf("expected 2 messages (user + assistant), got %d", len(result.Messages))
	}
	if raw.chatCalls != 1 {
		t.Errorf("expected 1 chat call, got %d", raw.chatCalls)
	}
}

func TestAgent_Run_NoResponse(t *testing.T) {
	raw, client := newTestClient()
	raw.queueResponse(&types.ChatResponse{
		Choices: []types.Choice{},
	}, nil)

	agent, err := New[testDeps, emptyOutput](client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = agent.Run(context.Background(), testDeps{}, WithPrompt("test"))
	if err == nil {
		t.Fatal("expected error for no response")
	}
	if err.Error() != "no response from model" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAgent_Run_WithMessages(t *testing.T) {
	raw, client := newTestClient()
	raw.queueResponse(textResponse("continuation"), nil)

	agent, err := New[testDeps, emptyOutput](client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	initialMessages := []types.Message{
		types.NewUserMessage(types.WithText("First message")),
		types.NewAssistantMessage(types.WithText("First response")),
	}

	result, err := agent.Run(context.Background(), testDeps{},
		WithMessages(initialMessages),
		WithPrompt("Continue"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 2 initial + 1 prompt + 1 response = 4
	if len(result.Messages) != 4 {
		t.Errorf("expected 4 messages, got %d", len(result.Messages))
	}
}

// =============================================================================
// Tool Calling Tests
// =============================================================================

func TestAgent_Run_SingleToolCall(t *testing.T) {
	raw, client := newTestClient()

	// First response: tool call
	raw.queueResponse(toolCallResponse(
		makeToolCall("call-1", "greet", map[string]any{"name": "Alice"}),
	), nil)

	// Second response: final text
	raw.queueResponse(textResponse("Greeting sent!"), nil)

	greetTool, _ := NewTool[testDeps, testInput, testOutput](
		"greet", "Greets a person",
		func(ctx context.Context, rc *RunContext[testDeps], in testInput) (testOutput, error) {
			return testOutput{Result: "Hello, " + in.Name}, nil
		},
	)

	agent, err := New[testDeps, emptyOutput](client,
		WithTools[testDeps, emptyOutput](greetTool),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := agent.Run(context.Background(), testDeps{}, WithPrompt("Greet Alice"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if raw.chatCalls != 2 {
		t.Errorf("expected 2 chat calls, got %d", raw.chatCalls)
	}

	// user prompt + assistant (tool call) + tool result + assistant (final)
	if len(result.Messages) != 4 {
		t.Errorf("expected 4 messages, got %d", len(result.Messages))
	}
}

func TestAgent_Run_MultipleToolCalls(t *testing.T) {
	raw, client := newTestClient()

	// First response: multiple tool calls
	raw.queueResponse(toolCallResponse(
		makeToolCall("call-1", "add", map[string]any{"a": float64(1), "b": float64(2)}),
		makeToolCall("call-2", "add", map[string]any{"a": float64(3), "b": float64(4)}),
	), nil)

	// Second response: final text
	raw.queueResponse(textResponse("Done"), nil)

	type addInput struct {
		A float64 `json:"a"`
		B float64 `json:"b"`
	}
	type addOutput struct {
		Sum float64 `json:"sum"`
	}

	addTool, _ := NewTool[testDeps, addInput, addOutput](
		"add", "Adds two numbers",
		func(ctx context.Context, rc *RunContext[testDeps], in addInput) (addOutput, error) {
			return addOutput{Sum: in.A + in.B}, nil
		},
	)

	agent, err := New[testDeps, emptyOutput](client,
		WithTools[testDeps, emptyOutput](addTool),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := agent.Run(context.Background(), testDeps{}, WithPrompt("Add numbers"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// user + assistant (2 tool calls) + 2 tool results + assistant (final)
	if len(result.Messages) != 5 {
		t.Errorf("expected 5 messages, got %d", len(result.Messages))
	}
}

func TestAgent_Run_UnknownTool(t *testing.T) {
	raw, client := newTestClient()

	// Response calls an unknown tool
	raw.queueResponse(toolCallResponse(
		makeToolCall("call-1", "unknown_tool", map[string]any{}),
	), nil)

	agent, err := New[testDeps, emptyOutput](client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = agent.Run(context.Background(), testDeps{}, WithPrompt("test"))
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
	if err.Error() != "unknown tool: unknown_tool" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAgent_Run_ToolWithRunContext(t *testing.T) {
	raw, client := newTestClient()

	raw.queueResponse(toolCallResponse(
		makeToolCall("call-1", "context_tool", map[string]any{"name": "test"}),
	), nil)
	raw.queueResponse(textResponse("Done"), nil)

	var capturedRC *RunContext[testDeps]

	contextTool, _ := NewTool[testDeps, testInput, testOutput](
		"context_tool", "Captures context",
		func(ctx context.Context, rc *RunContext[testDeps], in testInput) (testOutput, error) {
			capturedRC = rc
			return testOutput{Result: "captured"}, nil
		},
	)

	agent, err := New[testDeps, emptyOutput](client,
		WithTools[testDeps, emptyOutput](contextTool),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deps := testDeps{Value: "test-deps"}
	_, err = agent.Run(context.Background(), deps, WithPrompt("Test prompt"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedRC == nil {
		t.Fatal("RunContext not captured")
	}
	if capturedRC.Deps.Value != "test-deps" {
		t.Errorf("expected deps value 'test-deps', got %q", capturedRC.Deps.Value)
	}
	if capturedRC.Prompt != "Test prompt" {
		t.Errorf("expected prompt 'Test prompt', got %q", capturedRC.Prompt)
	}
	if capturedRC.ToolCallID != "call-1" {
		t.Errorf("expected tool call ID 'call-1', got %q", capturedRC.ToolCallID)
	}
	if capturedRC.RunID == "" {
		t.Error("expected RunID to be set")
	}
}

// =============================================================================
// Tool Retry Tests
// =============================================================================

func TestAgent_Run_ModelRetry(t *testing.T) {
	raw, client := newTestClient()

	// First call: tool call
	raw.queueResponse(toolCallResponse(
		makeToolCall("call-1", "flaky_tool", map[string]any{"name": "test"}),
	), nil)

	// Second call (after retry): tool call again
	raw.queueResponse(toolCallResponse(
		makeToolCall("call-2", "flaky_tool", map[string]any{"name": "test"}),
	), nil)

	// Third call: success
	raw.queueResponse(textResponse("Done"), nil)

	callCount := 0
	flakyTool, _ := NewTool[testDeps, testInput, testOutput](
		"flaky_tool", "Fails first time",
		func(ctx context.Context, rc *RunContext[testDeps], in testInput) (testOutput, error) {
			callCount++
			if callCount == 1 {
				return testOutput{}, NewModelRetry("First attempt failed, try again")
			}
			return testOutput{Result: "success"}, nil
		},
	)

	agent, err := New[testDeps, emptyOutput](client,
		WithTools[testDeps, emptyOutput](flakyTool),
		WithRetries[testDeps, emptyOutput](3),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = agent.Run(context.Background(), testDeps{}, WithPrompt("test"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callCount != 2 {
		t.Errorf("expected tool to be called 2 times, got %d", callCount)
	}
}

func TestAgent_Run_ModelRetry_ExceedsLimit(t *testing.T) {
	raw, client := newTestClient()

	// Queue enough responses for retries
	for i := 0; i < 5; i++ {
		raw.queueResponse(toolCallResponse(
			makeToolCall(fmt.Sprintf("call-%d", i), "always_fails", map[string]any{"name": "test"}),
		), nil)
	}

	alwaysFailsTool, _ := NewTool[testDeps, testInput, testOutput](
		"always_fails", "Always fails",
		func(ctx context.Context, rc *RunContext[testDeps], in testInput) (testOutput, error) {
			return testOutput{}, NewModelRetry("Always fails")
		},
	)

	agent, err := New[testDeps, emptyOutput](client,
		WithTools[testDeps, emptyOutput](alwaysFailsTool),
		WithRetries[testDeps, emptyOutput](2),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = agent.Run(context.Background(), testDeps{}, WithPrompt("test"))
	if err == nil {
		t.Fatal("expected error for exceeded retries")
	}
	if !errors.Is(err, &ModelRetry{}) && err.Error() != `tool "always_fails" exceeded max retries (2): Always fails` {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAgent_Run_ToolRetryCount(t *testing.T) {
	raw, client := newTestClient()

	// Queue responses for retry attempts
	raw.queueResponse(toolCallResponse(
		makeToolCall("call-1", "counting_tool", map[string]any{"name": "test"}),
	), nil)
	raw.queueResponse(toolCallResponse(
		makeToolCall("call-2", "counting_tool", map[string]any{"name": "test"}),
	), nil)
	raw.queueResponse(toolCallResponse(
		makeToolCall("call-3", "counting_tool", map[string]any{"name": "test"}),
	), nil)
	raw.queueResponse(textResponse("Done"), nil)

	var retryValues []int
	countingTool, _ := NewTool[testDeps, testInput, testOutput](
		"counting_tool", "Counts retries",
		func(ctx context.Context, rc *RunContext[testDeps], in testInput) (testOutput, error) {
			retryValues = append(retryValues, rc.Retry)
			if rc.Retry < 2 {
				return testOutput{}, NewModelRetry("retry please")
			}
			return testOutput{Result: "success"}, nil
		},
	)

	agent, err := New[testDeps, emptyOutput](client,
		WithTools[testDeps, emptyOutput](countingTool),
		WithRetries[testDeps, emptyOutput](5),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = agent.Run(context.Background(), testDeps{}, WithPrompt("test"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []int{0, 1, 2}
	if len(retryValues) != len(expected) {
		t.Fatalf("expected %d retry values, got %d", len(expected), len(retryValues))
	}
	for i, v := range expected {
		if retryValues[i] != v {
			t.Errorf("retry %d: expected %d, got %d", i, v, retryValues[i])
		}
	}
}

func TestAgent_Run_LastAttempt(t *testing.T) {
	raw, client := newTestClient()

	raw.queueResponse(toolCallResponse(
		makeToolCall("call-1", "last_attempt_tool", map[string]any{"name": "test"}),
	), nil)
	raw.queueResponse(toolCallResponse(
		makeToolCall("call-2", "last_attempt_tool", map[string]any{"name": "test"}),
	), nil)
	raw.queueResponse(textResponse("Done"), nil)

	var lastAttemptValues []bool
	lastAttemptTool, _ := NewTool[testDeps, testInput, testOutput](
		"last_attempt_tool", "Checks last attempt",
		func(ctx context.Context, rc *RunContext[testDeps], in testInput) (testOutput, error) {
			lastAttemptValues = append(lastAttemptValues, rc.LastAttempt())
			if !rc.LastAttempt() {
				return testOutput{}, NewModelRetry("not last attempt")
			}
			return testOutput{Result: "success"}, nil
		},
	)

	agent, err := New[testDeps, emptyOutput](client,
		WithTools[testDeps, emptyOutput](lastAttemptTool),
		WithRetries[testDeps, emptyOutput](1), // 0 is first, 1 is last
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = agent.Run(context.Background(), testDeps{}, WithPrompt("test"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []bool{false, true}
	if len(lastAttemptValues) != len(expected) {
		t.Fatalf("expected %d values, got %d", len(expected), len(lastAttemptValues))
	}
	for i, v := range expected {
		if lastAttemptValues[i] != v {
			t.Errorf("attempt %d: expected LastAttempt=%v, got %v", i, v, lastAttemptValues[i])
		}
	}
}

func TestAgent_Run_PerToolRetries(t *testing.T) {
	raw, client := newTestClient()

	// Queue responses for tool with custom retry count
	for i := 0; i < 6; i++ {
		raw.queueResponse(toolCallResponse(
			makeToolCall(fmt.Sprintf("call-%d", i), "custom_retries_tool", map[string]any{"name": "test"}),
		), nil)
	}

	callCount := 0
	customRetriesTool, _ := NewTool[testDeps, testInput, testOutput](
		"custom_retries_tool", "Has custom retries",
		func(ctx context.Context, rc *RunContext[testDeps], in testInput) (testOutput, error) {
			callCount++
			if rc.MaxRetries != 5 {
				t.Errorf("expected MaxRetries=5, got %d", rc.MaxRetries)
			}
			return testOutput{}, NewModelRetry("keep retrying")
		},
		ToolRetries[testDeps](5), // Custom retry count
	)

	agent, err := New[testDeps, emptyOutput](client,
		WithTools[testDeps, emptyOutput](customRetriesTool),
		WithRetries[testDeps, emptyOutput](1), // Agent default is 1
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = agent.Run(context.Background(), testDeps{}, WithPrompt("test"))
	if err == nil {
		t.Fatal("expected error for exceeded retries")
	}

	// Should have been called 6 times (initial + 5 retries)
	if callCount != 6 {
		t.Errorf("expected 6 calls, got %d", callCount)
	}
}

func TestAgent_Run_RunRetries_Override(t *testing.T) {
	raw, client := newTestClient()

	for i := 0; i < 4; i++ {
		raw.queueResponse(toolCallResponse(
			makeToolCall(fmt.Sprintf("call-%d", i), "retrying_tool", map[string]any{"name": "test"}),
		), nil)
	}

	callCount := 0
	retryingTool, _ := NewTool[testDeps, testInput, testOutput](
		"retrying_tool", "Retries",
		func(ctx context.Context, rc *RunContext[testDeps], in testInput) (testOutput, error) {
			callCount++
			if rc.MaxRetries != 3 {
				t.Errorf("expected MaxRetries=3 (run override), got %d", rc.MaxRetries)
			}
			return testOutput{}, NewModelRetry("retry")
		},
	)

	agent, err := New[testDeps, emptyOutput](client,
		WithTools[testDeps, emptyOutput](retryingTool),
		WithRetries[testDeps, emptyOutput](1), // Agent default
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = agent.Run(context.Background(), testDeps{},
		WithPrompt("test"),
		WithRunRetries(3), // Override for this run
	)
	if err == nil {
		t.Fatal("expected error for exceeded retries")
	}

	// 1 initial + 3 retries = 4
	if callCount != 4 {
		t.Errorf("expected 4 calls, got %d", callCount)
	}
}

func TestAgent_Run_NonModelRetryError(t *testing.T) {
	raw, client := newTestClient()

	raw.queueResponse(toolCallResponse(
		makeToolCall("call-1", "error_tool", map[string]any{"name": "test"}),
	), nil)
	raw.queueResponse(textResponse("Done"), nil)

	errorTool, _ := NewTool[testDeps, testInput, testOutput](
		"error_tool", "Returns non-retry error",
		func(ctx context.Context, rc *RunContext[testDeps], in testInput) (testOutput, error) {
			return testOutput{}, errors.New("regular error")
		},
	)

	agent, err := New[testDeps, emptyOutput](client,
		WithTools[testDeps, emptyOutput](errorTool),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Non-ModelRetry errors become ToolResult with IsError=true
	// The run should continue (tool error is sent to LLM)
	_, err = agent.Run(context.Background(), testDeps{}, WithPrompt("test"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAgent_Run_InputValidationRetry(t *testing.T) {
	raw, client := newTestClient()

	// First call with invalid input
	raw.queueResponse(toolCallResponse(
		makeToolCall("call-1", "validated_tool", map[string]any{"invalid_field": "test"}),
	), nil)

	// Second call with valid input
	raw.queueResponse(toolCallResponse(
		makeToolCall("call-2", "validated_tool", map[string]any{"name": "valid"}),
	), nil)

	raw.queueResponse(textResponse("Done"), nil)

	callCount := 0
	validatedTool, _ := NewTool[testDeps, testInput, testOutput](
		"validated_tool", "Requires valid input",
		func(ctx context.Context, rc *RunContext[testDeps], in testInput) (testOutput, error) {
			callCount++
			return testOutput{Result: in.Name}, nil
		},
	)

	agent, err := New[testDeps, emptyOutput](client,
		WithTools[testDeps, emptyOutput](validatedTool),
		WithRetries[testDeps, emptyOutput](3),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = agent.Run(context.Background(), testDeps{}, WithPrompt("test"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Handler should only be called once (second call with valid input)
	if callCount != 1 {
		t.Errorf("expected handler to be called 1 time, got %d", callCount)
	}
}

// =============================================================================
// Output Validation Retry Tests
// =============================================================================

func TestAgent_Run_OutputValidation_SchemaError(t *testing.T) {
	raw, client := newTestClient()

	// First response: returns SchemaValidationError
	raw.queueResponse(nil, &types.SchemaValidationError{
		RawResponse: "invalid",
		Err:         errors.New("schema mismatch"),
	})

	// Second response: valid
	raw.queueResponse(structuredResponse(`{"result":"success"}`), nil)

	agent, err := New[testDeps, testOutput](client,
		WithResponseFormat[testDeps, testOutput](types.ResponseFormatModeNative),
		WithOutputRetries[testDeps, testOutput](2),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := agent.Run(context.Background(), testDeps{}, WithPrompt("test"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Output.Result != "success" {
		t.Errorf("expected output 'success', got %q", result.Output.Result)
	}
	if raw.chatCalls != 2 {
		t.Errorf("expected 2 chat calls, got %d", raw.chatCalls)
	}
}

func TestAgent_Run_OutputValidation_ToolNotCalled(t *testing.T) {
	raw, client := newTestClient()

	// First response: no tool calls (will cause ToolNotCalledError in Tool mode)
	// The baseClient wrapper will detect this and return ToolNotCalledError
	raw.queueResponse(textResponse("I don't know how to call tools"), nil)

	// Second response: _output tool called correctly
	raw.queueResponse(outputToolResponse(`{"result":"success"}`), nil)

	agent, err := New[testDeps, testOutput](client,
		WithResponseFormat[testDeps, testOutput](types.ResponseFormatModeTool),
		WithOutputRetries[testDeps, testOutput](2),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := agent.Run(context.Background(), testDeps{}, WithPrompt("test"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Output.Result != "success" {
		t.Errorf("expected output 'success', got %q", result.Output.Result)
	}
}

func TestAgent_Run_OutputValidation_MisuseError(t *testing.T) {
	raw, client := newTestClient()

	// First response: _output called alongside other tools (will cause OutputToolMisuseError)
	raw.queueResponse(outputToolWithOtherToolsResponse(`{"result":"partial"}`, "other_tool"), nil)

	// Second response: _output tool called correctly (alone)
	raw.queueResponse(outputToolResponse(`{"result":"success"}`), nil)

	agent, err := New[testDeps, testOutput](client,
		WithResponseFormat[testDeps, testOutput](types.ResponseFormatModeTool),
		WithOutputRetries[testDeps, testOutput](2),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := agent.Run(context.Background(), testDeps{}, WithPrompt("test"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Output.Result != "success" {
		t.Errorf("expected output 'success', got %q", result.Output.Result)
	}
}

func TestAgent_Run_OutputValidation_ExceedsLimit(t *testing.T) {
	raw, client := newTestClient()

	// Queue multiple error responses
	for i := 0; i < 5; i++ {
		raw.queueResponse(nil, &types.SchemaValidationError{
			RawResponse: "invalid",
			Err:         errors.New("schema mismatch"),
		})
	}

	agent, err := New[testDeps, testOutput](client,
		WithResponseFormat[testDeps, testOutput](types.ResponseFormatModeNative),
		WithOutputRetries[testDeps, testOutput](2),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = agent.Run(context.Background(), testDeps{}, WithPrompt("test"))
	if err == nil {
		t.Fatal("expected error for exceeded output retries")
	}
}

func TestAgent_Run_OutputValidation_UnmarshalError(t *testing.T) {
	raw, client := newTestClient()

	// First response: invalid JSON for unmarshaling
	raw.queueResponse(structuredResponse(`{not valid json`), nil)

	// Second response: valid JSON
	raw.queueResponse(structuredResponse(`{"result":"success"}`), nil)

	agent, err := New[testDeps, testOutput](client,
		WithResponseFormat[testDeps, testOutput](types.ResponseFormatModeNative),
		WithOutputRetries[testDeps, testOutput](2),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := agent.Run(context.Background(), testDeps{}, WithPrompt("test"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Output.Result != "success" {
		t.Errorf("expected output 'success', got %q", result.Output.Result)
	}
}

func TestAgent_Run_OutputValidation_MissingStructuredContent(t *testing.T) {
	raw, client := newTestClient()

	// First response: no structured content but schema expected
	resp := textResponse("Just text, no JSON")
	resp.Choices[0].StructuredContent = ""
	raw.queueResponse(resp, nil)

	// Second response: with structured content
	raw.queueResponse(structuredResponse(`{"result":"success"}`), nil)

	agent, err := New[testDeps, testOutput](client,
		WithResponseFormat[testDeps, testOutput](types.ResponseFormatModeNative),
		WithOutputRetries[testDeps, testOutput](2),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := agent.Run(context.Background(), testDeps{}, WithPrompt("test"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Output.Result != "success" {
		t.Errorf("expected output 'success', got %q", result.Output.Result)
	}
}

func TestAgent_Run_OutputRetries_FallsBackToRetries(t *testing.T) {
	raw, client := newTestClient()

	// Queue multiple error responses (more than retries but less than we need to test)
	for i := 0; i < 5; i++ {
		raw.queueResponse(nil, &types.SchemaValidationError{
			RawResponse: "invalid",
			Err:         errors.New("schema mismatch"),
		})
	}

	agent, err := New[testDeps, testOutput](client,
		WithResponseFormat[testDeps, testOutput](types.ResponseFormatModeNative),
		WithRetries[testDeps, testOutput](3),
		// outputRetries not set, should fall back to retries=3
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = agent.Run(context.Background(), testDeps{}, WithPrompt("test"))
	if err == nil {
		t.Fatal("expected error for exceeded output retries")
	}

	// Should have made 4 calls (1 initial + 3 retries)
	if raw.chatCalls != 4 {
		t.Errorf("expected 4 chat calls (fallback to retries=3), got %d", raw.chatCalls)
	}
}

// =============================================================================
// Usage Limits Tests
// =============================================================================

func TestAgent_Run_UsageLimits_RequestLimit(t *testing.T) {
	raw, client := newTestClient()

	// Queue tool calls that will exceed request limit
	for i := 0; i < 5; i++ {
		raw.queueResponse(toolCallResponse(
			makeToolCall(fmt.Sprintf("call-%d", i), "echo_tool", map[string]any{"name": "test"}),
		), nil)
	}

	echoTool, _ := NewTool[testDeps, testInput, testOutput](
		"echo_tool", "Echoes input",
		func(ctx context.Context, rc *RunContext[testDeps], in testInput) (testOutput, error) {
			return testOutput{Result: in.Name}, nil
		},
	)

	agent, err := New[testDeps, emptyOutput](client,
		WithTools[testDeps, emptyOutput](echoTool),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = agent.Run(context.Background(), testDeps{},
		WithPrompt("test"),
		WithUsageLimits(UsageLimits{
			RequestLimit: 3,
		}),
	)
	if err == nil {
		t.Fatal("expected error for exceeded request limit")
	}

	var limitErr *UsageLimitExceeded
	if !errors.As(err, &limitErr) {
		t.Fatalf("expected UsageLimitExceeded error, got %T: %v", err, err)
	}
	if limitErr.Limit != "request_limit" {
		t.Errorf("expected limit 'request_limit', got %q", limitErr.Limit)
	}
	if limitErr.Max != 3 {
		t.Errorf("expected max 3, got %d", limitErr.Max)
	}
}

func TestAgent_Run_UsageLimits_CompletionTokensLimit(t *testing.T) {
	raw, client := newTestClient()

	// Response with high completion tokens
	raw.queueResponse(responseWithUsage("response", 10, 500, 510), nil)

	agent, err := New[testDeps, emptyOutput](client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = agent.Run(context.Background(), testDeps{},
		WithPrompt("test"),
		WithUsageLimits(UsageLimits{
			CompletionTokensLimit: 100,
		}),
	)
	if err == nil {
		t.Fatal("expected error for exceeded completion tokens limit")
	}

	var limitErr *UsageLimitExceeded
	if !errors.As(err, &limitErr) {
		t.Fatalf("expected UsageLimitExceeded error, got %T: %v", err, err)
	}
	if limitErr.Limit != "completion_tokens_limit" {
		t.Errorf("expected limit 'completion_tokens_limit', got %q", limitErr.Limit)
	}
}

func TestAgent_Run_UsageLimits_ToolCallsLimit(t *testing.T) {
	raw, client := newTestClient()

	// Queue tool calls
	for i := 0; i < 5; i++ {
		raw.queueResponse(toolCallResponse(
			makeToolCall(fmt.Sprintf("call-%d", i), "counted_tool", map[string]any{"name": "test"}),
		), nil)
	}

	countedTool, _ := NewTool[testDeps, testInput, testOutput](
		"counted_tool", "Counted tool",
		func(ctx context.Context, rc *RunContext[testDeps], in testInput) (testOutput, error) {
			return testOutput{Result: in.Name}, nil
		},
	)

	agent, err := New[testDeps, emptyOutput](client,
		WithTools[testDeps, emptyOutput](countedTool),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = agent.Run(context.Background(), testDeps{},
		WithPrompt("test"),
		WithUsageLimits(UsageLimits{
			ToolCallsLimit: 2,
		}),
	)
	if err == nil {
		t.Fatal("expected error for exceeded tool calls limit")
	}

	var limitErr *UsageLimitExceeded
	if !errors.As(err, &limitErr) {
		t.Fatalf("expected UsageLimitExceeded error, got %T: %v", err, err)
	}
	if limitErr.Limit != "tool_calls_limit" {
		t.Errorf("expected limit 'tool_calls_limit', got %q", limitErr.Limit)
	}
}

func TestAgent_Run_UsageLimits_FailedToolsNotCounted(t *testing.T) {
	raw, client := newTestClient()

	// First call: tool returns ModelRetry (doesn't count)
	raw.queueResponse(toolCallResponse(
		makeToolCall("call-1", "flaky_tool", map[string]any{"name": "test"}),
	), nil)

	// Second call: tool succeeds (counts as 1)
	raw.queueResponse(toolCallResponse(
		makeToolCall("call-2", "flaky_tool", map[string]any{"name": "test"}),
	), nil)

	// Third call: tool succeeds (counts as 2)
	raw.queueResponse(toolCallResponse(
		makeToolCall("call-3", "flaky_tool", map[string]any{"name": "test"}),
	), nil)

	raw.queueResponse(textResponse("Done"), nil)

	callCount := 0
	flakyTool, _ := NewTool[testDeps, testInput, testOutput](
		"flaky_tool", "Flaky tool",
		func(ctx context.Context, rc *RunContext[testDeps], in testInput) (testOutput, error) {
			callCount++
			if callCount == 1 {
				return testOutput{}, NewModelRetry("retry")
			}
			return testOutput{Result: "ok"}, nil
		},
	)

	agent, err := New[testDeps, emptyOutput](client,
		WithTools[testDeps, emptyOutput](flakyTool),
		WithRetries[testDeps, emptyOutput](5),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = agent.Run(context.Background(), testDeps{},
		WithPrompt("test"),
		WithUsageLimits(UsageLimits{
			ToolCallsLimit: 2, // Limit of 2 successful calls
		}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should complete without hitting limit (2 successful calls)
	if callCount != 3 {
		t.Errorf("expected 3 calls (1 retry + 2 success), got %d", callCount)
	}
}

// =============================================================================
// Max Iterations Tests
// =============================================================================

func TestAgent_Run_MaxIterations(t *testing.T) {
	raw, client := newTestClient()

	// Queue more tool calls than max iterations
	for i := 0; i < 15; i++ {
		raw.queueResponse(toolCallResponse(
			makeToolCall(fmt.Sprintf("call-%d", i), "infinite_tool", map[string]any{"name": "test"}),
		), nil)
	}

	infiniteTool, _ := NewTool[testDeps, testInput, testOutput](
		"infinite_tool", "Never stops",
		func(ctx context.Context, rc *RunContext[testDeps], in testInput) (testOutput, error) {
			return testOutput{Result: "again"}, nil
		},
	)

	agent, err := New[testDeps, emptyOutput](client,
		WithTools[testDeps, emptyOutput](infiniteTool),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = agent.Run(context.Background(), testDeps{}, WithPrompt("test"))
	if err == nil {
		t.Fatal("expected error for max iterations exceeded")
	}
	if err.Error() != "agent exceeded max iterations (10)" {
		t.Errorf("unexpected error: %v", err)
	}
}

// =============================================================================
// Usage Tracking Tests
// =============================================================================

func TestAgent_Run_UsageTracking(t *testing.T) {
	raw, client := newTestClient()

	// Multiple responses with usage
	raw.queueResponse(responseWithUsage("", 100, 50, 150), nil)

	agent, err := New[testDeps, emptyOutput](client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := agent.Run(context.Background(), testDeps{}, WithPrompt("test"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Usage.PromptTokens != 100 {
		t.Errorf("expected 100 prompt tokens, got %d", result.Usage.PromptTokens)
	}
	if result.Usage.CompletionTokens != 50 {
		t.Errorf("expected 50 completion tokens, got %d", result.Usage.CompletionTokens)
	}
	if result.Usage.TotalTokens != 150 {
		t.Errorf("expected 150 total tokens, got %d", result.Usage.TotalTokens)
	}
}

func TestAgent_Run_UsageTracking_Accumulated(t *testing.T) {
	raw, client := newTestClient()

	// First response: tool call with usage
	resp1 := toolCallResponse(makeToolCall("call-1", "echo", map[string]any{"name": "test"}))
	resp1.Usage = &types.Usage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150}
	raw.queueResponse(resp1, nil)

	// Second response: final with more usage
	resp2 := textResponse("Done")
	resp2.Usage = &types.Usage{PromptTokens: 150, CompletionTokens: 30, TotalTokens: 180}
	raw.queueResponse(resp2, nil)

	echoTool, _ := NewTool[testDeps, testInput, testOutput](
		"echo", "Echoes",
		func(ctx context.Context, rc *RunContext[testDeps], in testInput) (testOutput, error) {
			return testOutput{Result: in.Name}, nil
		},
	)

	agent, err := New[testDeps, emptyOutput](client,
		WithTools[testDeps, emptyOutput](echoTool),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := agent.Run(context.Background(), testDeps{}, WithPrompt("test"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should accumulate across both calls
	if result.Usage.PromptTokens != 250 {
		t.Errorf("expected 250 prompt tokens, got %d", result.Usage.PromptTokens)
	}
	if result.Usage.CompletionTokens != 80 {
		t.Errorf("expected 80 completion tokens, got %d", result.Usage.CompletionTokens)
	}
	if result.Usage.TotalTokens != 330 {
		t.Errorf("expected 330 total tokens, got %d", result.Usage.TotalTokens)
	}
}

// =============================================================================
// Structured Output Tests
// =============================================================================

func TestAgent_Run_StructuredOutput(t *testing.T) {
	raw, client := newTestClient()

	raw.queueResponse(structuredResponse(`{"result":"structured output"}`), nil)

	agent, err := New[testDeps, testOutput](client,
		WithResponseFormat[testDeps, testOutput](types.ResponseFormatModeNative),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := agent.Run(context.Background(), testDeps{}, WithPrompt("test"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Output.Result != "structured output" {
		t.Errorf("expected output 'structured output', got %q", result.Output.Result)
	}
}

// =============================================================================
// Client Error Handling Tests
// =============================================================================

func TestAgent_Run_ClientError(t *testing.T) {
	raw, client := newTestClient()

	clientErr := errors.New("API error")
	raw.queueResponse(nil, clientErr)

	agent, err := New[testDeps, emptyOutput](client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = agent.Run(context.Background(), testDeps{}, WithPrompt("test"))
	if err == nil {
		t.Fatal("expected error from client")
	}
	if !errors.Is(err, clientErr) {
		t.Errorf("expected client error, got: %v", err)
	}
}

// =============================================================================
// Tool Helper Tests
// =============================================================================

func TestGetToolDefinitions(t *testing.T) {
	tool1, _ := NewTool[testDeps, testInput, testOutput](
		"tool1", "First tool",
		func(ctx context.Context, rc *RunContext[testDeps], in testInput) (testOutput, error) {
			return testOutput{}, nil
		},
	)
	tool2, _ := NewTool[testDeps, testInput, testOutput](
		"tool2", "Second tool",
		func(ctx context.Context, rc *RunContext[testDeps], in testInput) (testOutput, error) {
			return testOutput{}, nil
		},
	)

	tools := []*Tool[testDeps]{tool1, tool2}
	defs := GetToolDefinitions(tools)

	if len(defs) != 2 {
		t.Fatalf("expected 2 definitions, got %d", len(defs))
	}
	if defs[0].Name != "tool1" {
		t.Errorf("expected first tool name 'tool1', got %q", defs[0].Name)
	}
	if defs[1].Name != "tool2" {
		t.Errorf("expected second tool name 'tool2', got %q", defs[1].Name)
	}
}

func TestModelRetry(t *testing.T) {
	t.Run("NewModelRetry", func(t *testing.T) {
		mr := NewModelRetry("test message")
		if mr.Message != "test message" {
			t.Errorf("expected message 'test message', got %q", mr.Message)
		}
		if mr.Error() != "test message" {
			t.Errorf("expected Error() to return 'test message', got %q", mr.Error())
		}
	})

	t.Run("IsModelRetry", func(t *testing.T) {
		mr := NewModelRetry("test")
		got, ok := IsModelRetry(mr)
		if !ok {
			t.Error("expected IsModelRetry to return true")
		}
		if got != mr {
			t.Error("expected IsModelRetry to return the same error")
		}

		regularErr := errors.New("regular error")
		_, ok = IsModelRetry(regularErr)
		if ok {
			t.Error("expected IsModelRetry to return false for regular error")
		}
	})

	t.Run("IsModelRetry_wrapped", func(t *testing.T) {
		mr := NewModelRetry("test")
		wrapped := fmt.Errorf("wrapped: %w", mr)

		got, ok := IsModelRetry(wrapped)
		if !ok {
			t.Error("expected IsModelRetry to find wrapped ModelRetry")
		}
		if got.Message != "test" {
			t.Errorf("expected message 'test', got %q", got.Message)
		}
	})
}

func TestRunContext_LastAttempt(t *testing.T) {
	tests := []struct {
		retry      int
		maxRetries int
		expected   bool
	}{
		{0, 0, true},
		{0, 1, false},
		{1, 1, true},
		{0, 3, false},
		{2, 3, false},
		{3, 3, true},
		{4, 3, true}, // Over max
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("retry=%d,max=%d", tt.retry, tt.maxRetries), func(t *testing.T) {
			rc := &RunContext[testDeps]{
				Retry:      tt.retry,
				MaxRetries: tt.maxRetries,
			}
			if rc.LastAttempt() != tt.expected {
				t.Errorf("LastAttempt() = %v, want %v", rc.LastAttempt(), tt.expected)
			}
		})
	}
}

// =============================================================================
// WrapTool Tests
// =============================================================================

func TestWrapTool(t *testing.T) {
	raw, client := newTestClient()

	raw.queueResponse(toolCallResponse(
		makeToolCall("call-1", "wrapped_tool", map[string]any{"name": "test"}),
	), nil)
	raw.queueResponse(textResponse("Done"), nil)

	// Create a types.Tool
	typesTool, err := types.NewTool[testInput, testOutput](
		"wrapped_tool", "A wrapped tool",
		func(ctx context.Context, in testInput) (testOutput, error) {
			return testOutput{Result: "wrapped: " + in.Name}, nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error creating types.Tool: %v", err)
	}

	// Wrap it for use with agent
	wrappedTool := WrapTool[testDeps](typesTool)

	agent, err := New[testDeps, emptyOutput](client,
		WithTools[testDeps, emptyOutput](wrappedTool),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = agent.Run(context.Background(), testDeps{}, WithPrompt("test"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWrapTool_WithRetries(t *testing.T) {
	typesTool, _ := types.NewTool[testInput, testOutput](
		"wrapped_tool", "A wrapped tool",
		func(ctx context.Context, in testInput) (testOutput, error) {
			return testOutput{}, nil
		},
	)

	wrappedTool := WrapTool[testDeps](typesTool, ToolRetries[testDeps](5))

	if wrappedTool.Retries != 5 {
		t.Errorf("expected Retries=5, got %d", wrappedTool.Retries)
	}
}

// =============================================================================
// Tool Reset After Success Tests
// =============================================================================

func TestAgent_Run_ToolRetryResetAfterSuccess(t *testing.T) {
	raw, client := newTestClient()

	// First call: tool fails
	raw.queueResponse(toolCallResponse(
		makeToolCall("call-1", "resetable_tool", map[string]any{"name": "test"}),
	), nil)

	// Second call: tool succeeds
	raw.queueResponse(toolCallResponse(
		makeToolCall("call-2", "resetable_tool", map[string]any{"name": "test"}),
	), nil)

	// Third call: same tool fails again (retry should be reset to 0)
	raw.queueResponse(toolCallResponse(
		makeToolCall("call-3", "resetable_tool", map[string]any{"name": "test"}),
	), nil)

	// Fourth call: tool succeeds again
	raw.queueResponse(toolCallResponse(
		makeToolCall("call-4", "resetable_tool", map[string]any{"name": "test"}),
	), nil)

	raw.queueResponse(textResponse("Done"), nil)

	callCount := 0
	var retryValues []int

	resetableTool, _ := NewTool[testDeps, testInput, testOutput](
		"resetable_tool", "Resets retry count",
		func(ctx context.Context, rc *RunContext[testDeps], in testInput) (testOutput, error) {
			callCount++
			retryValues = append(retryValues, rc.Retry)

			// Fail on odd calls (1, 3), succeed on even (2, 4)
			if callCount%2 == 1 {
				return testOutput{}, NewModelRetry("retry please")
			}
			return testOutput{Result: "success"}, nil
		},
	)

	agent, err := New[testDeps, emptyOutput](client,
		WithTools[testDeps, emptyOutput](resetableTool),
		WithRetries[testDeps, emptyOutput](5),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = agent.Run(context.Background(), testDeps{}, WithPrompt("test"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Retry values should be: 0 (fail), 1 (success, resets), 0 (fail), 1 (success)
	expected := []int{0, 1, 0, 1}
	if len(retryValues) != len(expected) {
		t.Fatalf("expected %d retry values, got %d: %v", len(expected), len(retryValues), retryValues)
	}
	for i, v := range expected {
		if retryValues[i] != v {
			t.Errorf("call %d: expected retry=%d, got %d", i+1, v, retryValues[i])
		}
	}
}

// =============================================================================
// Nil Message Tests
// =============================================================================

func TestAgent_Run_NilMessage(t *testing.T) {
	raw, client := newTestClient()

	raw.queueResponse(&types.ChatResponse{
		Choices: []types.Choice{
			{Message: nil},
		},
	}, nil)

	agent, err := New[testDeps, emptyOutput](client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = agent.Run(context.Background(), testDeps{}, WithPrompt("test"))
	if err == nil {
		t.Fatal("expected error for nil message")
	}
	if err.Error() != "no response from model" {
		t.Errorf("unexpected error: %v", err)
	}
}

// =============================================================================
// Integration Tests (Real API Calls)
// =============================================================================
// These tests make real API calls to OpenAI.
// Set OPENAI_API_KEY environment variable to run these tests.
// Run with: OPENAI_API_KEY="your-key" go test -v -run TestIntegration

// TestIntegration_BasicRun tests a basic agent run with real OpenAI API
func TestIntegration_BasicRun(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: OPENAI_API_KEY not set")
	}

	c := openai.NewClient(client.WithAPIKey(apiKey))

	agent, err := New[testDeps, emptyOutput](c,
		WithModel[testDeps, emptyOutput]("gpt-4o-mini"),
		WithSystemPrompt[testDeps, emptyOutput]("You are a helpful assistant. Keep responses brief."),
	)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	result, err := agent.Run(context.Background(), testDeps{}, WithPrompt("Say 'Hello, World!' and nothing else."))
	if err != nil {
		t.Fatalf("agent run failed: %v", err)
	}

	if len(result.Messages) < 2 {
		t.Fatalf("expected at least 2 messages (user + assistant), got %d", len(result.Messages))
	}

	// Verify usage was tracked
	if result.Usage.TotalTokens == 0 {
		t.Error("expected usage to be tracked")
	}

	t.Logf("Messages: %d", len(result.Messages))
	t.Logf("Usage - Prompt: %d, Completion: %d, Total: %d",
		result.Usage.PromptTokens, result.Usage.CompletionTokens, result.Usage.TotalTokens)
}

// TestIntegration_WithTool tests agent with a single tool call
func TestIntegration_WithTool(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: OPENAI_API_KEY not set")
	}

	c := openai.NewClient(client.WithAPIKey(apiKey))

	type calcInput struct {
		A int `json:"a" jsonschema:"First number"`
		B int `json:"b" jsonschema:"Second number"`
	}
	type calcOutput struct {
		Result int `json:"result"`
	}

	// Output type for structured response
	type mathResult struct {
		Answer      int    `json:"answer" jsonschema:"The calculated answer"`
		Explanation string `json:"explanation" jsonschema:"Brief explanation of the calculation"`
	}

	toolCalled := false
	addTool, err := NewTool[testDeps, calcInput, calcOutput](
		"add_numbers",
		"Adds two numbers together and returns the result. Call this tool exactly once with the two numbers to add.",
		func(ctx context.Context, rc *RunContext[testDeps], in calcInput) (calcOutput, error) {
			toolCalled = true
			t.Logf("  [TOOL EXEC] add_numbers called with: a=%d, b=%d, returning %d", in.A, in.B, in.A+in.B)
			return calcOutput{Result: in.A + in.B}, nil
		},
	)
	if err != nil {
		t.Fatalf("failed to create tool: %v", err)
	}

	agent, err := New[testDeps, mathResult](c,
		WithModel[testDeps, mathResult]("gpt-4o-mini"),
		WithSystemPrompt[testDeps, mathResult]("You are a helpful math assistant. Use the add_numbers tool to perform calculations, then provide the result."),
		WithTools[testDeps, mathResult](addTool),
		WithResponseFormat[testDeps, mathResult](types.ResponseFormatModeTool),
	)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	result, err := agent.Run(context.Background(), testDeps{},
		WithPrompt("What is 42 + 17? Use the add_numbers tool to calculate."),
	)
	if err != nil {
		t.Fatalf("agent run failed: %v", err)
	}

	if !toolCalled {
		t.Error("expected tool to be called")
	}

	if result.Output.Answer != 59 {
		t.Errorf("expected answer 59, got %d", result.Output.Answer)
	}

	t.Logf("=== FINAL RESULT ===")
	t.Logf("Answer: %d", result.Output.Answer)
	t.Logf("Explanation: %s", result.Output.Explanation)
	t.Logf("Messages: %d", len(result.Messages))
	t.Logf("Usage - Total: %d tokens", result.Usage.TotalTokens)
}

// TestIntegration_MultipleToolCalls tests agent with multiple tool calls in sequence
func TestIntegration_MultipleToolCalls(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: OPENAI_API_KEY not set")
	}

	c := openai.NewClient(client.WithAPIKey(apiKey))

	type calcInput struct {
		A int `json:"a" jsonschema:"First number"`
		B int `json:"b" jsonschema:"Second number"`
	}
	type calcOutput struct {
		Result int `json:"result"`
	}

	// Output type for structured response
	type mathResult struct {
		FinalAnswer int    `json:"final_answer" jsonschema:"The final calculated answer"`
		Steps       string `json:"steps" jsonschema:"Description of calculation steps performed"`
	}

	addCallCount := 0
	multiplyCallCount := 0

	addTool, err := NewTool[testDeps, calcInput, calcOutput](
		"add",
		"Adds two numbers together",
		func(ctx context.Context, rc *RunContext[testDeps], in calcInput) (calcOutput, error) {
			addCallCount++
			t.Logf("Add called: %d + %d = %d", in.A, in.B, in.A+in.B)
			return calcOutput{Result: in.A + in.B}, nil
		},
	)
	if err != nil {
		t.Fatalf("failed to create add tool: %v", err)
	}

	multiplyTool, err := NewTool[testDeps, calcInput, calcOutput](
		"multiply",
		"Multiplies two numbers together",
		func(ctx context.Context, rc *RunContext[testDeps], in calcInput) (calcOutput, error) {
			multiplyCallCount++
			t.Logf("Multiply called: %d * %d = %d", in.A, in.B, in.A*in.B)
			return calcOutput{Result: in.A * in.B}, nil
		},
	)
	if err != nil {
		t.Fatalf("failed to create multiply tool: %v", err)
	}

	agent, err := New[testDeps, mathResult](c,
		WithModel[testDeps, mathResult]("gpt-4o-mini"),
		WithSystemPrompt[testDeps, mathResult]("You are a math assistant. Use the add and multiply tools to perform calculations step by step."),
		WithTools[testDeps, mathResult](addTool, multiplyTool),
		WithResponseFormat[testDeps, mathResult](types.ResponseFormatModeTool),
	)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	result, err := agent.Run(context.Background(), testDeps{},
		WithPrompt("Calculate (3 + 5) * 2. First add 3 and 5, then multiply the result by 2."),
	)
	if err != nil {
		t.Fatalf("agent run failed: %v", err)
	}

	if addCallCount == 0 {
		t.Error("expected add tool to be called")
	}
	if multiplyCallCount == 0 {
		t.Error("expected multiply tool to be called")
	}

	if result.Output.FinalAnswer != 16 {
		t.Errorf("expected final answer 16, got %d", result.Output.FinalAnswer)
	}

	t.Logf("Final Answer: %d", result.Output.FinalAnswer)
	t.Logf("Steps: %s", result.Output.Steps)
	t.Logf("Add called %d times, Multiply called %d times", addCallCount, multiplyCallCount)
	t.Logf("Messages: %d", len(result.Messages))
}

// TestIntegration_StructuredOutput tests agent with structured output using Native mode
func TestIntegration_StructuredOutput(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: OPENAI_API_KEY not set")
	}

	c := openai.NewClient(client.WithAPIKey(apiKey))

	type PersonInfo struct {
		Name    string `json:"name" jsonschema:"The person's full name"`
		Age     int    `json:"age" jsonschema:"The person's age in years"`
		Country string `json:"country" jsonschema:"The country where the person lives"`
	}

	agent, err := New[testDeps, PersonInfo](c,
		WithModel[testDeps, PersonInfo]("gpt-4o-mini"),
		WithSystemPrompt[testDeps, PersonInfo]("Extract person information from the given text."),
		WithResponseFormat[testDeps, PersonInfo](types.ResponseFormatModeNative),
	)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	result, err := agent.Run(context.Background(), testDeps{},
		WithPrompt("John Smith is a 32-year-old software engineer living in Canada."),
	)
	if err != nil {
		t.Fatalf("agent run failed: %v", err)
	}

	if result.Output.Name == "" {
		t.Error("expected name to be extracted")
	}
	if result.Output.Age == 0 {
		t.Error("expected age to be extracted")
	}
	if result.Output.Country == "" {
		t.Error("expected country to be extracted")
	}

	t.Logf("Extracted: Name=%q, Age=%d, Country=%q",
		result.Output.Name, result.Output.Age, result.Output.Country)
}

// TestIntegration_StructuredOutputTool tests agent with structured output using Tool mode
func TestIntegration_StructuredOutputTool(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: OPENAI_API_KEY not set")
	}

	c := openai.NewClient(client.WithAPIKey(apiKey))

	type SentimentResult struct {
		Sentiment  string  `json:"sentiment" jsonschema:"The sentiment (positive/negative/neutral)"`
		Confidence float64 `json:"confidence" jsonschema:"Confidence score between 0 and 1"`
		Reasoning  string  `json:"reasoning" jsonschema:"Brief explanation for the sentiment"`
	}

	agent, err := New[testDeps, SentimentResult](c,
		WithModel[testDeps, SentimentResult]("gpt-4o-mini"),
		WithSystemPrompt[testDeps, SentimentResult]("Analyze the sentiment of the given text."),
		WithResponseFormat[testDeps, SentimentResult](types.ResponseFormatModeTool),
	)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	result, err := agent.Run(context.Background(), testDeps{},
		WithPrompt("I absolutely love this product! It's the best purchase I've ever made."),
	)
	if err != nil {
		t.Fatalf("agent run failed: %v", err)
	}

	if result.Output.Sentiment == "" {
		t.Error("expected sentiment to be extracted")
	}
	if result.Output.Confidence == 0 {
		t.Error("expected confidence to be non-zero")
	}

	t.Logf("Sentiment: %s (confidence: %.2f)", result.Output.Sentiment, result.Output.Confidence)
	t.Logf("Reasoning: %s", result.Output.Reasoning)
}

// TestIntegration_WithDependencies tests that dependencies are properly passed to tools
func TestIntegration_WithDependencies(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: OPENAI_API_KEY not set")
	}

	c := openai.NewClient(client.WithAPIKey(apiKey))

	type userDB struct {
		users map[string]string
	}

	type lookupInput struct {
		UserID string `json:"user_id" jsonschema:"The user ID to look up"`
	}
	type lookupOutput struct {
		Name  string `json:"name"`
		Found bool   `json:"found"`
	}

	lookupTool, err := NewTool[userDB, lookupInput, lookupOutput](
		"lookup_user",
		"Looks up a user by their ID",
		func(ctx context.Context, rc *RunContext[userDB], in lookupInput) (lookupOutput, error) {
			if name, ok := rc.Deps.users[in.UserID]; ok {
				return lookupOutput{Name: name, Found: true}, nil
			}
			return lookupOutput{Found: false}, nil
		},
	)
	if err != nil {
		t.Fatalf("failed to create tool: %v", err)
	}

	agent, err := New[userDB, emptyOutput](c,
		WithModel[userDB, emptyOutput]("gpt-4o-mini"),
		WithSystemPrompt[userDB, emptyOutput]("You are a user lookup assistant. Use the lookup_user tool to find users."),
		WithTools[userDB, emptyOutput](lookupTool),
	)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	deps := userDB{
		users: map[string]string{
			"u123": "Alice Smith",
			"u456": "Bob Jones",
		},
	}

	result, err := agent.Run(context.Background(), deps,
		WithPrompt("Look up user u123 and tell me their name."),
	)
	if err != nil {
		t.Fatalf("agent run failed: %v", err)
	}

	t.Logf("Messages: %d", len(result.Messages))
	t.Logf("Total tokens: %d", result.Usage.TotalTokens)
}

// TestIntegration_ConversationContinuation tests continuing a conversation with messages
func TestIntegration_ConversationContinuation(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: OPENAI_API_KEY not set")
	}

	c := openai.NewClient(client.WithAPIKey(apiKey))

	agent, err := New[testDeps, emptyOutput](c,
		WithModel[testDeps, emptyOutput]("gpt-4o-mini"),
		WithSystemPrompt[testDeps, emptyOutput]("You are a helpful assistant with a good memory."),
	)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// First turn
	result1, err := agent.Run(context.Background(), testDeps{},
		WithPrompt("My favorite color is blue. Remember that."),
	)
	if err != nil {
		t.Fatalf("first run failed: %v", err)
	}

	// Second turn - continue the conversation
	result2, err := agent.Run(context.Background(), testDeps{},
		WithMessages(result1.Messages),
		WithPrompt("What is my favorite color?"),
	)
	if err != nil {
		t.Fatalf("second run failed: %v", err)
	}

	// Should have more messages now
	if len(result2.Messages) <= len(result1.Messages) {
		t.Error("expected more messages after continuation")
	}

	t.Logf("First turn messages: %d", len(result1.Messages))
	t.Logf("Second turn messages: %d", len(result2.Messages))
}

// TestIntegration_ToolWithRetry tests tool retry functionality with real API
func TestIntegration_ToolWithRetry(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: OPENAI_API_KEY not set")
	}

	c := openai.NewClient(client.WithAPIKey(apiKey))

	type searchInput struct {
		Query string `json:"query" jsonschema:"The search query"`
	}
	type searchOutput struct {
		Results []string `json:"results"`
	}

	callCount := 0
	searchTool, err := NewTool[testDeps, searchInput, searchOutput](
		"search",
		"Searches for information. May fail on first attempt.",
		func(ctx context.Context, rc *RunContext[testDeps], in searchInput) (searchOutput, error) {
			callCount++
			t.Logf("Search called (attempt %d, retry count %d): %q", callCount, rc.Retry, in.Query)
			if callCount == 1 {
				// Fail first attempt to trigger retry
				return searchOutput{}, NewModelRetry("Search temporarily unavailable, please try again")
			}
			return searchOutput{Results: []string{"Result 1", "Result 2"}}, nil
		},
	)
	if err != nil {
		t.Fatalf("failed to create tool: %v", err)
	}

	agent, err := New[testDeps, emptyOutput](c,
		WithModel[testDeps, emptyOutput]("gpt-4o-mini"),
		WithSystemPrompt[testDeps, emptyOutput]("You are a search assistant. Use the search tool."),
		WithTools[testDeps, emptyOutput](searchTool),
		WithRetries[testDeps, emptyOutput](3),
	)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	result, err := agent.Run(context.Background(), testDeps{},
		WithPrompt("Search for 'golang tutorials'"),
	)
	if err != nil {
		t.Fatalf("agent run failed: %v", err)
	}

	if callCount < 2 {
		t.Errorf("expected tool to be called at least 2 times, got %d", callCount)
	}

	t.Logf("Tool called %d times", callCount)
	t.Logf("Total messages: %d", len(result.Messages))
}

// TestIntegration_UsageLimits tests usage limits with real API
func TestIntegration_UsageLimits(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: OPENAI_API_KEY not set")
	}

	c := openai.NewClient(client.WithAPIKey(apiKey))

	type echoInput struct {
		Text string `json:"text"`
	}
	type echoOutput struct {
		Echo string `json:"echo"`
	}

	callCount := 0
	echoTool, err := NewTool[testDeps, echoInput, echoOutput](
		"echo",
		"Echoes the input text back",
		func(ctx context.Context, rc *RunContext[testDeps], in echoInput) (echoOutput, error) {
			callCount++
			return echoOutput{Echo: in.Text}, nil
		},
	)
	if err != nil {
		t.Fatalf("failed to create tool: %v", err)
	}

	agent, err := New[testDeps, emptyOutput](c,
		WithModel[testDeps, emptyOutput]("gpt-4o-mini"),
		WithSystemPrompt[testDeps, emptyOutput]("Use the echo tool for each word the user provides."),
		WithTools[testDeps, emptyOutput](echoTool),
	)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Set a low tool calls limit
	_, err = agent.Run(context.Background(), testDeps{},
		WithPrompt("Echo these words one at a time: apple, banana, cherry, date, elderberry"),
		WithUsageLimits(UsageLimits{
			ToolCallsLimit: 2,
		}),
	)

	if err == nil {
		t.Log("Note: LLM didn't exceed tool limit (may have called tools in batch)")
	} else {
		var limitErr *UsageLimitExceeded
		if errors.As(err, &limitErr) {
			t.Logf("Tool calls limit exceeded as expected: %v", err)
		} else {
			t.Fatalf("unexpected error type: %v", err)
		}
	}

	t.Logf("Total tool calls made: %d", callCount)
}

// TestIntegration_SystemPromptFunc tests dynamic system prompt generation
func TestIntegration_SystemPromptFunc(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: OPENAI_API_KEY not set")
	}

	c := openai.NewClient(client.WithAPIKey(apiKey))

	type userContext struct {
		UserName string
		Language string
	}

	agent, err := New[userContext, emptyOutput](c,
		WithModel[userContext, emptyOutput]("gpt-4o-mini"),
		WithSystemPromptFunc[userContext, emptyOutput](func(deps userContext) string {
			return fmt.Sprintf("You are helping user %s. Respond in %s. Keep responses brief.",
				deps.UserName, deps.Language)
		}),
	)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	deps := userContext{
		UserName: "Alice",
		Language: "English",
	}

	result, err := agent.Run(context.Background(), deps,
		WithPrompt("Say hello to me by name."),
	)
	if err != nil {
		t.Fatalf("agent run failed: %v", err)
	}

	t.Logf("Messages: %d", len(result.Messages))
	t.Logf("Total tokens: %d", result.Usage.TotalTokens)
}

// =============================================================================
// Response Format Mode Tests
// =============================================================================

// TestIntegration_ResponseFormat_Native tests structured output using Native mode
// Native mode uses the provider's native JSON schema support (response_format API field)
func TestIntegration_ResponseFormat_Native(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: OPENAI_API_KEY not set")
	}

	c := openai.NewClient(client.WithAPIKey(apiKey))

	type MovieReview struct {
		Title       string   `json:"title" jsonschema:"The movie title"`
		Rating      int      `json:"rating" jsonschema:"Rating from 1 to 10"`
		Pros        []string `json:"pros" jsonschema:"List of positive aspects"`
		Cons        []string `json:"cons" jsonschema:"List of negative aspects"`
		Recommended bool     `json:"recommended" jsonschema:"Whether you would recommend this movie"`
	}

	agent, err := New[testDeps, MovieReview](c,
		WithModel[testDeps, MovieReview]("gpt-4o-mini"),
		WithSystemPrompt[testDeps, MovieReview]("You are a movie critic. Analyze movies and provide structured reviews."),
		WithResponseFormat[testDeps, MovieReview](types.ResponseFormatModeNative),
	)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	result, err := agent.Run(context.Background(), testDeps{},
		WithPrompt("Review the movie 'The Matrix' (1999). Be concise."),
	)
	if err != nil {
		t.Fatalf("agent run failed: %v", err)
	}

	// Validate structured output
	if result.Output.Title == "" {
		t.Error("expected title to be extracted")
	}
	if result.Output.Rating < 1 || result.Output.Rating > 10 {
		t.Errorf("expected rating between 1-10, got %d", result.Output.Rating)
	}
	if len(result.Output.Pros) == 0 {
		t.Error("expected at least one pro")
	}
	if len(result.Output.Cons) == 0 {
		t.Error("expected at least one con")
	}

	t.Logf("Native Mode Result:")
	t.Logf("  Title: %s", result.Output.Title)
	t.Logf("  Rating: %d/10", result.Output.Rating)
	t.Logf("  Pros: %v", result.Output.Pros)
	t.Logf("  Cons: %v", result.Output.Cons)
	t.Logf("  Recommended: %v", result.Output.Recommended)
	t.Logf("  Total tokens: %d", result.Usage.TotalTokens)
}

// TestIntegration_ResponseFormat_Tool tests structured output using Tool mode
// Tool mode injects an _output tool that the LLM must call to return structured data
func TestIntegration_ResponseFormat_Tool(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: OPENAI_API_KEY not set")
	}

	c := openai.NewClient(client.WithAPIKey(apiKey))

	type RecipeInfo struct {
		Name        string   `json:"name" jsonschema:"The recipe name"`
		PrepTime    int      `json:"prep_time_minutes" jsonschema:"Preparation time in minutes"`
		CookTime    int      `json:"cook_time_minutes" jsonschema:"Cooking time in minutes"`
		Servings    int      `json:"servings" jsonschema:"Number of servings"`
		Ingredients []string `json:"ingredients" jsonschema:"List of ingredients"`
		Difficulty  string   `json:"difficulty" jsonschema:"Difficulty level (easy/medium/hard)"`
	}

	agent, err := New[testDeps, RecipeInfo](c,
		WithModel[testDeps, RecipeInfo]("gpt-4o-mini"),
		WithSystemPrompt[testDeps, RecipeInfo]("You are a chef. Extract recipe information from descriptions."),
		WithResponseFormat[testDeps, RecipeInfo](types.ResponseFormatModeTool),
	)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	result, err := agent.Run(context.Background(), testDeps{},
		WithPrompt("Extract info for a simple pasta carbonara recipe. It takes about 10 minutes to prep and 20 minutes to cook, serves 4, and uses spaghetti, eggs, parmesan, pancetta, and black pepper."),
	)
	if err != nil {
		t.Fatalf("agent run failed: %v", err)
	}

	// Validate structured output
	if result.Output.Name == "" {
		t.Error("expected name to be extracted")
	}
	if result.Output.PrepTime == 0 {
		t.Error("expected prep time to be extracted")
	}
	if result.Output.CookTime == 0 {
		t.Error("expected cook time to be extracted")
	}
	if result.Output.Servings == 0 {
		t.Error("expected servings to be extracted")
	}
	if len(result.Output.Ingredients) == 0 {
		t.Error("expected at least one ingredient")
	}
	if result.Output.Difficulty == "" {
		t.Error("expected difficulty to be extracted")
	}

	t.Logf("Tool Mode Result:")
	t.Logf("  Name: %s", result.Output.Name)
	t.Logf("  Prep Time: %d min", result.Output.PrepTime)
	t.Logf("  Cook Time: %d min", result.Output.CookTime)
	t.Logf("  Servings: %d", result.Output.Servings)
	t.Logf("  Ingredients: %v", result.Output.Ingredients)
	t.Logf("  Difficulty: %s", result.Output.Difficulty)
	t.Logf("  Total tokens: %d", result.Usage.TotalTokens)
}

// TestIntegration_ResponseFormat_Prompted tests structured output using Prompted mode
// Prompted mode appends instructions to the prompt asking for JSON output
func TestIntegration_ResponseFormat_Prompted(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: OPENAI_API_KEY not set")
	}

	c := openai.NewClient(client.WithAPIKey(apiKey))

	type BookSummary struct {
		Title       string   `json:"title" jsonschema:"The book title"`
		Author      string   `json:"author" jsonschema:"The author's name"`
		Genre       string   `json:"genre" jsonschema:"The book's genre"`
		YearWritten int      `json:"year_written" jsonschema:"Year the book was written"`
		Themes      []string `json:"themes" jsonschema:"Main themes of the book"`
		PageCount   int      `json:"page_count" jsonschema:"Approximate page count"`
	}

	agent, err := New[testDeps, BookSummary](c,
		WithModel[testDeps, BookSummary]("gpt-4o-mini"),
		WithSystemPrompt[testDeps, BookSummary]("You are a librarian. Provide information about books."),
		WithResponseFormat[testDeps, BookSummary](types.ResponseFormatModePrompted),
	)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	result, err := agent.Run(context.Background(), testDeps{},
		WithPrompt("Provide information about '1984' by George Orwell."),
	)
	if err != nil {
		t.Fatalf("agent run failed: %v", err)
	}

	// Validate structured output
	if result.Output.Title == "" {
		t.Error("expected title to be extracted")
	}
	if result.Output.Author == "" {
		t.Error("expected author to be extracted")
	}
	if result.Output.Genre == "" {
		t.Error("expected genre to be extracted")
	}
	if result.Output.YearWritten == 0 {
		t.Error("expected year to be extracted")
	}
	if len(result.Output.Themes) == 0 {
		t.Error("expected at least one theme")
	}

	t.Logf("Prompted Mode Result:")
	t.Logf("  Title: %s", result.Output.Title)
	t.Logf("  Author: %s", result.Output.Author)
	t.Logf("  Genre: %s", result.Output.Genre)
	t.Logf("  Year: %d", result.Output.YearWritten)
	t.Logf("  Themes: %v", result.Output.Themes)
	t.Logf("  Page Count: %d", result.Output.PageCount)
	t.Logf("  Total tokens: %d", result.Usage.TotalTokens)
}

// TestIntegration_ResponseFormat_ToolWithOtherTools tests Tool mode alongside regular tools
// This verifies that _output tool works correctly when other tools are also available
func TestIntegration_ResponseFormat_ToolWithOtherTools(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: OPENAI_API_KEY not set")
	}

	c := openai.NewClient(client.WithAPIKey(apiKey))

	type WeatherReport struct {
		Location    string `json:"location" jsonschema:"The location queried"`
		Temperature int    `json:"temperature" jsonschema:"Temperature in Fahrenheit"`
		Condition   string `json:"condition" jsonschema:"Weather condition (sunny/cloudy/rainy/etc)"`
		Summary     string `json:"summary" jsonschema:"Brief weather summary"`
	}

	type weatherInput struct {
		City string `json:"city" jsonschema:"City name to get weather for"`
	}
	type weatherOutput struct {
		Temp      int    `json:"temp"`
		Condition string `json:"condition"`
	}

	getWeatherTool, err := NewTool[testDeps, weatherInput, weatherOutput](
		"get_weather",
		"Gets the current weather for a city. Call this exactly once.",
		func(ctx context.Context, rc *RunContext[testDeps], in weatherInput) (weatherOutput, error) {
			t.Logf("  [TOOL EXEC] get_weather called for: %s, returning temp=72, condition=sunny", in.City)
			// Return mock weather data
			return weatherOutput{Temp: 72, Condition: "sunny"}, nil
		},
	)
	if err != nil {
		t.Fatalf("failed to create tool: %v", err)
	}

	agent, err := New[testDeps, WeatherReport](c,
		WithModel[testDeps, WeatherReport]("gpt-4o-mini"),
		WithSystemPrompt[testDeps, WeatherReport]("You are a weather assistant. Use the get_weather tool exactly once to fetch weather data, then provide your final structured report."),
		WithTools[testDeps, WeatherReport](getWeatherTool),
		WithResponseFormat[testDeps, WeatherReport](types.ResponseFormatModeTool),
	)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	result, err := agent.Run(context.Background(), testDeps{},
		WithPrompt("What's the weather like in San Francisco?"),
	)
	if err != nil {
		t.Fatalf("agent run failed: %v", err)
	}

	// Validate structured output
	if result.Output.Location == "" {
		t.Error("expected location to be set")
	}
	if result.Output.Temperature == 0 {
		t.Error("expected temperature to be set")
	}
	if result.Output.Condition == "" {
		t.Error("expected condition to be set")
	}

	t.Logf("=== FINAL RESULT ===")
	t.Logf("  Location: %s", result.Output.Location)
	t.Logf("  Temperature: %d°F", result.Output.Temperature)
	t.Logf("  Condition: %s", result.Output.Condition)
	t.Logf("  Summary: %s", result.Output.Summary)
	t.Logf("  Total tokens: %d", result.Usage.TotalTokens)
}
