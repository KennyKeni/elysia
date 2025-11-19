package client

import (
	"net/http"
	"time"
)

// Config holds provider-agnostic client configuration
type Config struct {
	APIKey            string
	BaseURL           *string
	HTTPClient        *http.Client
	MaxRetries        int
	PerAttemptTimeout time.Duration
	TotalTimeout      time.Duration
	Headers           http.Header
}

// DefaultConfig returns config with sensible defaults
func DefaultConfig() Config {
	return Config{
		MaxRetries:        2,
		PerAttemptTimeout: 30 * time.Second,
		TotalTimeout:      2 * time.Minute,
		// Headers are nil by default - no custom headers
	}
}

// Option is a functional option for configuring a client
type Option func(*Config)

// WithAPIKey sets the API key
func WithAPIKey(apiKey string) Option {
	return func(c *Config) {
		c.APIKey = apiKey
	}
}

// WithBaseURL sets a custom base URL
func WithBaseURL(baseURL string) Option {
	return func(c *Config) {
		c.BaseURL = &baseURL
	}
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(client *http.Client) Option {
	return func(c *Config) {
		c.HTTPClient = client
	}
}

// WithMaxRetries sets the maximum number of retry attempts
func WithMaxRetries(maxRetries int) Option {
	return func(c *Config) {
		c.MaxRetries = maxRetries
	}
}

// WithPerAttemptTimeout sets the timeout for each retry attempt
func WithPerAttemptTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.PerAttemptTimeout = timeout
	}
}

// WithTotalTimeout sets the total timeout including all retries
func WithTotalTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.TotalTimeout = timeout
	}
}

// WithHeader adds a single custom header
func WithHeader(key, value string) Option {
	return func(c *Config) {
		if c.Headers == nil {
			c.Headers = make(http.Header)
		}
		c.Headers.Add(key, value)
	}
}

// WithHeaders replaces all headers with the provided set
func WithHeaders(headers http.Header) Option {
	return func(c *Config) {
		c.Headers = headers
	}
}
