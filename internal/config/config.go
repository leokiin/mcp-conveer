package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the top-level structure of conveer.yaml.
type Config struct {
	TeamLead TeamLead       `yaml:"teamlead"`
	Workflow []WorkflowStep `yaml:"workflow"`
	Agents   []AgentDef     `yaml:"agents"`
}

// TeamLead defines the orchestrator LLM used in routed mode.
type TeamLead struct {
	Model    string `yaml:"model"`
	Role     string `yaml:"role"`
	RoleFile string `yaml:"role_file"`
	Mode     string `yaml:"mode"` // "sequential" or "routed"
}

// WorkflowStep is one step in the sequential workflow graph.
type WorkflowStep struct {
	Step      string   `yaml:"step"`
	Agent     string   `yaml:"agent"`
	DependsOn []string `yaml:"depends_on"`
	Loop      *Loop    `yaml:"loop"`
}

// Loop describes a repeated group of steps with a termination condition.
type Loop struct {
	MaxIterations int            `yaml:"max_iterations"`
	Until         string         `yaml:"until"` // e.g. "reviewer.approved == true"
	Steps         []WorkflowStep `yaml:"steps"`
}

// AgentDef defines an agent in the agents list.
type AgentDef struct {
	Name     string `yaml:"name"`
	Model    string `yaml:"model"`
	Role     string `yaml:"role"`
	RoleFile string `yaml:"role_file"`
	// Type selects the agent implementation: "" or "llm" (default), "mcp-client".
	Type string `yaml:"type"`
	// mcp-client fields: command to launch the MCP server process, tool name to invoke,
	// and the argument key under which the task string is passed (default: "query").
	Command   string `yaml:"command"`
	Tool      string `yaml:"tool"`
	InputKey  string `yaml:"input_key"`
	// InputStep, if set, passes only the named step's JSON output as the tool argument
	// instead of the full enriched task string. Used to pass a clean value to external
	// MCP tools that expect a short, structured input (e.g. a library name for Context7).
	InputStep  string `yaml:"input_step"`
	// InputField, used together with InputStep, extracts a single string field from the
	// step's JSON output instead of passing the whole JSON object.
	// Example: input_step: extract-lib, input_field: library → passes "gin" from {"library":"gin"}.
	InputField string `yaml:"input_field"`
	// ExtraArgs are additional fixed arguments merged into the MCP tool call alongside the
	// main input_key argument. Useful for tools that require multiple required parameters
	// (e.g. Context7 resolve-library-id needs both "libraryName" and "query").
	ExtraArgs map[string]string `yaml:"extra_args"`

	// file-writer fields: step whose result contains the content, field name within
	// that result, and the output filename under docs/plan/{track_id}/.
	ContentStep  string `yaml:"content_step"`
	ContentField string `yaml:"content_field"`
	Filename     string `yaml:"filename"`

	// file-reader fields: step and field holding the path, plus read mode.
	// mode: "read_file" returns {"content":"..."}, "list_directory" returns {"entries":[...]}.
	// A missing path returns empty content/entries instead of an error.
	PathStep  string `yaml:"path_step"`
	PathField string `yaml:"path_field"`
	ReadMode  string `yaml:"read_mode"`

	// multi-file-reader fields: step and field holding []string of absolute paths.
	// MaxFiles caps the number of files read (default 10).
	// MaxLines caps lines per file (default 200); truncated files are marked truncated:true.
	FilesStep  string `yaml:"files_step"`
	FilesField string `yaml:"files_field"`
	MaxFiles   int    `yaml:"max_files"`
	MaxLines   int    `yaml:"max_lines"`
}

// Load reads and parses a YAML config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	return &cfg, nil
}

// AgentByName returns the AgentDef with the given name, or nil if not found.
func (c *Config) AgentByName(name string) *AgentDef {
	for i := range c.Agents {
		if c.Agents[i].Name == name {
			return &c.Agents[i]
		}
	}
	return nil
}
