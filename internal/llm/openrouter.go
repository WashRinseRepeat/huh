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
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

func (o *OpenRouterProvider) Query(ctx context.Context, systemPrompt string, userQuery string) (string, TokenUsage, error) {
	reqBody := openRouterRequest{
		Model: o.Model,
		Messages: []openRouterMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userQuery},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", TokenUsage{}, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", TokenUsage{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.APIKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", TokenUsage{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", TokenUsage{}, fmt.Errorf("openrouter API error: status %d", resp.StatusCode)
	}

	var parsedResp openRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsedResp); err != nil {
		return "", TokenUsage{}, err
	}

	if len(parsedResp.Choices) == 0 {
		return "", TokenUsage{}, fmt.Errorf("openrouter returned no choices")
	}

	usage := TokenUsage{
		PromptTokens:     parsedResp.Usage.PromptTokens,
		CompletionTokens: parsedResp.Usage.CompletionTokens,
	}
	return parsedResp.Choices[0].Message.Content, usage, nil
}
