package agent

import (
	"context"
	"encoding/json/v2"
	"fmt"

	"github.com/KennyKeni/elysia/types"
)

type RunResult[TOut any] struct {
	Output   TOut
	Messages []types.Message
	Usage    types.Usage
}

type Agent[TDep, TOut any] struct {
	systemPrompt       string
	systemPromptFunc   func(TDep) string
	client             types.Client
	tools              []Tool[TDep]
	maxIterations      int
	responseFormatMode types.ResponseFormatMode
}

type Option[TDep, TOut any] func(*Agent[TDep, TOut])

func New[TDep, TOut any](client types.Client, opts ...Option[TDep, TOut]) *Agent[TDep, TOut] {
	a := &Agent[TDep, TOut]{
		client:        client,
		maxIterations: 10,
	}

	for _, opt := range opts {
		opt(a)
	}

	return a
}

func WithSystemPrompt[TDep, TOut any](prompt string) Option[TDep, TOut] {
	return func(a *Agent[TDep, TOut]) {
		a.systemPrompt = prompt
	}
}

func WithSystemPromptFunc[TDep, TOut any](fn func(TDep) string) Option[TDep, TOut] {
	return func(a *Agent[TDep, TOut]) {
		a.systemPromptFunc = fn
	}
}

func WithTools[TDep, TOut any](tools ...Tool[TDep]) Option[TDep, TOut] {
	return func(a *Agent[TDep, TOut]) {
		a.tools = tools
	}
}

func WithResponseFormat[TDep, TOut any](mode types.ResponseFormatMode) Option[TDep, TOut] {
	return func(a *Agent[TDep, TOut]) {
		a.responseFormatMode = mode
	}
}

type runConfig struct {
	prompt   string
	messages []types.Message
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

	toolDefs := GetToolDefinitions[TDep](a.tools)

	// Initialize RunContext with starting messages + user prompt
	rc := &RunContext[TDep]{
		Deps:     dep,
		Messages: runCfg.messages,
	}
	if runCfg.prompt != "" {
		rc.Messages = append(rc.Messages, types.NewUserMessage(types.WithText(runCfg.prompt)))
	}

	for i := 0; i < a.maxIterations; i++ {
		resp, err := a.client.Chat(ctx, &types.ChatParams{
			Messages:       rc.Messages,
			SystemPrompt:   systemPrompt,
			Tools:          toolDefs,
			ResponseFormat: rf,
		})
		if err != nil {
			return nil, err
		}

		if len(resp.Choices) == 0 || resp.Choices[0].Message == nil {
			return nil, fmt.Errorf("no response from model")
		}
		choice := &resp.Choices[0]
		msg := choice.Message

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
					return nil, fmt.Errorf("failed to unmarshal output: %w", err)
				}
			} else if rf.Schema != nil {
				return nil, fmt.Errorf("expected structured output but got none")
			}
			return &RunResult[TOut]{
				Output:   res,
				Messages: rc.Messages,
				Usage:    rc.Usage,
			}, nil
		}

		// Case 2: Has tool calls - execute them
		for _, tc := range msg.ToolCalls {
			tool := a.findTool(tc.Function.Name)
			if tool == nil {
				return nil, fmt.Errorf("unknown tool: %s", tc.Function.Name)
			}

			result, err := tool.Execute(ctx, rc, tc.Function.Arguments)
			if err != nil {
				return nil, fmt.Errorf("tool execution failed: %w", err)
			}

			rc.Messages = append(rc.Messages, types.NewToolResultMessage(tc.ID, result))
		}
	}

	return nil, fmt.Errorf("agent exceeded max iterations (%d)", a.maxIterations)
}

func (a *Agent[TDep, TOut]) findTool(name string) *Tool[TDep] {
	for i := range a.tools {
		if a.tools[i].Name == name {
			return &a.tools[i]
		}
	}
	return nil
}
