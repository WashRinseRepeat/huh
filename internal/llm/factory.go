package llm

import (
	"fmt"
	"os"

	"github.com/WashRinseRepeat/huh/internal/config"
)

// resolveAPIKey returns an API key for a provider, preferring the value from
// the environment variable named by params["api_key_env"] if it is set and
// non-empty. Falls back to params["api_key"]. Keeps secrets out of the YAML
// file when api_key_env is used.
func resolveAPIKey(params map[string]string) string {
	if envName := params["api_key_env"]; envName != "" {
		if v := os.Getenv(envName); v != "" {
			return v
		}
	}
	return params["api_key"]
}

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
		apiKey := resolveAPIKey(providerConfig.Params)
		model := providerConfig.Params["model"]
		if apiKey == "" {
			return nil, fmt.Errorf("openai provider '%s' missing api_key (set api_key in config or api_key_env to an environment variable name)", name)
		}
		if model == "" {
			model = "gpt-4-turbo"
		}
		return NewOpenAIProvider(apiKey, model), nil

	case "openrouter":
		apiKey := resolveAPIKey(providerConfig.Params)
		model := providerConfig.Params["model"]
		if apiKey == "" {
			return nil, fmt.Errorf("openrouter provider '%s' missing api_key (set api_key in config or api_key_env to an environment variable name)", name)
		}
		if model == "" {
			model = "openai/gpt-3.5-turbo" // Default model for openrouter, just an example
		}
		return NewOpenRouterProvider(apiKey, model), nil

	default:
		return nil, fmt.Errorf("unsupported provider type: %s", providerConfig.Type)
	}
}
