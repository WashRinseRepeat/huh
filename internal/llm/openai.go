package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type OpenAIProvider struct {
	APIKey string
	Model  string
}

func NewOpenAIProvider(apiKey, model string) *OpenAIProvider {
	return &OpenAIProvider{
		APIKey: apiKey,
		Model:  model,
	}
}

func (o *OpenAIProvider) Name() string {
	return "openai"
}

type openAIRequest struct {
	Model    string                  `json:"model"`
	Messages []openAIMessage         `json:"messages"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
}

func (o *OpenAIProvider) Query(ctx context.Context, systemPrompt string, userQuery string) (string, error) {
	reqBody := openAIRequest{
		Model: o.Model,
		Messages: []openAIMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userQuery},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.APIKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openai API error: status %d", resp.StatusCode)
	}

	var parsedResp openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsedResp); err != nil {
		return "", err
	}

	if len(parsedResp.Choices) == 0 {
		return "", fmt.Errorf("openai returned no choices")
	}

	return parsedResp.Choices[0].Message.Content, nil
}
