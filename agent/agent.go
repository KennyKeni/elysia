package agent

import "github.com/KennyKeni/elysia/types"

type Agent[TDeps, TOutput any] struct {
	systemPrompt     string
	systemPromptFunc func(TDeps) string
	client           types.Client
	tools            []*Tool[TDeps]
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
func WithTools[TDeps, TOutput any](tools ...*Tool[TDeps]) Option[TDeps, TOutput] {
	return func(a *Agent[TDeps, TOutput]) {
		a.tools = tools
	}
}
