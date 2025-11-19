package openai

import (
	"context"
	"net/http"

	"github.com/KennyKeni/elysia/client"
	"github.com/KennyKeni/elysia/types"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// Client wraps the OpenAI SDK client and implements the unified chat interface
type Client struct {
	client openai.Client
}

// NewClient creates a new OpenAI adapter client with options
func NewClient(opts ...client.Option) *Client {
	cfg := client.DefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	openaiOpts := translateConfig(cfg)

	return &Client{
		client: openai.NewClient(openaiOpts...),
	}
}

// NewClientFromOpenAI creates a new OpenAI adapter with an existing OpenAI client
func NewClientFromOpenAI(client openai.Client) *Client {
	return &Client{client: client}
}

func translateConfig(cfg client.Config) []option.RequestOption {
	var opts []option.RequestOption

	// API Key
	if cfg.APIKey != "" {
		opts = append(opts, option.WithAPIKey(cfg.APIKey))
	}

	// Base URL
	if cfg.BaseURL != nil {
		opts = append(opts, option.WithBaseURL(*cfg.BaseURL))
	}

	// Retry maximum
	if cfg.MaxRetries > 0 {
		opts = append(opts, option.WithMaxRetries(cfg.MaxRetries))
	}

	// Timeout for each attempt
	if cfg.PerAttemptTimeout > 0 {
		opts = append(opts, option.WithRequestTimeout(cfg.PerAttemptTimeout))
	}

	// Http Client, only used if it isn't nil
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{}
	}

	// Total timeout is set on HTTP client
	if cfg.TotalTimeout > 0 {
		httpClient.Timeout = cfg.TotalTimeout
	}

	// Set HTTP Client
	opts = append(opts, option.WithHTTPClient(httpClient))

	if cfg.Headers != nil {
		for key, values := range cfg.Headers {
			for _, value := range values {
				opts = append(opts, option.WithHeader(key, value))
			}
		}
	}

	return opts
}

// Potentially add per-request options

// Chat performs a non-streaming chat completion request
func (c *Client) Chat(ctx context.Context, params *types.ChatParams) (*types.ChatResponse, error) {
	// Convert unified params to OpenAI params
	openaiParams, err := ToChatCompletionParams(params)
	if err != nil {
		return nil, err
	}

	// Call OpenAI SDK
	completion, err := c.client.Chat.Completions.New(ctx, openaiParams)
	if err != nil {
		return nil, err
	}

	if err := validateChatCompletion(completion); err != nil {
		return nil, err
	}

	// Convert OpenAI response to unified response
	return FromChatCompletion(completion), nil
}

// ChatStream performs a streaming chat completion request and returns an iterator over chunks.
func (c *Client) ChatStream(ctx context.Context, params *types.ChatParams) (*types.Stream, error) {
	openaiParams, err := ToChatCompletionParams(params)
	if err != nil {
		return nil, err
	}

	stream := c.client.Chat.Completions.NewStreaming(ctx, openaiParams)
	return newChatStream(stream), nil
}

// Embed performs an embedding request
func (c *Client) Embed(ctx context.Context, params *types.EmbeddingParams) (*types.EmbeddingResponse, error) {
	// Convert unified params to OpenAI params
	openaiParams, err := ToEmbeddingParams(params)
	if err != nil {
		return nil, err
	}

	// Call OpenAI SDK
	embedding, err := c.client.Embeddings.New(ctx, openaiParams)
	if err != nil {
		return nil, err
	}

	// Convert OpenAI response to unified response
	return FromCreateEmbeddingResponse(embedding), nil
}
