package generic

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/leokiin/mcp-conveer/internal/provider"
)

// Agent is a configurable LLM agent driven by a system prompt (role).
// It wraps any LLM provider and returns the response as a JSON string.
type Agent struct {
	name string
	role string
	llm  provider.LLMProvider
}

// New creates a generic agent with the given name, system role, and LLM provider.
func New(name, role string, llm provider.LLMProvider) *Agent {
	return &Agent{name: name, role: role, llm: llm}
}

func (a *Agent) Name() string { return a.name }

func (a *Agent) Handle(ctx context.Context, task string) (json.RawMessage, error) {
	raw, err := a.llm.Complete(ctx, a.role, task)
	if err != nil {
		return nil, fmt.Errorf("agent %s: %w", a.name, err)
	}

	// If the response is already valid JSON, return it as-is.
	if json.Valid([]byte(raw)) {
		return json.RawMessage(raw), nil
	}

	// Otherwise wrap in a JSON object so downstream steps can consume it.
	result := map[string]string{"response": raw}
	return json.Marshal(result)
}
