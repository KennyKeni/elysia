package mcp

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/KennyKeni/elysia/types"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewTool creates a types.Tool from an MCP tool definition and session.
// From the client, InputSchema is map[string]any after JSON unmarshaling.
func NewTool(mcpTool mcp.Tool, session *mcp.ClientSession) (*types.Tool, error) {
	var inputSchema map[string]any
	if mcpTool.InputSchema != nil {
		var ok bool
		inputSchema, ok = mcpTool.InputSchema.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("unexpected InputSchema type: %T", mcpTool.InputSchema)
		}
	}

	return &types.Tool{
		ToolDefinition: types.ToolDefinition{
			Name:        mcpTool.Name,
			Description: mcpTool.Description,
			InputSchema: inputSchema,
		},
		Execute: func(ctx context.Context, args map[string]any) (*types.ToolResult, error) {
			callResult, err := session.CallTool(ctx, &mcp.CallToolParams{
				Name:      mcpTool.Name,
				Arguments: args,
			})
			if err != nil {
				return &types.ToolResult{
					ContentPart: []types.ContentPart{
						types.NewContentPartText(fmt.Sprintf("MCP call error: %v", err)),
					},
					IsError: true,
				}, nil
			}

			return convertResult(callResult), nil
		},
	}, nil
}

// convertResult converts an MCP CallToolResult to types.ToolResult
func convertResult(callResult *mcp.CallToolResult) *types.ToolResult {
	result := &types.ToolResult{
		ContentPart:       make([]types.ContentPart, 0, len(callResult.Content)),
		StructuredContent: callResult.StructuredContent,
		IsError:           callResult.IsError,
	}

	for _, content := range callResult.Content {
		switch c := content.(type) {
		case *mcp.TextContent:
			result.ContentPart = append(result.ContentPart, types.NewContentPartText(c.Text))
		case *mcp.ImageContent:
			imageData := base64.StdEncoding.EncodeToString(c.Data)
			result.ContentPart = append(result.ContentPart, &types.ContentPartImage{Data: imageData})
		}
	}

	return result
}
