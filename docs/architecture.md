# Architecture

## Overview

mcp-conveer implements deterministic AI agent orchestration using a three-layer model:

```
┌──────────────────────────────────────────────────────────────┐
│  Layer 1 — Temporal Workflow                                  │
│  Defines the call graph as code: order, parallelism, loops   │
│                                                               │
│   SequentialWorkflow / RoutedWorkflow / FanOutWorkflow        │
└─────────────┬──────────────────────────┬─────────────────────┘
              │                          │
     Activity: CallAgent        Activity: CallAgent
              │                          │
┌─────────────▼──────────────────────────▼─────────────────────┐
│  Layer 2 — Activity Registry                                  │
│  Maps agent names to MCPServer instances; injects context     │
└─────────────┬──────────────────────────┬─────────────────────┘
              │                          │
       MCPServer.Call()           MCPServer.Call()
              │                          │
┌─────────────▼────────────┐  ┌──────────▼────────────────────┐
│  Layer 3 — MCP Server    │  │  Layer 3 — MCP Server         │
│  filescanner agent       │  │  generic agent                │
│  Claude Haiku            │  │  DeepSeek / Ollama            │
└──────────────────────────┘  └───────────────────────────────┘
```

**Layer 1 (Temporal Workflow)** describes what to run and in what order. It is pure Go code with no LLM calls of its own. It dispatches Activities and aggregates results.

**Layer 2 (Activity Registry)** is the bridge between Temporal and agents. `CallAgent` looks up the named MCPServer, injects context from prior steps, and invokes the agent. Temporal handles retries, timeouts, and state serialization at this boundary.

**Layer 3 (MCP Server / Agent)** is where the actual work happens. Each agent encapsulates its own LLM provider, system prompt, and output format. From Temporal's perspective, an agent is a black box that accepts a string task and returns JSON.

---

## Component Reference

### `internal/agent` — Agent Interface

```go
type Agent interface {
    Name() string
    Handle(ctx context.Context, task string) (json.RawMessage, error)
}
```

The minimal contract every agent implements. `Handle` receives the enriched task (original task + context from prior steps) and returns a JSON result.

`MCPServer` wraps any `Agent` as an MCP server. It exposes the agent's `Handle` method as a single MCP tool and provides two invocation modes:
- `ServeStdio()` — runs as a standalone MCP server on stdin/stdout, usable from Claude Code or any MCP client
- `Call(ctx, task)` — direct in-process call used by the Activity Registry, bypassing MCP transport

### `internal/agents/generic` — Generic LLM Agent

Configurable agent that drives any `LLMProvider` with a system prompt (role). If the LLM returns valid JSON it passes through as-is; otherwise it is wrapped in `{"response": "..."}` so downstream agents always receive consistent JSON.

### `internal/agents/mcpclient` — MCP Client Agent

An agent that calls an *external* MCP server process as a pipeline step. It launches the specified command via stdio, initializes the MCP connection, calls the named tool, and returns the text output as `json.RawMessage`.

This enables external MCP services (documentation APIs, code search, linters) to participate as first-class steps in the Temporal graph — at a deterministic point, with Temporal retries, without an agentic loop.

```yaml
- name: fetch-docs
  type: mcp-client
  command: "npx -y @upstash/context7-mcp@latest"
  tool: get-library-docs
  input_key: query          # argument key passed to the tool (default: "query")
```

The external process is started fresh per Activity invocation. This ensures statelessness and makes retries safe.

### `internal/agents/filescanner` — File Scanner Agent

Specialized agent that reads a file from disk and asks the LLM to return structured findings. The output schema is fixed:

```json
{
  "file": "path/to/file.go",
  "issues": [
    {"severity": "error|warning|info", "line": 42, "message": "..."}
  ],
  "summary": "one-line summary"
}
```

### `internal/activity` — Temporal Activity

`Registry` holds named `MCPServer` instances. `CallAgent` is the single Temporal Activity:

