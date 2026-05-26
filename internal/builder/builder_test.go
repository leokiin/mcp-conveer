package builder_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/leokiin/mcp-conveer/internal/builder"
	"github.com/leokiin/mcp-conveer/internal/config"
	"github.com/leokiin/mcp-conveer/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProvider_Routing(t *testing.T) {
	cases := []struct {
		model    string
		wantType interface{}
	}{
		{"ollama/llama3", &provider.OllamaProvider{}},
		{"deepseek/deepseek-v4-flash", &provider.DeepSeekProvider{}},
		{"claude-sonnet-4-6", &provider.AnthropicProvider{}},
		{"", &provider.AnthropicProvider{}},
	}
	for _, tc := range cases {
		p := builder.NewProvider(tc.model)
		assert.IsType(t, tc.wantType, p, "model %q", tc.model)
	}
}

func TestResolveRole_Inline(t *testing.T) {
	got, err := builder.ResolveRole("my prompt", "", ".")
	require.NoError(t, err)
	assert.Equal(t, "my prompt", got)
}

func TestResolveRole_Empty(t *testing.T) {
	got, err := builder.ResolveRole("", "", ".")
	require.NoError(t, err)
	assert.Equal(t, "", got)
}

func TestResolveRole_File(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "role.md"), []byte("from file"), 0600))

	got, err := builder.ResolveRole("", "role.md", dir)
	require.NoError(t, err)
	assert.Equal(t, "from file", got)
}

func TestResolveRole_FileOverridesInline(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "role.md"), []byte("from file"), 0600))

	got, err := builder.ResolveRole("inline", "role.md", dir)
	require.NoError(t, err)
	assert.Equal(t, "from file", got)
}

func TestResolveRole_FileNotFound(t *testing.T) {
	_, err := builder.ResolveRole("", "missing.md", t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing.md")
}

func TestNewAgent_MCPClient(t *testing.T) {
	def := config.AgentDef{
		Name:    "doc-fetcher",
		Type:    "mcp-client",
		Command: "npx -y @ctx/mcp",
		Tool:    "get-docs",
	}
	a, err := builder.NewAgent(def, ".")
	require.NoError(t, err)
	assert.Equal(t, "doc-fetcher", a.Name())
}

func TestNewAgent_Generic(t *testing.T) {
	def := config.AgentDef{
		Name:  "reviewer",
		Model: "ollama/llama3",
		Role:  "you are a reviewer",
	}
	a, err := builder.NewAgent(def, ".")
	require.NoError(t, err)
	assert.Equal(t, "reviewer", a.Name())
}

func TestNewAgent_RoleFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "role.md"), []byte("file role"), 0600))

	def := config.AgentDef{
		Name:     "scanner",
		Model:    "ollama/llama3",
		RoleFile: "role.md",
	}
	a, err := builder.NewAgent(def, dir)
	require.NoError(t, err)
	assert.Equal(t, "scanner", a.Name())
}

func TestNewAgent_MissingRoleFile(t *testing.T) {
	def := config.AgentDef{
		Name:     "scanner",
		Model:    "ollama/llama3",
		RoleFile: "missing.md",
	}
	_, err := builder.NewAgent(def, t.TempDir())
	require.Error(t, err)
}
