package agent

import (
	"context"
	"fmt"

	"github.com/KennyKeni/elysia/types"
)

type Agent[TDeps, TOutput any] struct {
	systemPrompt     string
	systemPromptFunc func(TDeps) string
	client           types.Client
	tools            []Tool[TDeps]
	maxIterations    int
}

type Option[TDeps, TOutput any] func(*Agent[TDeps, TOutput])

func New[TDeps, TOutput any](client types.Client, opts ...Option[TDeps, TOutput]) *Agent[TDeps, TOutput] {
	a := &Agent[TDeps, TOutput]{
		client:        client,
		maxIterations: 10,
	}

	for _, opt := range opts {
		opt(a)
	}

	return a
}

func WithSystemPrompt[TDeps, TOutput any](prompt string) Option[TDeps, TOutput] {
	return func(a *Agent[TDeps, TOutput]) {
		a.systemPrompt = prompt
	}
}

func WithSystemPromptFunc[TDeps, TOutput any](fn func(TDeps) string) Option[TDeps, TOutput] {
	return func(a *Agent[TDeps, TOutput]) {
		a.systemPromptFunc = fn
	}
}

func WithTools[TDeps, TOutput any](tools ...Tool[TDeps]) Option[TDeps, TOutput] {
	return func(a *Agent[TDeps, TOutput]) {
		a.tools = tools
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

func (a *Agent[TDeps, TOutput]) Run(ctx context.Context, deps TDeps, opts ...RunOption) (TOutput, error) {
	var res TOutput

	runCfg := runConfig{}
	for _, opt := range opts {
		opt(&runCfg)
	}

	prompt := runCfg.prompt
	messages := runCfg.messages

	messages = append(messages, types.NewUserMessage(types.WithText(prompt)))

	var systemPrompt string
	if a.systemPromptFunc != nil {
		systemPrompt = a.systemPromptFunc(deps)
	} else {
		systemPrompt = a.systemPrompt
	}

	toolDefs := GetToolDefinitions[TDeps](a.tools)

	rc := &RunContext[TDeps]{
		Deps:     deps,
		Messages: messages,
	}

	for i := 0; i < a.maxIterations; i++ {
		resp, err := a.client.Chat(ctx, &types.ChatParams{
			Messages:     messages,
			SystemPrompt: systemPrompt,
			Tools:        toolDefs,
		})

		if err != nil {
			return res, err
		}

		if len(resp.Choices) == 0 || resp.Choices[0].Message == nil {
			return res, fmt.Errorf("no response from model")
		}
		msg := resp.Choices[0].Message

		if resp.Usage != nil {
			rc.Usage.PromptTokens += resp.Usage.PromptTokens
			rc.Usage.CompletionTokens += resp.Usage.CompletionTokens
			rc.Usage.TotalTokens += resp.Usage.TotalTokens
		}

		rc.Messages = append(rc.Messages, *msg)

		// TODO Standardize usage
		//if len(msg.ToolCalls) == 0 {
		//	return a.parseOutput(msg)
		//}
	}

	return res, fmt.Errorf("agent exceeded max iterations (%d)", a.maxIterations)
}