1. Looks up the agent by name in the registry
2. Builds the enriched task: appends `depends_on` step outputs to the original task
3. Calls `MCPServer.Call(ctx, enrichedTask)`
4. Returns the JSON result to Temporal for state persistence

Context injection format (appended to the task string):

```
[original task]

--- Results from previous steps ---

[step-name-1]
{"key": "value", ...}

[step-name-2]
{"key": "value", ...}
```

### `internal/config` — Configuration & Topology

`Load()` parses `conveer.yaml` into a `Config` struct.

`BuildLayers()` performs a topological sort on workflow steps:
- Groups steps that can run in parallel into a single layer
- Validates that all `depends_on` references exist
- Returns an error on cycles
- Accepts `outerResolved` step names for loop steps (dependencies on steps defined outside the loop)

### `internal/provider` — LLM Providers

All providers implement:

```go
type LLMProvider interface {
    Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}
```

`ModelPrefix()` routes by prefix:

| Input | Provider | Name passed to provider |
|---|---|---|
| `deepseek/deepseek-chat` | DeepSeek | `deepseek-chat` |
| `ollama/llama3` | Ollama | `llama3` |
| `claude-sonnet-4-6` | Anthropic | `claude-sonnet-4-6` |

### `internal/workflow` — Workflow Types

**SequentialWorkflow** — main workflow for YAML-defined pipelines. Calls `BuildLayers`, runs each layer as a parallel batch of Activities, accumulates step results in `stepResults` map, evaluates loop conditions.

**RoutedWorkflow** — teamlead-driven workflow. The teamlead selects agents at runtime, they run in parallel, the teamlead synthesizes a final summary.

**FanOutWorkflow** — runs a single agent against a list of tasks in parallel. Used for bulk processing (e.g., scan 1000 files with the same agent).

### `internal/worker` — Worker Startup

`Start()` creates a Temporal client, registers all workflows and activities under the `mcp-conveer` task queue, and blocks until interrupted.

---

## Data Flow: Sequential Pipeline

```
CLI: mcp-conveer run --config --task
          │
          ▼
  config.Load() → parse YAML
          │
          ▼
  client.ExecuteWorkflow(SequentialWorkflow, {Config, Task})
          │
          ▼
  [Worker process]
  SequentialWorkflow receives input
          │
          ▼
  config.BuildLayers(workflow.steps)
  → Layer 0: [read-task]
  → Layer 1: [build-callgraph]   (depends_on: read-task)
  → Layer 2: [resolve/loop]      (depends_on: build-callgraph)
          │
          ▼ (for each layer, parallel goroutines)
  workflow.ExecuteActivity(Registry.CallAgent, {agent, task, context})
          │
          ▼ (Temporal Activity boundary — state persisted here)
  Registry.CallAgent:
    1. lookup MCPServer by agent name
    2. buildTaskWithContext(task, depends_on outputs)
    3. MCPServer.Call(ctx, enrichedTask)
    4. Agent.Handle → LLM.Complete → JSON result
          │
          ▼
  result stored in stepResults[stepName]
  → passed as context to dependent steps
          │
          ▼
  SequentialResult{Steps: [...]} → JSON printed to stdout
```

---

## Loop Execution

Loop steps re-execute their inner steps until either `max_iterations` is reached or the `until` condition evaluates to true.

The `until` condition is a simple equality check: `stepName.fieldName == value`. The condition is evaluated against `localResults` — the accumulated outputs of loop-internal steps.

Loop inner steps can reference outer steps via `depends_on`. `BuildLayers` is called with `outerResolved` containing outer step names; those dependencies are satisfied without requiring inner re-execution.

```yaml
loop:
  max_iterations: 5
  until: reviewer.approved == true   # check reviewer's JSON output
  steps:
    - agent: implementer
      depends_on: [plan, design-api]  # outer steps — passed via outerResolved
    - agent: reviewer
      depends_on: [implementer]       # inner step — re-executed each iteration
```

