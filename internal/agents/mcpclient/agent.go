package mcpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// Agent calls an external MCP server process and invokes a single tool.
// It is a pipeline step: the task string is passed as an argument to the tool,
// and the tool's text output is returned as JSON for downstream steps.
type Agent struct {
	name      string
	command   string            // e.g. "npx -y @upstash/context7-mcp@latest"
	toolName  string            // MCP tool to invoke
	inputKey  string            // argument key for the task (default: "query")
	extraArgs map[string]string // additional fixed arguments merged into every call
}

// New creates an MCP client agent.
// command is the full shell command to launch the MCP server (split on spaces).
// toolName is the MCP tool to call. inputKey is the argument key (default "query").
// extraArgs are additional fixed arguments passed alongside the main input.
func New(name, command, toolName, inputKey string, extraArgs map[string]string) *Agent {
	if inputKey == "" {
		inputKey = "query"
	}
	return &Agent{name: name, command: command, toolName: toolName, inputKey: inputKey, extraArgs: extraArgs}
}

func (a *Agent) Name() string { return a.name }

func (a *Agent) Handle(ctx context.Context, task string) (json.RawMessage, error) {
	parts, err := splitCommand(a.command)
	if err != nil {
		return nil, fmt.Errorf("mcpclient %s: %w", a.name, err)
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("mcpclient %s: empty command", a.name)
	}

	c, err := client.NewStdioMCPClient(parts[0], nil, parts[1:]...)
	if err != nil {
		return nil, fmt.Errorf("mcpclient %s: start process: %w", a.name, err)
	}
	defer c.Close()

	if _, err := c.Initialize(ctx, mcp.InitializeRequest{}); err != nil {
		return nil, fmt.Errorf("mcpclient %s: initialize: %w", a.name, err)
	}

	req := mcp.CallToolRequest{}
	req.Params.Name = a.toolName
	args := make(map[string]any, 1+len(a.extraArgs))
	args[a.inputKey] = task
	for k, v := range a.extraArgs {
		args[k] = v
	}
	req.Params.Arguments = args

	result, err := c.CallTool(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("mcpclient %s: call tool %s: %w", a.name, a.toolName, err)
	}
	if result.IsError {
		return nil, fmt.Errorf("mcpclient %s: tool %s error: %s", a.name, a.toolName, extractText(result))
	}

	text := extractText(result)
	if json.Valid([]byte(text)) {
		return json.RawMessage(text), nil
	}
	out, err := json.Marshal(map[string]string{"response": text})
	if err != nil {
		return nil, fmt.Errorf("mcpclient %s: marshal response: %w", a.name, err)
	}
	return out, nil
}

// splitCommand splits a shell-like command string into tokens,
// honoring single and double-quoted groups so spaces inside quotes are preserved.
func splitCommand(cmd string) ([]string, error) {
	var tokens []string
	var current strings.Builder
	inSingle := false
	inDouble := false

	for i := 0; i < len(cmd); i++ {
		ch := cmd[i]
		switch {
		case ch == '\'' && !inDouble:
			inSingle = !inSingle
		case ch == '"' && !inSingle:
			inDouble = !inDouble
		case (ch == ' ' || ch == '\t') && !inSingle && !inDouble:
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}
	if inSingle || inDouble {
		return nil, fmt.Errorf("unterminated quote in command: %s", cmd)
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens, nil
}

func extractText(result *mcp.CallToolResult) string {
	var parts []string
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n")
}
