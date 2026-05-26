# Configuration Reference

mcp-conveer is configured via a YAML file (conventionally `conveer.yaml`). The same file is used by both the worker (agent registration) and the CLI (workflow execution).

## Full Schema

```yaml
# teamlead — orchestrator configuration
teamlead:
  model: string        # required. LLM model for the teamlead agent.
  mode: string         # required. "sequential" or "routed".
  role: string         # optional. Inline system prompt for the teamlead.
  role_file: string    # optional. Path to system prompt file (relative to config dir).

# workflow — step graph (used in sequential mode)
workflow:
  - step: string       # optional. Step name for depends_on references.
                       # Defaults to agent name if omitted.
    agent: string      # required (unless loop). Agent name from agents list.
    depends_on:        # optional. List of step names that must complete first.
      - string
    loop:              # optional. Repeat a sub-graph until condition is met.
      max_iterations: int    # required. Maximum loop iterations.
      until: string          # optional. Condition string: "stepName.field == value".
      steps:                 # required. Steps to run each iteration.
        - agent: string
          depends_on: [...]

# agents — agent definitions
agents:
  - name: string       # required. Unique agent identifier.
    model: string      # required for type "" (LLM agent). LLM model string.
    role: string       # optional. Inline system prompt.
    role_file: string  # optional. Path to system prompt file (relative to config dir).
    type: string       # optional. "" (default LLM agent) or "mcp-client".
    # mcp-client fields (only when type: mcp-client):
    command: string    # Shell command to launch the MCP server process.
    tool: string       # MCP tool name to invoke.
    input_key: string  # Argument key for the task string (default: "query").
    input_step: string # Optional. Pass only this step's output instead of full context.
    input_field: string # Optional. Extract one string field from input_step JSON.
    extra_args:        # Optional. Additional fixed arguments merged into every tool call.
      key: value
```

## `teamlead`

The teamlead section configures the orchestrating agent. Its role depends on the `mode`.

| Field | Type | Required | Description |
|---|---|---|---|
| `model` | string | yes | Model string for the teamlead LLM (see Model Selection) |
| `mode` | string | yes | `"sequential"` or `"routed"` |
| `role` | string | no | Inline system prompt. Mutually exclusive with `role_file` |
| `role_file` | string | no | Path to a `.md` or `.txt` file containing the system prompt |

In `sequential` mode the teamlead model is not used at runtime — it is metadata only. In `routed` mode the teamlead LLM selects which agents to invoke and synthesizes the final result.

## `workflow`

Defines the step graph for `sequential` mode. Ignored in `routed` mode.

### Step fields

| Field | Type | Required | Description |
|---|---|---|---|
| `step` | string | no | Name used in `depends_on` references. Defaults to `agent` |
| `agent` | string | cond. | Agent name from the `agents` list. Required unless the step is a loop container |
| `depends_on` | []string | no | Step names that must complete before this step. Enables context chaining |
| `loop` | object | no | Turns the step into a repeating loop container |

### Loop fields

| Field | Type | Required | Description |
|---|---|---|---|
| `max_iterations` | int | yes | Hard cap on loop iterations |
| `until` | string | no | Termination condition. Format: `"stepName.fieldName == value"` |
| `steps` | []Step | yes | Steps to execute each iteration |

**Loop condition syntax:**

```
stepName.fieldName == value
```

- `stepName` — the name of a step inside the loop
- `fieldName` — a top-level JSON field in that step's output
- `value` — `true`, `false`, a number, or a quoted/unquoted string

Examples:
```yaml
until: reviewer.approved == true
until: checker.score == 10
until: validator.status == "pass"
```

Steps inside a loop can reference outer steps via `depends_on`. Those dependencies are resolved once (from pre-loop results) and injected each iteration.

## `agents`

Each agent in the list is registered in the Activity Registry and made available to workflow steps.

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | yes | Unique identifier. Referenced by workflow steps and `depends_on` |
| `type` | string | no | `""` (default, LLM agent) or `"mcp-client"` |
| `model` | string | type `""` | LLM model string (see Model Selection) |
| `role` | string | no | Inline system prompt. Use for short roles (<3 lines) |
| `role_file` | string | no | Path to a Markdown/text file with the system prompt. Relative to config directory |
| `command` | string | type `mcp-client` | Full shell command to launch the external MCP server process |
| `tool` | string | type `mcp-client` | MCP tool name to invoke on the external server |
| `input_key` | string | no | Argument key for the task string (default: `"query"`) |
| `input_step` | string | no | Pass only this step's JSON output as the tool argument instead of the full enriched context |
| `input_field` | string | no | Extract a single string field from `input_step` JSON (e.g. `"library"` → passes `"gin"` instead of `{"library":"gin"}`) |
| `extra_args` | map[string]string | no | Additional fixed arguments merged into every tool call alongside `input_key`. Required when the MCP tool has multiple required parameters |

