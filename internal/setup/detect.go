package setup

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// --- Ollama detection ---

type ollamaTagsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

type ollamaDetectMsg struct {
	models []string
	err    error
}

func detectOllama(host string) tea.Cmd {
	return func() tea.Msg {
		models, err := fetchOllamaModels(host)
		return ollamaDetectMsg{models: models, err: err}
	}
}

func fetchOllamaModels(host string) ([]string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(fmt.Sprintf("%s/api/tags", host))
	if err != nil {
		return nil, fmt.Errorf("could not reach Ollama at %s: %w", host, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama returned status %d", resp.StatusCode)
	}

	var tagsResp ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return nil, fmt.Errorf("error parsing Ollama response: %w", err)
	}

	var names []string
	for _, m := range tagsResp.Models {
		names = append(names, m.Name)
	}

	if len(names) == 0 {
		return nil, fmt.Errorf("Ollama is running but has no models installed")
	}

	return names, nil
}
