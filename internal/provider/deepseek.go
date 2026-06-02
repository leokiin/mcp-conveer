package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"
)

// DeepSeekProvider calls the DeepSeek API (OpenAI-compatible).
type DeepSeekProvider struct {
	model   string
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewDeepSeek creates a provider for a DeepSeek model.
// API key is read from DEEPSEEK_API_KEY env var.
func NewDeepSeek(model string) *DeepSeekProvider {
	return NewDeepSeekWithURL(model, os.Getenv("DEEPSEEK_API_KEY"), "https://api.deepseek.com")
}

// NewDeepSeekWithURL creates a provider with an explicit base URL and API key (for testing).
func NewDeepSeekWithURL(model, apiKey, baseURL string) *DeepSeekProvider {
	return &DeepSeekProvider{
		model:   model,
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 90 * time.Second},
	}
}

type deepSeekMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type deepSeekRequest struct {
	Model    string            `json:"model"`
	Messages []deepSeekMessage `json:"messages"`
}

type deepSeekResponse struct {
	Choices []struct {
		Message deepSeekMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

func (p *DeepSeekProvider) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, Usage, error) {
	messages := []deepSeekMessage{}
	if systemPrompt != "" {
		messages = append(messages, deepSeekMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, deepSeekMessage{Role: "user", Content: userPrompt})

	var result deepSeekResponse
	if err := doPost(ctx, p.client, p.baseURL+"/chat/completions",
		map[string]string{"Authorization": "Bearer " + p.apiKey},
		deepSeekRequest{Model: p.model, Messages: messages},
		&result,
	); err != nil {
		return "", Usage{}, fmt.Errorf("deepseek: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", Usage{}, fmt.Errorf("deepseek: empty response")
	}
	usage := Usage{
		InputTokens:  result.Usage.PromptTokens,
		OutputTokens: result.Usage.CompletionTokens,
	}
	return result.Choices[0].Message.Content, usage, nil
}
