package llm

import (
	"fmt"
	"huh/internal/config"
)

func NewProvider(name string) (LLM, error) {
	// If name is empty, use default from config
	if name == "" {
		name = config.AppConfig.DefaultProvider
	}

	providerConfig, ok := config.AppConfig.Providers[name]
	if !ok {
		return nil, fmt.Errorf("provider '%s' not found in configuration", name)
	}

	switch providerConfig.Type {
	case "ollama":
		host := providerConfig.Params["host"]
		model := providerConfig.Params["model"]
		if host == "" {
			host = "http://localhost:11434"
		}
		if model == "" {
			model = "llama3:8b"
		}
		return NewOllamaProvider(host, model), nil

	case "openai":
		apiKey := providerConfig.Params["api_key"]
		model := providerConfig.Params["model"]
		if apiKey == "" {
			return nil, fmt.Errorf("openai provider '%s' missing api_key", name)
		}
		if model == "" {
			model = "gpt-4-turbo"
		}
		return NewOpenAIProvider(apiKey, model), nil

	default:
		return nil, fmt.Errorf("unsupported provider type: %s", providerConfig.Type)
	}
}
