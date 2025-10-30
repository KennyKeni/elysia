package types

import "context"

type Client interface {
	Chat(ctx context.Context, params ChatParams) (ChatResponse, error)
	// ChatStream(ctx context.Context, params *types.ChatParams) (*StreamResponse, error)
}
