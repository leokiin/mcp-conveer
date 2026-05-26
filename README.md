# mcp-conveer

> Deterministic AI agent orchestration via Temporal + MCP in Go.
> *"LangGraph for Temporal users — durable AI pipelines with MCP nodes, any model per step."*

---

## The Problem

Processing many files or documents with a single LLM agent fills its context window with raw data. Cost grows, quality drops. You need the orchestrator's context to stay clean — agents must work in isolation and return only compressed results.

## The Solution

Each pipeline node is an MCP server. Temporal Workflow describes the call graph as code — not as a prompt. Any node can be an LLM agent (Claude, DeepSeek, Ollama), a RAG service, a validator, or an external API. Temporal handles order, state, and retries.

```
Temporal Workflow  (call graph, state, retries)
  ├── Activity → MCP server: task-reader      → DeepSeek / Haiku   (parse task)
  ├── Activity → MCP server: callgraph-builder → Ollama / DeepSeek  (local or cheap)
  ├── Activity → MCP server: incident-finder  → DeepSeek / Sonnet  (analyze)
  └── Activity → MCP server: problem-solver   → DeepSeek / Sonnet  (fix)
```

Each agent is isolated: it receives only its task plus the compressed outputs of its direct dependencies — no raw data, no growing history.

---

## Key Properties

| Property | What it means |
|---|---|
| **Deterministic** | The call graph is code, not a prompt |
| **Durable** | Crashes on file 500 of 1000? Temporal resumes from file 501 |
| **Context isolation** | Each agent sees only its task + direct dependency outputs |
| **Context chaining** | `depends_on` outputs are automatically forwarded as enriched context |
| **Model freedom** | DeepSeek for cost, Ollama for private data, Claude for quality — per step |
| **MCP-agnostic client** | Any stdio MCP server plugs in via `type: mcp-client` in YAML — Context7, filesystem, GitHub, custom servers. Commands support quoted arguments for packages with spaces. |

---

## vs LangGraph

| | LangGraph | mcp-conveer |
|---|---|---|
| Durability | No — restart from scratch on crash | **Yes — Temporal resumes from last completed Activity** |
| Node protocol | Python callable | **Go struct registered in-process via MCP interface** |
| Graph definition | Code | **YAML config** |
| State management | In-memory / LangGraph Platform (paid) | **Temporal (free, self-hosted)** |
| Context passing | Shared growing history | **Compressed JSON per dependency** |

---

## Prerequisites

- Go 1.22+
- Docker (for Temporal + PostgreSQL)
- An API key for at least one provider: DeepSeek, Anthropic, or a local Ollama installation

## Quickstart

### 1. Start Temporal + PostgreSQL

```bash
docker compose up -d
```

