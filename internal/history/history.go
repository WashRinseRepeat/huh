package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Item struct {
	Question       string    `json:"question"`
	Answer         string    `json:"answer"`
	ContextContent string    `json:"context_content"`
	ContextInfo    string    `json:"context_info"`
	Timestamp      time.Time `json:"timestamp"`
}

const (
	maxHistoryItems = 10
	historyFileName = "history.json"
)

func GetHistoryPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("error getting user config dir: %w", err)
	}
	return filepath.Join(configDir, "huh", historyFileName), nil
}

func Load() ([]Item, error) {
	path, err := GetHistoryPath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return []Item{}, nil
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading history file: %w", err)
	}

	var items []Item
	if err := json.Unmarshal(b, &items); err != nil {
		// If file is corrupt, return empty or error?
		// Let's return empty to recover gracefully
		return []Item{}, nil
	}

	return items, nil
}

func Add(newItem Item) error {
	items, err := Load()
	if err != nil {
		// If load fails, we start fresh
		items = []Item{}
	}

	// Avoid adding exact duplicates at the end (idempotency for "viewing" same item)
	if len(items) > 0 {
		last := items[len(items)-1]
		if last.Question == newItem.Question && last.Answer == newItem.Answer {
			return nil
		}
	}

	items = append(items, newItem)

	// Trim to max
	if len(items) > maxHistoryItems {
		items = items[len(items)-maxHistoryItems:]
	}

	return Save(items)
}

func Save(items []Item) error {
	path, err := GetHistoryPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creating config directory: %w", err)
	}

	b, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling history: %w", err)
	}

	if err := os.WriteFile(path, b, 0644); err != nil {
		return fmt.Errorf("error writing history file: %w", err)
	}

	return nil
}
