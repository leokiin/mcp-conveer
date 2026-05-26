package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// MCPServer wraps an Agent as a Model Context Protocol server.
// The agent's Handle method is exposed as a single MCP tool named after the agent.
type MCPServer struct {
	agent  Agent
	server *server.MCPServer
}

// NewMCPServer creates an MCPServer for the given agent.
func NewMCPServer(a Agent) *MCPServer {
	s := server.NewMCPServer(a.Name(), "1.0.0")

	ms := &MCPServer{agent: a, server: s}

	tool := mcp.NewTool(a.Name(),
		mcp.WithDescription(fmt.Sprintf("Run the %s agent on the given task", a.Name())),
		mcp.WithString("task",
			mcp.Required(),
			mcp.Description("Task description for the agent to process"),
		),
	)

	s.AddTool(tool, ms.handleTool)
	return ms
}

func (ms *MCPServer) handleTool(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	task, err := req.RequireString("task")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result, err := ms.agent.Handle(ctx, task)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(string(result)), nil
}

// ServeStdio starts the MCP server on stdin/stdout (default transport for local agents).
func (ms *MCPServer) ServeStdio() error {
	return server.ServeStdio(ms.server)
}

// Call invokes the agent directly without going through MCP transport.
// Used by Temporal Activities to call agents in the same process.
func (ms *MCPServer) Call(ctx context.Context, task string) (json.RawMessage, error) {
	return ms.agent.Handle(ctx, task)
}
