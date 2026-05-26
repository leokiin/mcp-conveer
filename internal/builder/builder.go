package builder

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/leokiin/mcp-conveer/internal/agent"
	"github.com/leokiin/mcp-conveer/internal/agents/generic"
	"github.com/leokiin/mcp-conveer/internal/agents/mcpclient"
	"github.com/leokiin/mcp-conveer/internal/config"
	"github.com/leokiin/mcp-conveer/internal/provider"
)

// NewProvider creates the right LLM provider based on the model string prefix.
// Prefix "ollama/" → local Ollama, "deepseek/" → DeepSeek API, no prefix → Anthropic.
func NewProvider(model string) provider.LLMProvider {
	prefix, name := provider.ModelPrefix(model)
	switch prefix {
	case "ollama":
		return provider.NewOllama(name, os.Getenv("OLLAMA_BASE_URL"))
	case "deepseek":
		return provider.NewDeepSeek(name)
	default:
		return provider.NewAnthropic(model)
	}
}

// NewAgent builds the right agent implementation based on AgentDef.Type.
func NewAgent(def config.AgentDef, cfgDir string) (agent.Agent, error) {
	if def.Type == "mcp-client" {
		return mcpclient.New(def.Name, def.Command, def.Tool, def.InputKey, def.ExtraArgs), nil
	}
	role, err := ResolveRole(def.Role, def.RoleFile, cfgDir)
	if err != nil {
		return nil, err
	}
	if role == "" {
		log.Printf("warn: agent %s has empty role — system prompt will be blank", def.Name)
	}
	return generic.New(def.Name, role, NewProvider(def.Model)), nil
}

// ResolveRole returns the system prompt from inline role or role_file.
// role_file paths are resolved relative to cfgDir.
func ResolveRole(role, roleFile, cfgDir string) (string, error) {
	if roleFile != "" {
		path := filepath.Join(cfgDir, roleFile)
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read role file %s: %w", path, err)
		}
		return string(data), nil
	}
	return role, nil
}
