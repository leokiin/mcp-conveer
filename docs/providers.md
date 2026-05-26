# Providers

A provider is the bridge between an agent and an LLM API. All providers implement:

```go
type LLMProvider interface {
    Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}
```

The provider receives a system prompt (the agent's role) and a user prompt (the enriched task) and returns the model's raw text response.

---

## Anthropic (Claude)

**Package:** `internal/provider/anthropic.go`

**Models:** `claude-haiku-4-5`, `claude-sonnet-4-6`, `claude-opus-4-7` (and other Claude variants)

**Setup:**

```bash
export ANTHROPIC_API_KEY=sk-ant-...
```

**Config:**

```yaml
agents:
  - name: analyzer
    model: claude-sonnet-4-6        # no prefix = Anthropic
```

The Anthropic provider uses the official `anthropic-sdk-go` client. The API key is picked up automatically from `ANTHROPIC_API_KEY` by the SDK.

**Current limitations:**
- `MaxTokens` is fixed at 2048. For agents that produce long outputs (large JSON, detailed reports), this may truncate responses. Consider this when designing agent output schemas.

**Model selection guide:**

| Model | Best for | Cost |
|---|---|---|
| `claude-haiku-4-5` | Fast, cheap, simple extraction tasks | Low |
| `claude-sonnet-4-6` | Analysis, code generation, most tasks | Medium |
| `claude-opus-4-7` | Complex reasoning, final decisions | High |

---

## DeepSeek

**Package:** `internal/provider/deepseek.go`

**Models:** `deepseek-chat`, `deepseek-v4-flash`, and other DeepSeek model IDs

**Setup:**

```bash
export DEEPSEEK_API_KEY=sk-...
```

**Config:**

```yaml
agents:
  - name: worker
    model: deepseek/deepseek-chat    # "deepseek/" prefix = DeepSeek
```

The DeepSeek provider uses the OpenAI-compatible REST API at `https://api.deepseek.com/chat/completions`. It implements the API directly without an SDK dependency.

**Why DeepSeek?** Significantly cheaper than Claude for high-volume tasks. The OpenAI-compatible format means the implementation is simple. Recommended for worker agents in large-scale pipelines where cost matters.

---

## Ollama (local)

**Package:** `internal/provider/ollama.go`

**Models:** Any model available in your local Ollama installation

**Setup:**

```bash
# Install Ollama: https://ollama.ai
ollama pull llama3
ollama pull mistral
# Ollama runs on http://localhost:11434 by default

# Optional: custom endpoint
export OLLAMA_BASE_URL=http://my-ollama-server:11434
```

**Config:**

```yaml
agents:
  - name: private-scanner
    model: ollama/llama3    # "ollama/" prefix + model name
```

The Ollama provider calls the `/api/chat` endpoint with `stream: false`. No API key required.

**Why Ollama?** For pipelines that process sensitive or private code. Ollama runs entirely on your machine — no data leaves the network. Pair with cloud providers for non-sensitive steps in the same pipeline:

```yaml
agents:
  - name: private-scanner
    model: ollama/llama3             # sensitive code → local
  - name: report-writer
    model: deepseek/deepseek-chat    # summary → cloud (no raw code)
```

**OLLAMA_BASE_URL** defaults to `http://localhost:11434`. Set it to point to a remote Ollama instance if running Ollama on a separate machine or in a container.

---

## Provider Selection Strategy

| Scenario | Recommendation |
|---|---|
| High-volume batch processing | DeepSeek — lowest cost per token |
| Sensitive / private data | Ollama — local, no data leaves machine |
| Complex analysis, code understanding | Claude Sonnet |
| Planning, final decisions | Claude Opus |
| Fast extraction, parsing | Claude Haiku or DeepSeek |
| Mixed pipeline: cost + quality | DeepSeek for workers, Claude for reasoning steps |

The main strength of mcp-conveer is that you can assign a different provider per agent step, so the choice is not global — it is per task:

```yaml
agents:
  - name: task-reader
    model: deepseek/deepseek-chat    # cheap: just extract key terms
  - name: callgraph-builder
    model: ollama/llama3             # local: reads source code
  - name: incident-finder
    model: claude-sonnet-4-6         # quality: needs reasoning
  - name: problem-solver
    model: claude-opus-4-7           # best: final fix must be correct
```

---

## Adding a New Provider

1. Create `internal/provider/myprovider.go`:

```go
package provider

import "context"

type MyProvider struct {
    model  string
    apiKey string
}

func NewMyProvider(model string) *MyProvider {
    return &MyProvider{
        model:  model,
        apiKey: os.Getenv("MY_PROVIDER_API_KEY"),
    }
}

func (p *MyProvider) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
    // Call your API and return the raw response text.
}
```

2. Add routing in `cmd/worker/main.go` (`newProvider` function):

```go
func newProvider(model string) provider.LLMProvider {
    prefix, name := provider.ModelPrefix(model)
    switch prefix {
    case "ollama":
        return provider.NewOllama(name, os.Getenv("OLLAMA_BASE_URL"))
    case "deepseek":
        return provider.NewDeepSeek(name)
    case "myprovider":                     // add your prefix
        return provider.NewMyProvider(name)
    default:
        return provider.NewAnthropic(model)
    }
}
```

3. Use in config:

```yaml
agents:
  - name: my-agent
    model: myprovider/my-model-id
```
