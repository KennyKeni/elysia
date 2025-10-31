package types

import (
	"context"
	"encoding/json"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ToolHandler func(ctx context.Context, args map[string]any) (result any, err error)

type Tool interface {
	Name() string
	Description() string
	InputSchema() any
	OutputSchema() any
	Execute(ctx context.Context, args map[string]any) (result any, err error)
}

type NativeTool struct {
	name         string
	description  string
	inputSchema  any
	outputSchema any
	handler      ToolHandler
}

func (n *NativeTool) Name() string {
	return n.name
}

func (n *NativeTool) Description() string {
	return n.description
}

func (n *NativeTool) InputSchema() any {
	return n.inputSchema
}

func (n *NativeTool) OutputSchema() any {
	return n.outputSchema
}

func (n *NativeTool) Execute(ctx context.Context, args map[string]any) (result any, err error) {
	return n.handler(ctx, args)
}

type MCPToolAdapter struct {
	mcpTool mcp.Tool
	session *mcp.ClientSession
}

func (m *MCPToolAdapter) Name() string {
	return m.mcpTool.Name
}

func (m *MCPToolAdapter) Description() string {
	return m.mcpTool.Description
}

func (m *MCPToolAdapter) InputSchema() any {
	return m.mcpTool.InputSchema
}

func (m *MCPToolAdapter) OutputSchema() any {
	return m.mcpTool.OutputSchema
}

func (m *MCPToolAdapter) Execute(ctx context.Context, args map[string]any) (result any, err error) {
	result, err = m.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      m.mcpTool.Name,
		Arguments: args,
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// NewNativeTool creates a new native tool with automatic schema generation from Go types
func NewNativeTool[TIn, TOut any](
	name, description string,
	handler func(ctx context.Context, args TIn) (TOut, error),
) (*NativeTool, error) {
	// Generate input schema using jsonschema.For
	inputSchema, err := jsonschema.For[TIn](nil)
	if err != nil {
		return nil, err
	}

	// Generate output schema using jsonschema.For
	outputSchema, err := jsonschema.For[TOut](nil)
	if err != nil {
		return nil, err
	}

	// Resolve schemas for validation
	resolvedInput, err := inputSchema.Resolve(nil)
	if err != nil {
		return nil, err
	}
	resolvedOutput, err := outputSchema.Resolve(nil)
	if err != nil {
		return nil, err
	}

	// Wrap the typed handler with validation
	wrappedHandler := func(ctx context.Context, args map[string]any) (any, error) {
		// Validate input against schema
		if err := resolvedInput.Validate(args); err != nil {
			return nil, err
		}

		// Convert map to typed input
		argsJSON, err := json.Marshal(args)
		if err != nil {
			return nil, err
		}
		var typedArgs TIn
		if err := json.Unmarshal(argsJSON, &typedArgs); err != nil {
			return nil, err
		}

		// Execute handler
		result, err := handler(ctx, typedArgs)
		if err != nil {
			return nil, err
		}

		// Validate output against schema
		resultJSON, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}
		var resultMap map[string]any
		if err := json.Unmarshal(resultJSON, &resultMap); err != nil {
			return nil, err
		}
		if err := resolvedOutput.Validate(resultMap); err != nil {
			return nil, err
		}

		return result, nil
	}

	return &NativeTool{
		name:         name,
		description:  description,
		inputSchema:  inputSchema,
		outputSchema: outputSchema,
		handler:      wrappedHandler,
	}, nil
}

// NewMCPToolAdapter creates a new MCP tool adapter
func NewMCPToolAdapter(mcpTool mcp.Tool, session *mcp.ClientSession) *MCPToolAdapter {
	return &MCPToolAdapter{
		mcpTool: mcpTool,
		session: session,
	}
}
