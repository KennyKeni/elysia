package agent

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"

	"github.com/KennyKeni/elysia/types"
	"github.com/google/uuid"
)

type RunResult[TOut any] struct {
	Output   TOut
	Messages []types.Message
	Usage    types.Usage
}

// UsageLimits sets hard ceilings on an agent run.
type UsageLimits struct {
	// RequestLimit is the maximum number of LLM round-trips (0 = unlimited)
	RequestLimit int

	// CompletionTokensLimit is the maximum completion tokens per LLM response (0 = unlimited)
	CompletionTokensLimit int

	// ToolCallsLimit is the maximum successful tool executions (0 = unlimited)
	// Failed/retrying calls don't count
	ToolCallsLimit int
}

// UsageLimitExceeded is returned when a usage limit is exceeded.
type UsageLimitExceeded struct {
	Limit string
	Value int
	Max   int
}

func (e *UsageLimitExceeded) Error() string {
	return fmt.Sprintf("usage limit exceeded: %s (%d >= %d)", e.Limit, e.Value, e.Max)
}

type Agent[TDep, TOut any] struct {
	systemPrompt       string
	systemPromptFunc   func(TDep) string
	client             types.Client
	model              string                 // Model to use for chat requests
	toolMap            map[string]*Tool[TDep] // For O(1) lookup
	toolList           []*Tool[TDep]          // For O(1) iteration, preserves order
	maxIterations      int
	responseFormatMode types.ResponseFormatMode
	retries            int // Default retry count for tools
	outputRetries      int // Retry count for output validation (falls back to retries if 0)
}

type Option[TDep, TOut any] func(*Agent[TDep, TOut]) error

func New[TDep, TOut any](client types.Client, opts ...Option[TDep, TOut]) (*Agent[TDep, TOut], error) {
	a := &Agent[TDep, TOut]{
		client:        client,
		maxIterations: 10,
		toolMap:       make(map[string]*Tool[TDep]),
		toolList:      make([]*Tool[TDep], 0),
	}

	for _, opt := range opts {
		if err := opt(a); err != nil {
			return nil, err
		}
	}

	return a, nil
}

func WithSystemPrompt[TDep, TOut any](prompt string) Option[TDep, TOut] {
	return func(a *Agent[TDep, TOut]) error {
		a.systemPrompt = prompt
		return nil
	}
}

func WithSystemPromptFunc[TDep, TOut any](fn func(TDep) string) Option[TDep, TOut] {
	return func(a *Agent[TDep, TOut]) error {
		a.systemPromptFunc = fn
		return nil
	}
}

func WithTools[TDep, TOut any](tools ...*Tool[TDep]) Option[TDep, TOut] {
	return func(a *Agent[TDep, TOut]) error {
		for _, t := range tools {
			if _, exists := a.toolMap[t.Name]; exists {
				return fmt.Errorf("duplicate tool name: %s", t.Name)
			}
			a.toolMap[t.Name] = t
			a.toolList = append(a.toolList, t)
		}
		return nil
	}
}

func WithResponseFormat[TDep, TOut any](mode types.ResponseFormatMode) Option[TDep, TOut] {
	return func(a *Agent[TDep, TOut]) error {
		a.responseFormatMode = mode
		return nil
	}
}

func WithRetries[TDep, TOut any](retries int) Option[TDep, TOut] {
	return func(a *Agent[TDep, TOut]) error {
		a.retries = retries
		return nil
	}
}

func WithOutputRetries[TDep, TOut any](retries int) Option[TDep, TOut] {
	return func(a *Agent[TDep, TOut]) error {
		a.outputRetries = retries
		return nil
	}
}

func WithModel[TDep, TOut any](model string) Option[TDep, TOut] {
	return func(a *Agent[TDep, TOut]) error {
		a.model = model
		return nil
	}
}

type runConfig struct {
	prompt      string
	messages    []types.Message
	retries     *int         // Override agent-level retries if set
	usageLimits *UsageLimits // Hard ceilings on this run
}
type RunOption func(*runConfig)

func WithPrompt(prompt string) RunOption {
	return func(rc *runConfig) {
		rc.prompt = prompt
	}
}

func WithMessages(messages []types.Message) RunOption {
	return func(rc *runConfig) {
		rc.messages = messages
	}
}

