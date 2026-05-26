package provider

import "context"

// LLMProvider abstracts different LLM backends (Anthropic, Ollama, etc.).
type LLMProvider interface {
	Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// ModelPrefix extracts the provider prefix from a model string like "ollama/llama3".
// Returns ("ollama", "llama3") or ("", "claude-haiku-4-5").
func ModelPrefix(model string) (provider, name string) {
	for i, c := range model {
		if c == '/' {
			return model[:i], model[i+1:]
		}
	}
	return "", model
}
