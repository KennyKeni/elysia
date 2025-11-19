package mcp

import (
	"context"
	"encoding/base64"

	"github.com/KennyKeni/elysia/types"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ToolAdapter struct {
	mcpTool mcp.Tool
	session *mcp.ClientSession
}

func (m *ToolAdapter) Name() string {
	return m.mcpTool.Name
}

func (m *ToolAdapter) Description() string {
	return m.mcpTool.Description
}

func (m *ToolAdapter) InputSchema() any {
	return m.mcpTool.InputSchema
}

func (m *ToolAdapter) OutputSchema() any {
	return m.mcpTool.OutputSchema
}

func (m *ToolAdapter) Execute(ctx context.Context, args map[string]any) (*types.ToolResult, error) {
	callResult, err := m.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      m.mcpTool.Name,
		Arguments: args,
	})
	if err != nil {
		return nil, err
	}

	// Convert MCP CallToolResult to types.ToolResult
	result := &types.ToolResult{
		ContentPart:       make([]types.ContentPart, 0, len(callResult.Content)),
		StructuredContent: callResult.StructuredContent,
		IsError:           callResult.IsError,
	}

	// Convert MCP Content to types.ContentPart
	for _, content := range callResult.Content {
		switch c := content.(type) {
		case *mcp.TextContent:
			result.ContentPart = append(result.ContentPart, &types.ContentPartText{Text: c.Text})
		case *mcp.ImageContent:
			// Convert []byte to base64 string
			imageData := base64.StdEncoding.EncodeToString(c.Data)
			result.ContentPart = append(result.ContentPart, &types.ContentPartImage{Data: imageData})
			// Add more content type conversions as needed
		}
	}

	return result, nil
}

// NewToolAdapter creates a new MCP tool adapter
func NewToolAdapter(mcpTool mcp.Tool, session *mcp.ClientSession) *ToolAdapter {
	return &ToolAdapter{
		mcpTool: mcpTool,
		session: session,
	}
}
