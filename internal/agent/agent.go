package agent

import (
	"context"
	"encoding/json"
)

// Agent is a specialized worker that processes a task and returns a structured JSON result.
// Each agent encapsulates its own LLM model, prompts, and access permissions.
type Agent interface {
	// Name returns the unique identifier for this agent, used as the MCP tool name.
	Name() string
	// Handle processes the task and returns a JSON-encoded result.
	Handle(ctx context.Context, task string) (json.RawMessage, error)
}
