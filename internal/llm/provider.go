package llm

import "context"

type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
}

func (t TokenUsage) TotalTokens() int {
	return t.PromptTokens + t.CompletionTokens
}

type LLM interface {
	Name() string
	Query(ctx context.Context, systemPrompt string, userQuery string) (string, TokenUsage, error)
}
