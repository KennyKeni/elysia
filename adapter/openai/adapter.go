package openai

import (
	"context"
	"net/http"

	"github.com/KennyKeni/elysia/types"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// Client wraps the OpenAI SDK client and implements the unified chat interface
type Client struct {
	client openai.Client
}

// NewClient creates a new OpenAI adapter client with options
func NewClient(opts ...option.RequestOption) *Client {
	return &Client{
		client: openai.NewClient(opts...),
	}
}

// Convenience option constructors

// WithHTTPClient configures a custom HTTP client
func WithHTTPClient(client *http.Client) option.RequestOption {
	return option.WithHTTPClient(client)
}

// WithAPIKey configures the API key for authentication
func WithAPIKey(apiKey string) option.RequestOption {
	return option.WithAPIKey(apiKey)
}

// WithBaseURL configures a custom base URL
func WithBaseURL(baseURL string) option.RequestOption {
	return option.WithBaseURL(baseURL)
}

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

	// Convert OpenAI response to unified response
	return ToChatResponse(completion), nil
}