This starts:
- PostgreSQL on port 5432 (Temporal backend)
- Temporal server on port 7233
- Temporal UI on [localhost:8080](http://localhost:8080)

### 2. Set API key

```bash
# DeepSeek (recommended — cheap, OpenAI-compatible)
export DEEPSEEK_API_KEY=sk-...

# Or Anthropic Claude
export ANTHROPIC_API_KEY=sk-ant-...

# Ollama runs locally — no key needed
# export OLLAMA_BASE_URL=http://localhost:11434  # default
```

### 3. Build

```bash
go build ./cmd/mcp-conveer ./cmd/worker ./cmd/agent
```

### 4. Start the worker

```bash
./worker --config examples/incident-review/incident-review.yaml
```

The worker registers all agents defined in the config and listens for Temporal tasks.

### 5. Run a pipeline

```bash
./mcp-conveer run \
  --config examples/incident-review/incident-review.yaml \
  --task "find why auth/token.go panics on nil user"
```

Output is a JSON object with all step results:

```json
{
  "steps": [
    {"step_name": "read-task", "result": {"response": "..."}},
    {"step_name": "build-callgraph", "result": {"callgraph": [...]}},
    {"step_name": "resolve", "result": {"fix": "..."}}
  ]
}
```

### 6. (Optional) Run an agent as a standalone MCP server

Any agent from the config can be served over stdio — for use in Claude Code, n8n, or any MCP client:

```bash
./agent --name task-reader --config examples/incident-review/incident-review.yaml
```

Add to Claude Code (`~/.claude/settings.json`):

```json
"mcpServers": {
  "task-reader": {
    "command": "/path/to/agent",
    "args": ["--name", "task-reader", "--config", "/path/to/pipeline.yaml"]
  }
}
```

---

## Orchestration Modes

### `sequential` — explicit graph with optional parallelism

The call graph is defined in YAML. Steps without `depends_on` run in parallel automatically. Steps with `depends_on` receive their dependency outputs as additional context.

```yaml
teamlead:
  model: deepseek/deepseek-v4-flash
  mode: sequential

workflow:
  - step: read-task
    agent: task-reader

  - step: build-callgraph
    agent: callgraph-builder
    depends_on: [read-task]            # runs after read-task, receives its output

  - step: resolve
    depends_on: [build-callgraph]
    loop:
      max_iterations: 5
      until: incident-finder.found == true
      steps:
        - agent: incident-finder
          depends_on: [build-callgraph, read-task]
        - agent: problem-solver
          depends_on: [incident-finder]

agents:
  - name: task-reader
    model: deepseek/deepseek-v4-flash
    role: "Extract key terms and expected behavior from the incident description."

  - name: callgraph-builder
    model: deepseek/deepseek-v4-flash      # or: ollama/llama3 (code never leaves machine)
    role_file: ./agents/callgraph-builder.md
```

### `routed` — teamlead LLM decides which agents to call

```yaml
teamlead:
  model: deepseek/deepseek-v4-flash
  mode: routed

agents:
  - name: scanner
    model: ollama/llama3
    role: "Find structural and syntax issues"

  - name: security
    model: deepseek/deepseek-v4-flash
    role: "Find security vulnerabilities"
```

The teamlead selects agents at runtime → they run in parallel → the teamlead synthesizes results. Use when the task is open-ended and the required agents are not known in advance.

---

## Context Chaining

Each step with `depends_on` automatically receives its dependency outputs appended to the task. The agent sees the original task plus only the data it needs:

```
[original task]

--- Results from previous steps ---

[read-task]
{"response": "auth/token.go panics when user is nil in GenerateToken()"}

[build-callgraph]
{"callgraph": [...], "suspicious": ["GenerateToken", "validateUser"]}
```

Raw conversation history never accumulates. Each agent's context window stays clean.

### Context grows too large? Add a summarizer step in YAML.

When a step has many upstream dependencies, its context can grow large. The fix is one new step in the config — no code changes:

**Before** — `describe` receives three raw outputs, including verbose Context7 docs:

```yaml
  - step: describe
    agent: func-describer
    depends_on: [extract-func, extract-libs, fetch-docs]
```

**After** — a cheap summarizer compresses all three into ~500 tokens, `describe` gets a clean input:

```yaml
  - step: summarize-context
    agent: summarizer
    depends_on: [extract-func, extract-libs, fetch-docs]

  - step: describe
    agent: func-describer
    depends_on: [summarize-context]
```

```yaml
agents:
  - name: summarizer
    model: deepseek/deepseek-v4-flash   # cheap model — compression is a simple task
    role: |
      Compress the inputs into a compact JSON object. Drop raw data.
      Keep: key findings, relevant code, library name, 2-3 sentence doc summary.
      Target: under 500 tokens.
```

The same pattern applies anywhere context accumulates: after a fan-out, before an expensive model, at the entry point of a loop.

---

## Supported Models

```yaml
agents:
  - name: cheap-worker
    model: deepseek/deepseek-v4-flash    # DeepSeek — cheap, OpenAI-compatible

  - name: private-scanner
    model: ollama/llama3             # local — code never leaves the machine

  - name: analyzer
    model: claude-sonnet-4-6         # Anthropic Claude Sonnet

  - name: reviewer
    model: claude-opus-4-7           # Anthropic Claude Opus (highest quality)
```

| Prefix | Provider | Required env var |
|---|---|---|
| `deepseek/` | DeepSeek API | `DEEPSEEK_API_KEY` |
| `ollama/` | Local Ollama | — (no key needed) |
| *(none)* | Anthropic | `ANTHROPIC_API_KEY` |

---

## Examples

| Example | What it does |
|---|---|
| [`examples/incident-review/`](examples/incident-review/) | Bug hunt: read task → build callgraph → find + fix loop |
| [`examples/feature-implementation/`](examples/feature-implementation/) | Feature: plan → API design → implement + test + review loop |
| [`examples/code-review/`](examples/code-review/) | Code description: extract code → detect library → fetch docs from Context7 (MCP) → describe function |

---

## Architecture

```
cmd/
  mcp-conveer/   CLI: mcp-conveer run --config --task
  worker/        Temporal worker: loads agents from config, starts listening
  agent/         Standalone MCP server: serves one agent over stdio

internal/
  agent/         Agent interface + MCPServer wrapper (mark3labs/mcp-go)
  agents/
    filescanner/ Specialized agent: reads a file, returns JSON issues
    generic/     Configurable LLM agent driven by a system prompt (role)
    mcpclient/   MCP client agent: calls an external MCP server as a pipeline step
  activity/      Temporal Activity: Registry.CallAgent + context injection
  builder/       Shared factory: NewProvider / NewAgent / ResolveRole
  config/        YAML parsing + BuildLayers() topological sort
  provider/      LLM providers: Anthropic, DeepSeek, Ollama
  workflow/      SequentialWorkflow, RoutedWorkflow, FanOutWorkflow
  worker/        Worker startup helper

examples/
  incident-review/        4-agent bug-hunt pipeline
  feature-implementation/ 5-agent feature pipeline with review loop
  code-review/            4-agent code-description pipeline with Context7 MCP
```

### Two execution modes

```
┌─────────────────────────────────────────────────────────────┐
│ Mode 1 — Temporal pipeline (cmd/worker)                     │
│                                                             │
│  YAML config → worker registers agents in-process          │
│  Temporal Activity calls agents directly (zero IPC)        │
│  Durable: crashes resume from last completed step          │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ Mode 2 — Standalone MCP server (cmd/agent)                  │
│                                                             │
│  ./agent --name task-reader --config pipeline.yaml         │
│  Serves the agent over stdio (MCP protocol)                │
│  Connects to Claude Code, n8n, any MCP client              │
└─────────────────────────────────────────────────────────────┘
```

Same agent code, same YAML config — different transport.

See [`docs/architecture.md`](docs/architecture.md) for a deep dive into design decisions.

---

## Documentation

| Document | Contents |
|---|---|
| [`docs/architecture.md`](docs/architecture.md) | Three-layer design, data flow, decision rationale |
| [`docs/configuration.md`](docs/configuration.md) | Full YAML config reference with all fields |
| [`docs/custom-agents.md`](docs/custom-agents.md) | How to implement and register a custom agent |
| [`docs/providers.md`](docs/providers.md) | Provider setup: Anthropic, DeepSeek, Ollama |

---

## Reliability & Safety

| Concern | Behaviour |
|---|---|
| **HTTP timeouts** | All LLM providers use a 90-second client timeout — no goroutine leaks on hung requests |
| **Temporal determinism** | Agent output ordering is sorted before synthesis so workflow history is deterministic on replay |
| **Concurrent registry** | `activity.Registry` uses `sync.RWMutex` — safe to call `Register` and `CallAgent` from different goroutines |
| **File-scanner paths** | `filescanner` agent accepts **absolute paths only** — rejects relative paths to prevent unintended traversal |
| **mcp-client commands** | Commands in `type: mcp-client` config are parsed with quote-aware splitting — single and double quotes preserve spaces in arguments (e.g. `npx -y "@scope/pkg arg"`) |
| **Error identities** | `UnknownAgentError` is exported and supports `errors.As` for typed error handling in callers |

---

## Stack

- **Go** — single binary, native concurrency
- **Temporal** (`go.temporal.io/sdk`) — durability, retries, visibility
- **PostgreSQL** — Temporal persistence backend
- **MCP** (`mark3labs/mcp-go`) — agent protocol, 8600+ stars
- **Anthropic** (`anthropic-sdk-go`) — Claude API
- **DeepSeek** — cheap OpenAI-compatible API
- **Ollama** — local models via OpenAI-compatible API

## Why Not Just Goroutines?

Goroutines don't survive process restart. Processing 1000 files and crashing halfway means starting over. Temporal stores workflow state in PostgreSQL and resumes from the last completed Activity — automatically, without code changes.

## Why Not Agentic Loops?

Modern AI frameworks often use an **agentic loop**: the LLM runs inside a single call, decides which tools to invoke, calls them, reasons over results, and iterates until done — all within one Activity.

mcp-conveer takes the opposite approach: **one Activity = one LLM call**. Each step does one thing and passes its output to the next step via `depends_on`.

| | Agentic loop | mcp-conveer (explicit pipeline) |
|---|---|---|
| Call graph | LLM decides at runtime | Defined in YAML, fixed |
| Retry granularity | Entire loop restarts | Each step retried independently |
| Cost | Multiple LLM calls per Activity, hard to predict | One LLM call per Activity |
| Observability | Black box — Temporal sees one Activity | Every step visible in Temporal UI |
| Debuggability | Hard — need to trace inside the loop | Each step output inspectable |
| Determinism | LLM may take different paths on retry | Same graph every run |

The explicit pipeline model means external services (documentation APIs, code search, linters) are **first-class steps** in the graph — not hidden tool calls inside an LLM's reasoning. They run at a predictable point, their output is stored, and they can be retried independently if they fail.

Anthropic's own multi-agent guidance recommends explicit orchestration over autonomous tool-use loops wherever the workflow can be defined in advance — it is cheaper, more reliable, and easier to debug.

The tradeoff: agentic loops handle open-ended tasks where the required steps are truly unknown upfront. mcp-conveer is the right choice when the pipeline structure is known — processing pipelines, code review, incident analysis, feature implementation workflows.
