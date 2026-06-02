package provider

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// OllamaProvider calls a local Ollama server via its OpenAI-compatible API.
type OllamaProvider struct {
	baseURL string
	model   string
	client  *http.Client
}

// NewOllama creates a provider for an Ollama model.
// baseURL defaults to http://localhost:11434 if empty.
func NewOllama(model, baseURL string) *OllamaProvider {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &OllamaProvider{
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{Timeout: 90 * time.Second},
	}
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type ollamaChatResponse struct {
	Message         ollamaMessage `json:"message"`
	PromptEvalCount int           `json:"prompt_eval_count"`
	EvalCount       int           `json:"eval_count"`
}

func (p *OllamaProvider) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, Usage, error) {
	messages := []ollamaMessage{}
	if systemPrompt != "" {
		messages = append(messages, ollamaMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, ollamaMessage{Role: "user", Content: userPrompt})

	var result ollamaChatResponse
	if err := doPost(ctx, p.client, p.baseURL+"/api/chat", nil,
		ollamaChatRequest{Model: p.model, Messages: messages, Stream: false},
		&result,
	); err != nil {
		return "", Usage{}, fmt.Errorf("ollama: %w", err)
	}
	usage := Usage{
		InputTokens:  result.PromptEvalCount,
		OutputTokens: result.EvalCount,
	}
	return result.Message.Content, usage, nil
}
