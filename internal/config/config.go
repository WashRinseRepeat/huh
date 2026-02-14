package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"
)

//go:embed config.example.yaml
var defaultConfigFile []byte

type ProviderConfig struct {
	Type   string            `mapstructure:"type" yaml:"type"`
	Params map[string]string `mapstructure:"params" yaml:"params"`
}

type Config struct {
	DefaultProvider string                    `mapstructure:"default_provider" yaml:"default_provider"`
	SystemPrompt    string                    `mapstructure:"system_prompt" yaml:"system_prompt"`
	Context         map[string]string         `mapstructure:"context" yaml:"context"`
	Providers       map[string]ProviderConfig `mapstructure:"providers" yaml:"providers"`
}

var AppConfig Config

func Init() {
	var huhDir string
	configPath, err := GetConfigLocation()
	if err != nil {
		fmt.Printf("Error getting config location: %v\n", err)
		huhDir = "huh"
	} else {
		huhDir = filepath.Dir(configPath)
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(huhDir)
	viper.AddConfigPath(".")

	// Defaults
	viper.SetDefault("default_provider", "ollama")
	viper.SetDefault("context", map[string]string{"level": "basic"})
	viper.SetDefault("system_prompt", "") // Default handled in code if empty

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; the setup wizard will handle creation
		} else {
			fmt.Printf("Error reading config file: %s\n", err)
		}
	}

	if err := viper.Unmarshal(&AppConfig); err != nil {
		fmt.Printf("Unable to decode into struct: %v\n", err)
	}
}

func GetConfigLocation() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("error getting user config dir: %w", err)
	}
	huhDir := filepath.Join(configDir, "huh")
	return filepath.Join(huhDir, "config.yaml"), nil
}

func ConfigExists() bool {
	path, err := GetConfigLocation()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

func WriteConfig(cfg Config) error {
	path, err := GetConfigLocation()
	if err != nil {
		return fmt.Errorf("error getting config location: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creating config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("error marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	return nil
}

func Reload() {
	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("Error reading config file: %s\n", err)
	}
	if err := viper.Unmarshal(&AppConfig); err != nil {
		fmt.Printf("Unable to decode into struct: %v\n", err)
	}
}
