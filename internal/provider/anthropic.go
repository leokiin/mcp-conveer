package provider

import (
	"context"

	"github.com/anthropics/anthropic-sdk-go"
)

const defaultMaxTokens int64 = 8192

// AnthropicProvider calls the Claude API.
type AnthropicProvider struct {
	client anthropic.Client
	model  anthropic.Model
}

// NewAnthropic creates a provider for a Claude model.
// The API key is read from ANTHROPIC_API_KEY by the SDK automatically.
func NewAnthropic(model string) *AnthropicProvider {
	return &AnthropicProvider{
		client: anthropic.NewClient(),
		model:  anthropic.Model(model),
	}
}

func (p *AnthropicProvider) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	var system []anthropic.TextBlockParam
	if systemPrompt != "" {
		system = []anthropic.TextBlockParam{{Text: systemPrompt}}
	}
	msg, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     p.model,
		MaxTokens: defaultMaxTokens,
		System:    system,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
		},
	})
	if err != nil {
		return "", err
	}

	if len(msg.Content) == 0 {
		return "", nil
	}
	block := msg.Content[0]
	if block.Type == "text" {
		return block.Text, nil
	}
	return "", nil
}
