package llm

import (
	"os"
	"testing"

	"github.com/WashRinseRepeat/huh/internal/config"
)

func TestResolveAPIKey(t *testing.T) {
	const envName = "HUH_TEST_API_KEY_RESOLVE"
	os.Unsetenv(envName)
	defer os.Unsetenv(envName)

	t.Run("plain api_key", func(t *testing.T) {
		got := resolveAPIKey(map[string]string{"api_key": "literal-key"})
		if got != "literal-key" {
			t.Errorf("got %q, want %q", got, "literal-key")
		}
	})

	t.Run("api_key_env overrides when env set", func(t *testing.T) {
		os.Setenv(envName, "from-env")
		defer os.Unsetenv(envName)
		got := resolveAPIKey(map[string]string{"api_key": "ignored", "api_key_env": envName})
		if got != "from-env" {
			t.Errorf("got %q, want %q", got, "from-env")
		}
	})

	t.Run("api_key_env falls back to api_key when env unset", func(t *testing.T) {
		os.Unsetenv(envName)
		got := resolveAPIKey(map[string]string{"api_key": "fallback", "api_key_env": envName})
		if got != "fallback" {
			t.Errorf("got %q, want %q", got, "fallback")
		}
	})

	t.Run("nothing set", func(t *testing.T) {
		got := resolveAPIKey(map[string]string{})
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

func TestNewProvider(t *testing.T) {
	// Setup mock config
	config.AppConfig.Providers = map[string]config.ProviderConfig{
		"ollama": {
			Type: "ollama",
			Params: map[string]string{
				"host":  "http://test:11434",
				"model": "test-model",
			},
		},
		"openai": {
			Type: "openai",
			Params: map[string]string{
				"api_key": "sk-test",
				"model":   "gpt-test",
			},
		},
	}
	config.AppConfig.DefaultProvider = "ollama"

	tests := []struct {
		name          string
		providerName  string
		wantErr       bool
		wantType      string
		expectedModel string
	}{
		{
			name:          "Default Provider (Ollama)",
			providerName:  "",
			wantErr:       false,
			wantType:      "ollama",
			expectedModel: "test-model",
		},
		{
			name:          "Explicit Ollama",
			providerName:  "ollama",
			wantErr:       false,
			wantType:      "ollama",
			expectedModel: "test-model",
		},
		{
			name:          "Explicit OpenAI",
			providerName:  "openai",
			wantErr:       false,
			wantType:      "openai",
			expectedModel: "gpt-test", // Check model to confirm params passed
		},
		{
			name:         "Unknown Provider",
			providerName: "missing",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewProvider(tt.providerName)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if got.Name() != tt.wantType {
					t.Errorf("NewProvider() Name = %v, want %v", got.Name(), tt.wantType)
				}
				// Verify params
				if o, ok := got.(*OllamaProvider); ok {
					if o.Model != tt.expectedModel {
						t.Errorf("OllamaProvider.Model = %v, want %v", o.Model, tt.expectedModel)
					}
				}
				if o, ok := got.(*OpenAIProvider); ok {
					if o.Model != tt.expectedModel {
						t.Errorf("OpenAIProvider.Model = %v, want %v", o.Model, tt.expectedModel)
					}
				}
			}
		})
	}
}
