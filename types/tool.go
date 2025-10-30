package types

import (
	"context"
	"encoding/json"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ToolHandler func(ctx context.Context, argsJSON []byte) (resultJSON []byte, err error)

type Tool interface {
	Name() string
	Description() string
	InputSchema() json.RawMessage
	OutputSchema() json.RawMessage
	Execute(ctx context.Context, argsJSON []byte) (resultJSON []byte, err error)
}

type NativeTool struct {
	name         string
	description  string
	inputSchema  json.RawMessage
	outputSchema json.RawMessage
	handler      ToolHandler
}

func (n *NativeTool) Name() string {
	return n.name
}

func (n *NativeTool) Description() string {
	return n.description
}

func (n *NativeTool) InputSchema() json.RawMessage {
	return n.inputSchema
}

func (n *NativeTool) OutputSchema() json.RawMessage {
	return n.outputSchema
}

func (n *NativeTool) Execute(ctx context.Context, argsJSON []byte) (resultJSON []byte, err error) {
	return n.handler(ctx, argsJSON)
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

func (m *MCPToolAdapter) InputSchema() json.RawMessage {
	if m.mcpTool.InputSchema == nil {
		return nil
	}
	data, _ := json.Marshal(m.mcpTool.InputSchema)
	return data
}

func (m *MCPToolAdapter) OutputSchema() json.RawMessage {
	if m.mcpTool.OutputSchema == nil {
		return nil
	}
	data, _ := json.Marshal(m.mcpTool.OutputSchema)
	return data
}

func (m *MCPToolAdapter) Execute(ctx context.Context, argsJSON []byte) (resultJSON []byte, err error) {
	var args map[string]any
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return nil, err
	}

	result, err := m.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      m.mcpTool.Name,
		Arguments: args,
	})
	if err != nil {
		return nil, err
	}

	return json.Marshal(result)
}

// NewNativeTool creates a new native tool with raw JSON schemas
func NewNativeTool(name, description string, inputSchema, outputSchema json.RawMessage, handler ToolHandler) *NativeTool {
	return &NativeTool{
		name:         name,
		description:  description,
		inputSchema:  inputSchema,
		outputSchema: outputSchema,
		handler:      handler,
	}
}

// NewTypedNativeTool creates a new native tool with automatic schema generation from Go types
func NewTypedNativeTool[TIn, TOut any](
	name, description string,
	handler func(ctx context.Context, args TIn) (TOut, error),
) (*NativeTool, error) {
	// Generate input schema using jsonschema.For
	inputSchema, err := jsonschema.For[TIn](nil)
	if err != nil {
		return nil, err
	}
	inputJSON, _ := json.Marshal(inputSchema)

	// Generate output schema using jsonschema.For
	outputSchema, err := jsonschema.For[TOut](nil)
	if err != nil {
		return nil, err
	}
	outputJSON, _ := json.Marshal(outputSchema)

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
	wrappedHandler := func(ctx context.Context, argsJSON []byte) ([]byte, error) {
		// Validate input against schema
		var inputMap map[string]any
		if err := json.Unmarshal(argsJSON, &inputMap); err != nil {
			return nil, err
		}
		if err := resolvedInput.Validate(inputMap); err != nil {
			return nil, err
		}

		// Unmarshal to typed input
		var args TIn
		if err := json.Unmarshal(argsJSON, &args); err != nil {
			return nil, err
		}

		// Execute handler
		result, err := handler(ctx, args)
		if err != nil {
			return nil, err
		}

		// Marshal result
		resultJSON, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}

		// Validate output against schema
		var outputMap map[string]any
		if err := json.Unmarshal(resultJSON, &outputMap); err != nil {
			return nil, err
		}
		if err := resolvedOutput.Validate(outputMap); err != nil {
			return nil, err
		}

		return resultJSON, nil
	}

	return &NativeTool{
		name:         name,
		description:  description,
		inputSchema:  inputJSON,
		outputSchema: outputJSON,
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
