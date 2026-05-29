package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Host      string
	Port      string
	ExaAPIKey string
	LLM       LLMConfig
	Workflow  WorkflowConfig
}

type LLMConfig struct {
	Provider string
	APIKey   string
	Endpoint string
	Model    string
	SiteURL  string
	AppName  string
}

type WorkflowConfig struct {
	MaxCandidates       int
	MaxResults          int
	ResearchConcurrency int
}

func Load() Config {
	LoadDotEnv(".env")
	LoadDotEnv("../.env")

	provider := getenv("LLM_PROVIDER", "")
	if provider == "" {
		if os.Getenv("OPENAI_API_KEY") != "" {
			provider = "openai"
		} else {
			provider = "openrouter"
		}
	}

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	endpoint := getenv("LLM_ENDPOINT", "https://openrouter.ai/api/v1/chat/completions")
	model := getenv("LLM_MODEL", "anthropic/claude-4-sonnet-20250522")

	if provider == "openai" {
		apiKey = os.Getenv("OPENAI_API_KEY")
		endpoint = getenv("LLM_ENDPOINT", "https://api.openai.com/v1/chat/completions")
		model = getenv("LLM_MODEL", "gpt-4o-mini")
	}

	return Config{
		Host:      getenv("HOST", "127.0.0.1"),
		Port:      getenv("PORT", "3000"),
		ExaAPIKey: os.Getenv("EXA_API_KEY"),
		LLM: LLMConfig{
			Provider: provider,
			APIKey:   apiKey,
			Endpoint: endpoint,
			Model:    model,
			SiteURL:  getenv("OPENROUTER_SITE_URL", "http://localhost:3000"),
			AppName:  getenv("OPENROUTER_APP_NAME", "find-food"),
		},
		Workflow: WorkflowConfig{
			MaxCandidates:       getenvInt("FIND_FOOD_MAX_CANDIDATES", 6),
			MaxResults:          getenvInt("FIND_FOOD_MAX_RESULTS", 5),
			ResearchConcurrency: getenvInt("FIND_FOOD_RESEARCH_CONCURRENCY", 3),
		},
	}
}

func LoadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		if key == "" || os.Getenv(key) != "" {
			continue
		}

		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		_ = os.Setenv(key, value)
	}
}

func getenv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getenvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
