package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Valid(t *testing.T) {
	yaml := `
agents:
  - name: scanner
    model: ollama/llama3
    role: you are a scanner
  - name: reviewer
    model: deepseek/deepseek-v4-flash
workflow:
  - agent: scanner
  - agent: reviewer
    depends_on: [scanner]
`
	f := writeTempYAML(t, yaml)
	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Agents) != 2 {
		t.Errorf("want 2 agents, got %d", len(cfg.Agents))
	}
	if len(cfg.Workflow) != 2 {
		t.Errorf("want 2 workflow steps, got %d", len(cfg.Workflow))
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	f := writeTempYAML(t, ":: invalid yaml :")
	_, err := Load(f)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestAgentByName_Found(t *testing.T) {
	cfg := &Config{
		Agents: []AgentDef{
			{Name: "scanner"},
			{Name: "reviewer"},
		},
	}
	got := cfg.AgentByName("reviewer")
	if got == nil {
		t.Fatal("expected to find reviewer")
	}
	if got.Name != "reviewer" {
		t.Errorf("wrong agent: %s", got.Name)
	}
}

func TestAgentByName_NotFound(t *testing.T) {
	cfg := &Config{Agents: []AgentDef{{Name: "scanner"}}}
	if cfg.AgentByName("missing") != nil {
		t.Error("expected nil for unknown agent")
	}
}

func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(f, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return f
}