`role` and `role_file` are mutually exclusive. If both are set, `role_file` takes precedence.

### Agent types

**`type: ""` (default) — LLM agent**

Calls an LLM with a system prompt and returns the response as JSON. Requires `model` and `role` or `role_file`.

**`type: mcp-client` — external MCP server**

Launches an external MCP server process via stdio and calls the specified tool. The task string is passed as `{input_key: task}`. The tool's text response is returned as `json.RawMessage` to the next pipeline step.

Any stdio MCP server works without code changes — Context7, filesystem, GitHub, or a custom server. Only `command` and `tool` change between servers.

```yaml
- name: context7-fetcher
  type: mcp-client
  command: "npx -y @upstash/context7-mcp@latest"
  tool: resolve-library-id
  input_key: libraryName
  input_step: lib-extractor
  input_field: library
  extra_args:
    query: "API documentation and usage examples"
```

**`extra_args`** lets you pass additional fixed arguments alongside the main `input_key` value. Required when the MCP tool has multiple required parameters (e.g. Context7's `resolve-library-id` requires both `libraryName` and `query`).

This enables external services (documentation APIs, code search tools, linters, filesystem access) to participate as explicit, retriable steps in the Temporal pipeline.

**`input_step` and `input_field`**

By default, the mcp-client receives the full enriched task string (original task + all `depends_on` results). Some tools expect a short, structured input instead — for example, a library name rather than paragraphs of context.

Use `input_step` to pass only a specific prior step's JSON output as the tool argument. Use `input_field` together with `input_step` to extract a single string field from that JSON:

```yaml
# Step 1: LLM extracts the library name
- name: lib-extractor
  model: deepseek/deepseek-v4-flash
  role: 'Extract the Go library name from the task. Return JSON: {"library": "name"}.'

# Step 2: mcp-client receives only "temporal", not the full context
- name: docs-resolver
  type: mcp-client
  command: "npx -y @upstash/context7-mcp@latest"
  tool: resolve-library-id
  input_key: libraryName
  input_step: lib-extractor   # use only lib-extractor's output
  input_field: library        # extract "temporal" from {"library": "temporal"}
```

Without `input_step`, the tool receives the entire enriched context string — suitable for tools that accept free-form text (search engines, linters, code analysis tools).

## Model Selection

The model string determines which LLM provider is used:

| Format | Provider | Env var required |
|---|---|---|
| `deepseek/<model>` | DeepSeek API | `DEEPSEEK_API_KEY` |
| `ollama/<model>` | Local Ollama | none (`OLLAMA_BASE_URL` optional) |
| `claude-<variant>` | Anthropic | `ANTHROPIC_API_KEY` |
| `<any other string>` | Anthropic | `ANTHROPIC_API_KEY` |

**Examples:**

```yaml
model: deepseek/deepseek-chat      # DeepSeek
model: deepseek/deepseek-v4-flash  # DeepSeek (specific variant)
model: ollama/llama3               # local Ollama
model: ollama/mistral              # local Ollama, different model
model: claude-haiku-4-5            # Anthropic Haiku
model: claude-sonnet-4-6           # Anthropic Sonnet
model: claude-opus-4-7             # Anthropic Opus
```

## Environment Variables

| Variable | Provider | Default | Description |
|---|---|---|---|
| `DEEPSEEK_API_KEY` | DeepSeek | — | Required for DeepSeek models |
| `ANTHROPIC_API_KEY` | Anthropic | — | Required for Claude models |
| `OLLAMA_BASE_URL` | Ollama | `http://localhost:11434` | Custom Ollama endpoint |

## CLI Flags

### `mcp-conveer run`

| Flag | Default | Description |
|---|---|---|
| `--config` | `conveer.yaml` | Path to config file |
| `--task` | — | Task description passed to the first pipeline step |
| `--temporal` | `localhost:7233` | Temporal server address |

### `worker`

| Flag | Default | Description |
|---|---|---|
| `--config` | — | Path to config file. Registers all agents from the `agents` list |
| `--temporal` | `localhost:7233` | Temporal server address |

### `agent`

Runs a single agent as a standalone MCP server over stdio.

| Flag | Default | Description |
|---|---|---|
| `--name` | — | Agent name as defined in the config `agents` list (required) |
| `--config` | — | Path to config file (required) |

```bash
./agent --name task-reader --config examples/incident-review/incident-review.yaml
```

## Complete Examples

### Simple parallel scan (3 agents, no dependencies)

```yaml
teamlead:
  model: deepseek/deepseek-chat
  mode: sequential

workflow:
  - agent: scanner
  - agent: security
  - agent: performance

agents:
  - name: scanner
    model: ollama/llama3
    role: "Find structural and syntax issues. Return JSON with an issues array."

  - name: security
    model: deepseek/deepseek-chat
    role: "Find security vulnerabilities. Return JSON with severity and description."

  - name: performance
    model: claude-haiku-4-5
    role: "Find performance bottlenecks. Return JSON with recommendations."
```

All three agents run in parallel. No context is passed between them.

### Incident review (sequential with loop)

```yaml
teamlead:
  model: deepseek/deepseek-chat
  mode: sequential

workflow:
  - step: read-task
    agent: task-reader

  - step: build-callgraph
    agent: callgraph-builder
    depends_on: [read-task]

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
    model: deepseek/deepseek-chat
    role: "Extract key terms and expected behavior from the incident description."

  - name: callgraph-builder
    model: deepseek/deepseek-chat
    role_file: ./agents/callgraph-builder.md

  - name: incident-finder
    model: deepseek/deepseek-chat
    role_file: ./agents/incident-finder.md

  - name: problem-solver
    model: deepseek/deepseek-chat
    role_file: ./agents/problem-solver.md
```

### Feature implementation (sequential with review loop)

```yaml
teamlead:
  model: deepseek/deepseek-chat
  mode: sequential

workflow:
  - step: plan
    agent: planner

  - step: design-api
    agent: api-designer
    depends_on: [plan]

  - step: implement
    loop:
      max_iterations: 4
      until: reviewer.approved == true
      steps:
        - agent: implementer
          depends_on: [plan, design-api]
        - agent: test-writer
          depends_on: [implementer]
        - agent: reviewer
          depends_on: [implementer, test-writer]

agents:
  - name: planner
    model: claude-opus-4-7
    role_file: ./agents/planner.md

  - name: api-designer
    model: claude-sonnet-4-6
    role_file: ./agents/api-designer.md

  - name: implementer
    model: claude-sonnet-4-6
    role_file: ./agents/implementer.md

  - name: test-writer
    model: claude-haiku-4-5
    role_file: ./agents/test-writer.md

  - name: reviewer
    model: claude-opus-4-7
    role_file: ./agents/reviewer.md
```

### Routed mode

```yaml
teamlead:
  model: deepseek/deepseek-chat
  mode: routed
  role: "You are a teamlead. Select the appropriate agents for the given task."

agents:
  - name: scanner
    model: ollama/llama3
    role: "Find structural and syntax issues"

  - name: security
    model: deepseek/deepseek-chat
    role: "Find security vulnerabilities"

  - name: performance
    model: claude-haiku-4-5
    role: "Find performance bottlenecks"
```

The teamlead LLM receives the available agent names and roles, then returns a JSON array of agent names to invoke. Those agents run in parallel. The teamlead synthesizes a final summary.

### Pipeline with external MCP service (Context7 docs)

Demonstrates the full `mcp-client` chain: LLM extracts the library name → Context7 resolves it → LLM describes the function using real docs.

```yaml
workflow:
  - step: read-code
    agent: code-reader

  - step: extract-libs
    agent: lib-extractor
    depends_on: [read-code]

  - step: fetch-docs
    agent: context7-fetcher
    depends_on: [extract-libs]

  - step: describe
    agent: func-describer
    depends_on: [read-code, extract-libs, fetch-docs]

agents:
  - name: code-reader
    model: deepseek/deepseek-v4-flash
    role_file: ./agents/code-reader.md

  - name: lib-extractor
    model: deepseek/deepseek-v4-flash
    role_file: ./agents/lib-extractor.md

  - name: context7-fetcher
    type: mcp-client
    command: "npx -y @upstash/context7-mcp@latest"
    tool: resolve-library-id
    input_key: libraryName        # receives "gin" (extracted from lib-extractor output)
    input_step: extract-libs      # use only this step's output, not full context
    input_field: library          # extract {"library": "gin"} → "gin"
    extra_args:
      query: "API documentation and usage examples"   # Context7 requires two args

  - name: func-describer
    model: deepseek/deepseek-v4-flash
    role_file: ./agents/func-describer.md
```

`context7-fetcher` runs at a defined point in the graph and receives only the library name — not the entire accumulated context. `func-describer` gets all three prior outputs via `depends_on`: the code, the library metadata, and the Context7 response.

Any stdio MCP server plugs in by changing only `command` and `tool`. See [`examples/code-review/`](../examples/code-review/) for a runnable version.
