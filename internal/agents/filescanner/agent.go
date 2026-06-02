package filescanner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/leokiin/mcp-conveer/internal/provider"
)

const systemPrompt = `You are a code reviewer. Analyze the provided file content and return a JSON object with this exact structure:
{
  "file": "<filename>",
  "issues": [
    {"severity": "error|warning|info", "line": <number or null>, "message": "<description>"}
  ],
  "summary": "<one-line summary of findings>"
}

Return only valid JSON, no markdown, no extra text.`

// ScanResult is the structured output from the file-scanner agent.
type ScanResult struct {
	File    string  `json:"file"`
	Issues  []Issue `json:"issues"`
	Summary string  `json:"summary"`
}

// Issue represents a single finding in a scanned file.
type Issue struct {
	Severity string `json:"severity"`
	Line     *int   `json:"line"`
	Message  string `json:"message"`
}

// Agent scans source files and returns structured findings.
type Agent struct {
	llm provider.LLMProvider
}

// New creates a file-scanner agent backed by the given LLM provider.
func New(llm provider.LLMProvider) *Agent {
	return &Agent{llm: llm}
}

func (a *Agent) Name() string { return "file-scanner" }

func (a *Agent) Handle(ctx context.Context, task string) (json.RawMessage, error) {
	clean := filepath.Clean(task)
	if !filepath.IsAbs(clean) {
		return nil, fmt.Errorf("filescanner: path must be absolute, got: %s", task)
	}

	content, err := os.ReadFile(clean)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", clean, err)
	}

	userPrompt := fmt.Sprintf("File: %s\n\nContent:\n%s", clean, string(content))

	raw, _, err := a.llm.Complete(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("llm: %w", err)
	}

	var result ScanResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("parse llm response: %w\nresponse was: %s", err, raw)
	}

	return json.Marshal(result)
}
