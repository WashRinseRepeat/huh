package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type OpenRouterProvider struct {
	APIKey string
	Model  string
}

func NewOpenRouterProvider(apiKey, model string) *OpenRouterProvider {
	return &OpenRouterProvider{
		APIKey: apiKey,
		Model:  model,
	}
}

func (o *OpenRouterProvider) Name() string {
	return "openrouter"
}

type openRouterRequest struct {
	Model    string              `json:"model"`
	Messages []openRouterMessage `json:"messages"`
}

type openRouterMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openRouterResponse struct {
	Choices []struct {
		Message openRouterMessage `json:"message"`
	} `json:"choices"`
}

func (o *OpenRouterProvider) Query(ctx context.Context, systemPrompt string, userQuery string) (string, error) {
	reqBody := openRouterRequest{
		Model: o.Model,
		Messages: []openRouterMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userQuery},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.APIKey)
	// OpenRouter specific headers for ranking/stats (optional but recommended)
	// We can add these later if requested, or maybe add a "HTTP-Referer" and "X-Title" if we had app metadata.

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openrouter API error: status %d", resp.StatusCode)
	}

	var parsedResp openRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsedResp); err != nil {
		return "", err
	}

	if len(parsedResp.Choices) == 0 {
		return "", fmt.Errorf("openrouter returned no choices")
	}

	return parsedResp.Choices[0].Message.Content, nil
}
