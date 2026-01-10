package usercontext

import (
	"huh/internal/config"
	"testing"
)

func TestGetContextMatchesConfig(t *testing.T) {
	// Mock config
	config.AppConfig.Context = map[string]string{
		"shell":      "zsh", // Override detected shell
		"custom_key": "custom_value",
	}

	ctx := GetContext()

	if ctx.Shell != "zsh" {
		t.Errorf("Expected shell to be overridden to zsh, got %s", ctx.Shell)
	}

	if val, ok := ctx.Custom["custom_key"]; !ok || val != "custom_value" {
		t.Errorf("Expected custom_key to be custom_value, got %v", val)
	}
}
