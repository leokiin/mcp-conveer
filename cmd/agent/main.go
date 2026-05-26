// cmd/agent runs a single agent as a standalone MCP server over stdio.
//
// Usage:
//
//	./agent --name task-reader --config pipeline.yaml
//
// The agent is loaded from the config file by name and served via MCP stdio
// transport. Any MCP client (Claude Code, n8n, custom tooling) can connect to it.
//
// Claude Code example (~/.claude/settings.json):
//
//	"mcpServers": {
//	  "task-reader": {
//	    "command": "/path/to/agent",
//	    "args": ["--name", "task-reader", "--config", "/path/to/pipeline.yaml"]
//	  }
//	}
package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/leokiin/mcp-conveer/internal/agent"
	"github.com/leokiin/mcp-conveer/internal/builder"
	"github.com/leokiin/mcp-conveer/internal/config"
)

func main() {
	name := flag.String("name", "", "Agent name as defined in config (required)")
	cfgPath := flag.String("config", "", "Path to conveer.yaml (required)")
	flag.Parse()

	if *name == "" || *cfgPath == "" {
		flag.Usage()
		os.Exit(1)
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	def := cfg.AgentByName(*name)
	if def == nil {
		log.Fatalf("agent %q not found in config", *name)
	}

	cfgDir := filepath.Dir(*cfgPath)
	a, err := builder.NewAgent(*def, cfgDir)
	if err != nil {
		log.Fatalf("build agent: %v", err)
	}

	srv := agent.NewMCPServer(a)
	if err := srv.ServeStdio(); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
