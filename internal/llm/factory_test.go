package llm

import (
	"testing"

	"huh/internal/config"
)

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
