package activity

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"

	"github.com/leokiin/mcp-conveer/internal/agent"
)

// Registry holds named MCP servers so Activities can call them by name.
// Register must be called before worker.Start; after that the map is read-only.
type Registry struct {
	mu      sync.RWMutex
	servers map[string]*agent.MCPServer
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{servers: make(map[string]*agent.MCPServer)}
}

// Register adds an MCP server under a name.
func (r *Registry) Register(name string, srv *agent.MCPServer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.servers[name] = srv
}

// CallInput is the parameter type for the CallAgent activity.
type CallInput struct {
	AgentName string                     `json:"agent_name"`
	Task      string                     `json:"task"`
	Context   map[string]json.RawMessage `json:"context,omitempty"` // results from depends_on steps
	// InputStep, if set, uses only Context[InputStep] as the task instead of the
	// full enriched context string. Allows mcp-client steps to receive a clean,
	// short input (e.g. a library name) rather than the entire accumulated context.
	InputStep  string `json:"input_step,omitempty"`
	// InputField, used with InputStep, extracts a single string field from the step JSON.
	// Example: InputStep="extract-lib", InputField="library" → passes "gin" from {"library":"gin"}.
	InputField string `json:"input_field,omitempty"`
}

// CallAgent is a Temporal Activity that invokes a named agent and returns its JSON result.
// It is a method on Registry so the worker can register it without a closure.
func (r *Registry) CallAgent(ctx context.Context, input CallInput) (json.RawMessage, error) {
	r.mu.RLock()
	srv, ok := r.servers[input.AgentName]
	r.mu.RUnlock()
	if !ok {
		return nil, &UnknownAgentError{input.AgentName}
	}
	task := buildTaskWithContext(input.Task, input.Context)
	if input.InputStep != "" {
		if raw, ok := input.Context[input.InputStep]; ok {
			task = extractField(raw, input.InputField)
		}
	}
	return srv.Call(ctx, task)
}

// buildTaskWithContext appends prior step results to the task so agents have full context.
func buildTaskWithContext(task string, priorResults map[string]json.RawMessage) string {
	if len(priorResults) == 0 {
		return task
	}
	keys := make([]string, 0, len(priorResults))
	for k := range priorResults {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	b := new(strings.Builder)
	b.WriteString(task)
	b.WriteString("\n\n--- Results from previous steps ---\n")
	for _, step := range keys {
		b.WriteString("\n[")
		b.WriteString(step)
		b.WriteString("]\n")
		b.Write(priorResults[step])
		b.WriteByte('\n')
	}
	return b.String()
}

// extractField returns the string value of field from a JSON object,
// or the raw JSON string if field is empty or not found.
func extractField(raw json.RawMessage, field string) string {
	if field != "" {
		var obj map[string]interface{}
		if err := json.Unmarshal(raw, &obj); err == nil {
			if val, ok := obj[field]; ok {
				if s, ok := val.(string); ok {
					return s
				}
				// field exists but is not a string (number, bool, object) — fall through to raw JSON
			}
		}
	}
	// empty field or parse failure: pass the raw JSON blob to the next agent
	return string(raw)
}

// UnknownAgentError is returned by CallAgent when no agent with the given name is registered.
type UnknownAgentError struct{ Name string }

func (e *UnknownAgentError) Error() string { return "unknown agent: " + e.Name }

// Is enables errors.Is matching: a zero-Name target matches any UnknownAgentError.
func (e *UnknownAgentError) Is(target error) bool {
	t, ok := target.(*UnknownAgentError)
	return ok && (t.Name == "" || t.Name == e.Name)
}
