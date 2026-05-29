package main

import (
	"log"
	"net/http"

	"find-restaurants/internal/agent"
	"find-restaurants/internal/api"
	"find-restaurants/internal/config"
	"find-restaurants/internal/exa"
	"find-restaurants/internal/llm"
)

func main() {
	cfg := config.Load()

	llmClient := llm.NewClient(cfg.LLM)
	exaClient := exa.NewClient(cfg.ExaAPIKey)
	workflow := agent.NewWorkflow(llmClient, exaClient, cfg.Workflow)
	server := api.NewServer(workflow)

	addr := cfg.Host + ":" + cfg.Port
	log.Printf("find-food API listening on http://%s", addr)
	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		log.Fatal(err)
	}
}
