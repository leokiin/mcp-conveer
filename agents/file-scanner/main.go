package main

import (
	"log"
	"os"

	"github.com/leokiin/mcp-conveer/internal/agent"
	"github.com/leokiin/mcp-conveer/internal/agents/filescanner"
	"github.com/leokiin/mcp-conveer/internal/provider"
)

func main() {
	model := os.Getenv("AGENT_MODEL")
	if model == "" {
		model = "claude-haiku-4-5"
	}

	prefix, name := provider.ModelPrefix(model)

	var llm provider.LLMProvider
	switch prefix {
	case "ollama":
		llm = provider.NewOllama(name, os.Getenv("OLLAMA_BASE_URL"))
	default:
		llm = provider.NewAnthropic(model)
	}

	srv := agent.NewMCPServer(filescanner.New(llm))
	log.Printf("file-scanner starting (model: %s)", model)
	if err := srv.ServeStdio(); err != nil {
		log.Fatal(err)
	}
}
