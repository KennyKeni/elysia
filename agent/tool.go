package agent

import (
	"context"
	"errors"
	json "encoding/json/v2"
	"fmt"

	"github.com/KennyKeni/elysia/types"
)

// ModelRetry is returned by tool handlers to request a retry with feedback to the LLM.
// The message is sent back to the LLM so it can adjust its approach.
type ModelRetry struct {
	Message string
}

func (e *ModelRetry) Error() string {
	return e.Message
}

// NewModelRetry creates a ModelRetry error with the given feedback message.
func NewModelRetry(message string) *ModelRetry {
	return &ModelRetry{Message: message}
}

// IsModelRetry checks if an error is a ModelRetry and returns it.
func IsModelRetry(err error) (*ModelRetry, bool) {
	var mr *ModelRetry
	if errors.As(err, &mr) {
		return mr, true
	}
	return nil, false
}

// RunContext provides context to tool handlers during execution.
type RunContext[TDep any] struct {
	// Deps contains user-provided dependencies (DB connections, API clients, etc.)
	Deps TDep

	// Messages is the full conversation history so far
	Messages []types.Message

	// Usage tracks token consumption for this run
	Usage types.Usage

	// Retry is the current retry attempt (0 = first attempt)
	Retry int

	// MaxRetries is the maximum retry count configured for this tool
	MaxRetries int

	// ToolCallID is the unique ID for this specific tool call
	ToolCallID string

	// RunID is the unique ID for the entire agent run (useful for tracing)
	RunID string

	// Prompt is the original user prompt that started this run
	Prompt string

	// PartialOutput indicates whether this is a partial (streaming) output.
	// NOTE: Streaming not yet supported - this field is reserved for future use.
	PartialOutput bool
}

// LastAttempt returns true if this is the final attempt before failure.
func (rc *RunContext[TDep]) LastAttempt() bool {
	return rc.Retry >= rc.MaxRetries
}

type Tool[TDep any] struct {
	types.ToolDefinition
	Execute func(ctx context.Context, rc *RunContext[TDep], args map[string]any) (*types.ToolResult, error)
	Retries int // Per-tool retry count (0 = use agent default)
}

// ToolOption configures a Tool.
type ToolOption[TDep any] func(*Tool[TDep])

// ToolRetries sets the retry count for a specific tool, overriding the agent default.
func ToolRetries[TDep any](retries int) ToolOption[TDep] {
	return func(t *Tool[TDep]) {
		t.Retries = retries
	}
}

// WrapTool wraps a types.Tool (MCP, external tools) into an agent.Tool
func WrapTool[TDep any](tool *types.Tool, opts ...ToolOption[TDep]) *Tool[TDep] {
	t := &Tool[TDep]{
		ToolDefinition: tool.ToolDefinition,
		Execute: func(ctx context.Context, rc *RunContext[TDep], args map[string]any) (*types.ToolResult, error) {
			return tool.Execute(ctx, args)
		},
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// NewTool creates an agent tool with typed input/output and RunContext access.
// Use ToolRetries to override the agent's default retry count for this tool.
func NewTool[TDep, TIn, TOut any](
	name, description string,
	handler func(context.Context, *RunContext[TDep], TIn) (TOut, error),
	opts ...ToolOption[TDep],
) (*Tool[TDep], error) {
	resolvedInputSchema, err := types.ResolveSchemaFor[TIn]()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve input schema: %w", err)
	}

	resolvedOutputSchema, err := types.ResolveSchemaFor[TOut]()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve output schema: %w", err)
	}

	inputSchemaMap, err := types.SchemaMapFor[TIn]()
	if err != nil {
		return nil, fmt.Errorf("failed to generate input schema map: %w", err)
	}

	outputSchemaMap, err := types.SchemaMapFor[TOut]()
	if err != nil {
		return nil, fmt.Errorf("failed to generate output schema map: %w", err)
	}

	validateAndExecute := func(ctx context.Context, rc *RunContext[TDep], args map[string]any) (*types.ToolResult, error) {
		// Validate input against the schema (args is already map[string]any)
		if err := resolvedInputSchema.Validate(args); err != nil {
			// Input validation error - return as ModelRetry for retry handling
			return nil, &ModelRetry{Message: fmt.Sprintf("input validation error: %v", err)}
		}

		// Unmarshal args into typed input
		typedInput, err := types.UnmarshalToolArgs[TIn](args)
		if err != nil {
			return nil, &ModelRetry{Message: fmt.Sprintf("failed to unmarshal input: %v", err)}
		}

		// Run handler - may return ModelRetry or other errors
		output, err := handler(ctx, rc, typedInput)
		if err != nil {
			// Pass through ModelRetry, wrap other errors
			if _, ok := IsModelRetry(err); ok {
				return nil, err
			}
			// Non-retry errors become ToolResult with IsError=true
			return types.ToolResultFromError(err), nil
		}

		// Validate output against the schema (output is a struct, need ValidateStruct)
		if err := types.ValidateStruct(resolvedOutputSchema, output); err != nil {
			return types.ToolResultFromError(fmt.Errorf("output validation error: %w", err)), nil
		}

		// Marshal output to ToolResult
		outputJSON, err := json.Marshal(output)
		if err != nil {
			return types.ToolResultFromError(fmt.Errorf("failed to marshal output: %w", err)), nil
		}

		return &types.ToolResult{
			ContentPart: []types.ContentPart{
				types.NewContentPartText(string(outputJSON)),
			},
			StructuredContent: output,
			IsError:           false,
		}, nil
	}

	t := &Tool[TDep]{
		ToolDefinition: types.ToolDefinition{
			Name:         name,
			Description:  description,
			InputSchema:  inputSchemaMap,
			OutputSchema: outputSchemaMap,
		},
		Execute: validateAndExecute,
	}

	for _, opt := range opts {
		opt(t)
	}

	return t, nil
}

func GetToolDefinitions[TDep any](tools []*Tool[TDep]) []types.ToolDefinition {
	res := make([]types.ToolDefinition, len(tools))
	for i, tool := range tools {
		res[i] = tool.ToolDefinition
	}
	return res
}
