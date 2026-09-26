// Package tools holds MCP tool definitions shared across deployables.
package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HelloGreeting is the payload returned by the hello_world tool.
// Keeping the output type exported lets other packages reuse it.
type HelloGreeting struct {
	Message string `json:"message" jsonschema:"the greeting text"`
}

// HelloHandler returns a handler producing a hello world greeting.
// The SDK infers the tool's input schema from the (empty) args struct.
func HelloHandler() mcp.ToolHandlerFor[struct{}, HelloGreeting] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, HelloGreeting, error) {
		return nil, HelloGreeting{Message: "Hello, World!"}, nil
	}
}