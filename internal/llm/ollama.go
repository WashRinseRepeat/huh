package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type OllamaProvider struct {
	Host  string
	Model string
}

func NewOllamaProvider(host, model string) *OllamaProvider {
	return &OllamaProvider{
		Host:  host,
		Model: model,
	}
}

func (o *OllamaProvider) Name() string {
	return "ollama"
}

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	System string `json:"system"`
	Stream bool   `json:"stream"`
}

type ollamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

func (o *OllamaProvider) Query(ctx context.Context, systemPrompt string, userQuery string) (string, error) {
	reqBody := ollamaRequest{
		Model:  o.Model,
		Prompt: userQuery,
		System: systemPrompt,
		Stream: false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	apiURL := fmt.Sprintf("%s/api/generate", o.Host)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama API error: status %d", resp.StatusCode)
	}

	var startResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&startResp); err != nil {
		return "", err
	}

	return startResp.Response, nil
}
