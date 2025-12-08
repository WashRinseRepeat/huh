package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

//go:embed config.example.yaml
var defaultConfigFile []byte

type ProviderConfig struct {
	Type   string            `mapstructure:"type" yaml:"type"`
	Params map[string]string `mapstructure:"params" yaml:"params"`
}

type Config struct {
	DefaultProvider string                    `mapstructure:"default_provider" yaml:"default_provider"`
	Context         map[string]string         `mapstructure:"context" yaml:"context"`
	Providers       map[string]ProviderConfig `mapstructure:"providers" yaml:"providers"`
}

var AppConfig Config

func Init() {
	configDir, err := os.UserConfigDir()
	if err != nil {
		fmt.Printf("Error getting user config dir: %v\n", err)
		configDir = "."
	}
	huhDir := filepath.Join(configDir, "huh")
	configPath := filepath.Join(huhDir, "config.yaml")

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(huhDir)
	viper.AddConfigPath(".")

	// Defaults
	viper.SetDefault("default_provider", "ollama")
	viper.SetDefault("context", map[string]string{"level": "basic"})

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; create default
			createDefaultConfig(configPath)
			if err := viper.ReadInConfig(); err != nil {
				fmt.Printf("Error reading newly created config file: %s\n", err)
			}
		} else {
			fmt.Printf("Error reading config file: %s\n", err)
		}
	}

	if err := viper.Unmarshal(&AppConfig); err != nil {
		fmt.Printf("Unable to decode into struct: %v\n", err)
	}
}

func createDefaultConfig(path string) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("Error creating config directory: %v\n", err)
		return
	}

	if err := os.WriteFile(path, defaultConfigFile, 0644); err != nil {
		fmt.Printf("Error writing default config file: %v\n", err)
		return
	}
	fmt.Printf("Created default config file at %s\n", path)
}
