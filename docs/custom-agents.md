# Writing Custom Agents

An agent is any Go struct that implements the `Agent` interface. Once implemented, it can be:

- Registered in the Activity Registry and used in a Temporal pipeline
- Wrapped as a standalone MCP server and used from Claude Code or any MCP client

## The Agent Interface

```go
// internal/agent/agent.go
type Agent interface {
    Name() string
    Handle(ctx context.Context, task string) (json.RawMessage, error)
}
```

**`Name()`** returns the agent's identifier. This name is used as:
- The MCP tool name when wrapped in an `MCPServer`
- The key in the Activity Registry (`registry.Register(name, srv)`)
- The `agent:` value in `conveer.yaml`

**`Handle()`** receives the enriched task string and must return valid JSON. If the agent is used in a pipeline with `depends_on`, the task will include outputs from prior steps (see [Context Chaining](../README.md#context-chaining)).

## Minimal Example

```go
package myagent

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/dinar/mcp-conveer/internal/provider"
)

type Agent struct {
    llm provider.LLMProvider
}

func New(llm provider.LLMProvider) *Agent {
    return &Agent{llm: llm}
}

func (a *Agent) Name() string { return "my-agent" }

func (a *Agent) Handle(ctx context.Context, task string) (json.RawMessage, error) {
    const systemPrompt = `You are an expert reviewer. Analyze the task and return JSON:
{"finding": "...", "severity": "low|medium|high", "recommendation": "..."}`

    raw, err := a.llm.Complete(ctx, systemPrompt, task)
    if err != nil {
        return nil, fmt.Errorf("my-agent: %w", err)
    }

    // Validate the output is proper JSON before returning.
    if !json.Valid([]byte(raw)) {
        return nil, fmt.Errorf("my-agent: LLM returned non-JSON: %s", raw)
    }
    return json.RawMessage(raw), nil
}
```

## Registering in the Worker

Add your agent to `cmd/worker/main.go`:

```go
import "github.com/dinar/mcp-conveer/internal/agents/myagent"

// Inside main():
registry.Register("my-agent", agent.NewMCPServer(myagent.New(newProvider("deepseek/deepseek-chat"))))
```

Or load it from config by defining it in `conveer.yaml`:

```yaml
agents:
  - name: my-agent
    model: deepseek/deepseek-chat
    role: "Analyze the task and return JSON with finding, severity, and recommendation."
```

Config-based agents use the `generic.Agent` which wraps any `LLMProvider` with a system prompt. For agents that need custom logic (file I/O, API calls, non-LLM processing), implement the interface directly.

## Output Format

Agents should always return valid JSON. The generic agent handles non-JSON responses by wrapping them:

```go
// generic/agent.go
if json.Valid([]byte(raw)) {
    return json.RawMessage(raw), nil
}
// Fallback: wrap plain text
result := map[string]string{"response": raw}
return json.Marshal(result)
```

For custom agents, validate and marshal explicitly:

```go
type Result struct {
    Finding        string `json:"finding"`
    Severity       string `json:"severity"`
    Recommendation string `json:"recommendation"`
}

var result Result
if err := json.Unmarshal([]byte(raw), &result); err != nil {
    return nil, fmt.Errorf("parse LLM response: %w\nresponse: %s", err, raw)
}
return json.Marshal(result)
```

Define a concrete output struct and validate it. This catches LLM hallucinations early (at the agent boundary) rather than letting them propagate as context to downstream agents.

## Non-LLM Agents

Agents don't have to call an LLM. Any Go code that implements the interface works:

```go
// A code metrics agent that runs go vet and returns findings as JSON.
type GoVetAgent struct{}

func (a *GoVetAgent) Name() string { return "go-vet" }

func (a *GoVetAgent) Handle(ctx context.Context, task string) (json.RawMessage, error) {
    out, err := exec.CommandContext(ctx, "go", "vet", task).CombinedOutput()
    result := map[string]string{
        "output": string(out),
        "error":  "",
    }
    if err != nil {
        result["error"] = err.Error()
    }
    return json.Marshal(result)
}
```

An agent can also call an external API, query a database, or search a vector store — anything that can produce a JSON result.

## Standalone MCP Server

Any agent can be deployed as a standalone MCP server (separate binary) using `ServeStdio`:

```go
// agents/my-agent/main.go
package main

import (
    "log"

    "github.com/dinar/mcp-conveer/internal/agent"
    "github.com/dinar/mcp-conveer/internal/provider"
    "github.com/yourmodule/myagent"
)

func main() {
    llm := provider.NewAnthropic("claude-haiku-4-5")
    srv := agent.NewMCPServer(myagent.New(llm))

    log.Println("my-agent starting on stdio")
    if err := srv.ServeStdio(); err != nil {
        log.Fatal(err)
    }
}
```

This binary:
- Works as a Temporal worker agent (register via `MCPServer.Call`)
- Works as a standalone MCP server in Claude Code or any MCP-compatible client
- Requires no changes when switching between usage modes

## Writing Effective System Prompts

System prompts (roles) are the primary way to control agent behavior. Effective prompts:

1. **State the role concisely**: "You are a code reviewer."
2. **Describe the input**: "You receive a task description and source code."
3. **Specify the exact output schema**: Include the JSON structure the agent must return.
4. **Add a strict constraint**: "Return only valid JSON, no markdown, no extra text."

Example (`agents/callgraph-builder.md`):

```markdown
# Callgraph Builder

You receive a task description and source code.

Build a call graph for the relevant functions mentioned in the task.

Return JSON:
{
  "callgraph": [
    {"caller": "functionA", "callee": "functionB", "file": "path/to/file.go", "line": 42}
  ],
  "suspicious": ["list of function names that look problematic"]
}

Return only valid JSON.
```

Keep prompts tightly scoped. An agent that does one thing and returns a fixed schema is easier to chain, test, and replace than an agent with a broad mandate.

## Testing Agents

Test agents by calling `Handle` directly, without Temporal or MCP:

```go
func TestMyAgent_Handle(t *testing.T) {
    llm := &mockLLM{response: `{"finding": "nil pointer", "severity": "high", "recommendation": "add nil check"}`}
    a := myagent.New(llm)

    result, err := a.Handle(context.Background(), "analyze auth/token.go")
    require.NoError(t, err)

    var out map[string]string
    require.NoError(t, json.Unmarshal(result, &out))
    assert.Equal(t, "high", out["severity"])
}

type mockLLM struct{ response string }
func (m *mockLLM) Complete(_ context.Context, _, _ string) (string, error) {
    return m.response, nil
}
```

The `LLMProvider` interface makes the LLM swappable in tests. Test the agent logic without incurring API costs.
