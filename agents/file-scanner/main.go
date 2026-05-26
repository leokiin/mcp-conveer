package main

import (
	"log"
	"os"

	"github.com/leokiin/mcp-conveer/internal/agent"
	"github.com/leokiin/mcp-conveer/internal/agents/filescanner"
	"github.com/leokiin/mcp-conveer/internal/builder"
)

func main() {
	model := os.Getenv("AGENT_MODEL")
	if model == "" {
		model = "claude-haiku-4-5"
	}

	srv := agent.NewMCPServer(filescanner.New(builder.NewProvider(model)))
	log.Printf("file-scanner starting (model: %s)", model)
	if err := srv.ServeStdio(); err != nil {
		log.Fatal(err)
	}
}