---

## Key Design Decisions

### Two execution modes

Every agent defined in YAML can run in two modes without code changes:

**Mode 1 — In-process (Temporal pipeline)**
```
./worker --config pipeline.yaml
```
The worker loads all agents into memory and the Activity Registry calls them directly via `MCPServer.Call()`. Zero IPC overhead. Temporal provides durability, retries, and visibility.

**Mode 2 — Standalone MCP server**
```
./agent --name task-reader --config pipeline.yaml
```
The `cmd/agent` binary loads a single agent and serves it via `ServeStdio()` (MCP stdio transport). Any MCP client can connect — Claude Code, n8n, custom tooling.

```json
// Claude Code: ~/.claude/settings.json
"mcpServers": {
  "task-reader": {
    "command": "/path/to/agent",
    "args": ["--name", "task-reader", "--config", "/path/to/pipeline.yaml"]
  }
}
```

Same agent code, same YAML config — different transport. Mode 1 is the right choice for production pipelines. Mode 2 makes individual agents reusable in the broader MCP ecosystem.

### Why explicit pipeline instead of agentic loops?

Modern AI frameworks often use an **agentic loop**: the LLM runs inside a single Activity, decides which tools to invoke, calls them, and iterates — all within one Temporal Activity boundary.

mcp-conveer uses **one Activity = one LLM call**. External services are explicit pipeline steps, not hidden tool calls.

| | Agentic loop | Explicit pipeline |
|---|---|---|
| Call graph | LLM decides at runtime | Defined in YAML, fixed |
| Retry granularity | Entire loop restarts | Each step retried independently |
| Cost | Multiple LLM calls, hard to predict | One LLM call per Activity |
| Observability | Black box to Temporal | Every step visible in Temporal UI |
| Determinism | LLM may take different paths | Same graph every run |

When external data is needed (documentation, search results, lint output), an `mcp-client` step fetches it at a defined point in the graph and passes the result to the next LLM step via `depends_on`. The LLM sees the data in its context — no tool use loop required.

### Why MCP as the agent interface?

MCP gives a standardized contract: a named tool with a typed input schema. This means:
- The orchestrator doesn't know the agent's implementation details
- The same agent runs in the Temporal graph (Mode 1) and as a standalone MCP server (Mode 2)
- The contract is self-documenting via the tool schema

Plain Go functions would work functionally but lose the dual-mode capability and MCP ecosystem compatibility.

### Why Temporal instead of goroutines?

Goroutines are ephemeral. If the process restarts mid-pipeline, all progress is lost. Temporal persists workflow state in PostgreSQL at every Activity boundary. A 1000-file pipeline that crashes at file 500 resumes from file 501 automatically.

Additional Temporal benefits: built-in retries with backoff, configurable timeouts, workflow visibility UI, cron scheduling, and concurrency control — all without writing infrastructure code.

### Why YAML config instead of code?

Go code can always describe the graph, but most pipelines follow the same patterns (sequential with dependencies, loops). YAML lets non-Go developers define pipelines, makes the graph reviewable without reading code, and enables tooling (validators, visualizers) without parsing Go AST.

The Go implementation handles the complex cases (topological sort, loop condition evaluation) so the YAML remains simple.

### Why per-step model selection?

Not all tasks require the same model. File scanning is a job for Haiku (fast, cheap). Analysis requires Sonnet. Final decisions with nuanced reasoning benefit from Opus. Fixing one model for the entire pipeline means either overpaying for simple tasks or underperforming on complex ones.

The `LLMProvider` interface makes swapping models a one-line YAML change with no code changes.

### Why context chaining (not shared history)?

Passing the full conversation history to every agent causes context window bloat, inflates cost, and degrades quality as noise accumulates. Each agent should see only what it needs: the original task and the compressed outputs of its direct dependencies. `buildTaskWithContext` implements this as a simple string concatenation — no framework required.