func WithRunRetries(retries int) RunOption {
	return func(rc *runConfig) {
		rc.retries = &retries
	}
}

func WithUsageLimits(limits UsageLimits) RunOption {
	return func(rc *runConfig) {
		rc.usageLimits = &limits
	}
}

func (a *Agent[TDep, TOut]) Run(ctx context.Context, dep TDep, opts ...RunOption) (*RunResult[TOut], error) {
	var err error
	var res TOut
	var rf types.ResponseFormat

	runCfg := runConfig{}
	for _, opt := range opts {
		opt(&runCfg)
	}

	if a.responseFormatMode != "" {
		rf, err = types.ResponseFormatFor[TOut](a.responseFormatMode, "", "")
		if err != nil {
			return nil, fmt.Errorf("failed to build response format: %w", err)
		}
	}

	var systemPrompt string
	if a.systemPromptFunc != nil {
		systemPrompt = a.systemPromptFunc(dep)
	} else {
		systemPrompt = a.systemPrompt
	}

	toolDefs := GetToolDefinitions(a.toolList)

	// Generate unique run ID
	runID := uuid.New().String()

	// Initialize RunContext
	rc := &RunContext[TDep]{
		Deps:     dep,
		Messages: runCfg.messages,
		RunID:    runID,
		Prompt:   runCfg.prompt,
	}
	if runCfg.prompt != "" {
		rc.Messages = append(rc.Messages, types.NewUserMessage(types.WithText(runCfg.prompt)))
	}

	// Track retry counts per tool across iterations
	toolRetries := make(map[string]int)

	// Track usage for limits
	var requestCount int
	var successfulToolCalls int

	// Track output validation retries
	var outputRetryCount int
	maxOutputRetries := a.getEffectiveOutputRetries()

	for i := 0; i < a.maxIterations; i++ {
		// Check request limit
		if runCfg.usageLimits != nil && runCfg.usageLimits.RequestLimit > 0 {
			if requestCount >= runCfg.usageLimits.RequestLimit {
				return nil, &UsageLimitExceeded{Limit: "request_limit", Value: requestCount, Max: runCfg.usageLimits.RequestLimit}
			}
		}

		resp, err := a.client.Chat(ctx, &types.ChatParams{
			Model:          a.model,
			Messages:       rc.Messages,
			SystemPrompt:   systemPrompt,
			Tools:          toolDefs,
			ResponseFormat: rf,
		})
		requestCount++

		if err != nil {
			// Check if it's a recoverable output validation error
			if isOutputValidationError(err) {
				if outputRetryCount >= maxOutputRetries {
					return nil, fmt.Errorf("output validation exceeded max retries (%d): %w", maxOutputRetries, err)
				}
				outputRetryCount++
				// Add feedback message for LLM to see
				rc.Messages = append(rc.Messages, types.NewUserMessage(
					types.WithText(fmt.Sprintf("Output validation error: %v. Please try again.", err)),
				))
				continue
			}
			return nil, err
		}

		if len(resp.Choices) == 0 || resp.Choices[0].Message == nil {
			return nil, fmt.Errorf("no response from model")
		}
		choice := &resp.Choices[0]
		msg := choice.Message

		// Check completion tokens limit
		if runCfg.usageLimits != nil && runCfg.usageLimits.CompletionTokensLimit > 0 && resp.Usage != nil {
			if int(resp.Usage.CompletionTokens) > runCfg.usageLimits.CompletionTokensLimit {
				return nil, &UsageLimitExceeded{Limit: "completion_tokens_limit", Value: int(resp.Usage.CompletionTokens), Max: runCfg.usageLimits.CompletionTokensLimit}
			}
		}

		if resp.Usage != nil {
			rc.Usage.PromptTokens += resp.Usage.PromptTokens
			rc.Usage.CompletionTokens += resp.Usage.CompletionTokens
			rc.Usage.TotalTokens += resp.Usage.TotalTokens
		}

		rc.Messages = append(rc.Messages, *msg)

		// Case 1: No tool calls - model is done
		if len(msg.ToolCalls) == 0 {
			if choice.StructuredContent != "" {
				if err := json.Unmarshal([]byte(choice.StructuredContent), &res); err != nil {
					// Unmarshal failed - retry if within limit
					if outputRetryCount >= maxOutputRetries {
						return nil, fmt.Errorf("output unmarshal exceeded max retries (%d): %w", maxOutputRetries, err)
					}
					outputRetryCount++
					rc.Messages = append(rc.Messages, types.NewUserMessage(
						types.WithText(fmt.Sprintf("Failed to parse output: %v. Please provide valid output.", err)),
					))
					continue
				}
			} else if rf.Schema != nil {
				// Expected structured output but got none - retry if within limit
				if outputRetryCount >= maxOutputRetries {
					return nil, fmt.Errorf("expected structured output but got none (max retries %d exceeded)", maxOutputRetries)
				}
				outputRetryCount++
				rc.Messages = append(rc.Messages, types.NewUserMessage(
					types.WithText("Expected structured output but received none. Please provide the output in the required format."),
				))
				continue
			}
			return &RunResult[TOut]{
				Output:   res,
				Messages: rc.Messages,
				Usage:    rc.Usage,
			}, nil
		}

		// Case 2: Has tool calls - execute them all, collect results
		for _, tc := range msg.ToolCalls {
			tool := a.findTool(tc.Function.Name)
			if tool == nil {
				return nil, fmt.Errorf("unknown tool: %s", tc.Function.Name)
			}

			// Get retry count for this tool and check limit
			retryCount := toolRetries[tool.Name]
			maxRetries := a.getEffectiveRetries(tool, runCfg.retries)

			// Set RunContext fields for this tool call
			rc.Retry = retryCount
			rc.MaxRetries = maxRetries
			rc.ToolCallID = tc.ID

			result, execErr := tool.Execute(ctx, rc, tc.Function.Arguments)

			if execErr != nil {
				// Check if it's a ModelRetry error
				if mr, ok := IsModelRetry(execErr); ok {
					if retryCount >= maxRetries {
						return nil, fmt.Errorf("tool %q exceeded max retries (%d): %w", tool.Name, maxRetries, execErr)
					}
					// Increment retry count for next iteration
					toolRetries[tool.Name] = retryCount + 1
					// Convert to error result for LLM to see
					result = &types.ToolResult{
						ContentPart: []types.ContentPart{
							types.NewContentPartText(mr.Message),
						},
						IsError: true,
					}
				} else {
					// Non-ModelRetry error - fatal
					return nil, fmt.Errorf("tool execution failed: %w", execErr)
				}
			} else {
				// Success - reset retry count for this tool
				toolRetries[tool.Name] = 0
				successfulToolCalls++

				// Check tool calls limit
				if runCfg.usageLimits != nil && runCfg.usageLimits.ToolCallsLimit > 0 {
					if successfulToolCalls > runCfg.usageLimits.ToolCallsLimit {
						return nil, &UsageLimitExceeded{Limit: "tool_calls_limit", Value: successfulToolCalls, Max: runCfg.usageLimits.ToolCallsLimit}
					}
				}
			}

			rc.Messages = append(rc.Messages, types.NewToolResultMessage(tc.ID, result))
		}
	}

	return nil, fmt.Errorf("agent exceeded max iterations (%d)", a.maxIterations)
}

// getEffectiveRetries returns the retry count for a tool call.
// Priority: run override > tool-specific > agent default
func (a *Agent[TDep, TOut]) getEffectiveRetries(tool *Tool[TDep], runRetries *int) int {
	if runRetries != nil {
		return *runRetries
	}
	if tool.Retries > 0 {
		return tool.Retries
	}
	return a.retries
}

// getEffectiveOutputRetries returns the retry count for output validation.
// Falls back to retries if outputRetries is 0.
func (a *Agent[TDep, TOut]) getEffectiveOutputRetries() int {
	if a.outputRetries > 0 {
		return a.outputRetries
	}
	return a.retries
}

// isOutputValidationError returns true if the error is a recoverable output validation error.
func isOutputValidationError(err error) bool {
	var schemaErr *types.SchemaValidationError
	var toolNotCalledErr *types.ToolNotCalledError
	var misuseErr *types.OutputToolMisuseError
	return errors.As(err, &schemaErr) ||
		errors.As(err, &toolNotCalledErr) ||
		errors.As(err, &misuseErr)
}

func (a *Agent[TDep, TOut]) findTool(name string) *Tool[TDep] {
	return a.toolMap[name]
}
