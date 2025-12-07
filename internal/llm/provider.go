package llm

import "context"

type LLM interface {
	Name() string
	Query(ctx context.Context, systemPrompt string, userQuery string) (string, error)
}
