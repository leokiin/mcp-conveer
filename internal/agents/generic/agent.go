package generic

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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
	systemPrompt := "today: " + time.Now().Format("2006-01-02") + "\n\n" + a.role
	raw, usage, err := a.llm.Complete(ctx, systemPrompt, task)
	if err != nil {
		return nil, fmt.Errorf("agent %s: %w", a.name, err)
	}

	if json.Valid([]byte(raw)) {
		// Merge _tokens into the existing JSON object.
		var obj map[string]json.RawMessage
		if err := json.Unmarshal([]byte(raw), &obj); err == nil {
			tokBytes, _ := json.Marshal(usage)
			obj["_tokens"] = tokBytes
			return json.Marshal(obj)
		}
	}

	result := map[string]interface{}{"response": raw, "_tokens": usage}
	return json.Marshal(result)
}
