package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Provider string `mapstructure:"provider"`
	Context  struct {
		Level string `mapstructure:"level"`
	} `mapstructure:"context"`
	Ollama struct {
		Model string `mapstructure:"model"`
		Host  string `mapstructure:"host"`
	} `mapstructure:"ollama"`
}

var AppConfig Config

func Init() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("$HOME/.config/huh")
	viper.AddConfigPath(".")

	// Defaults
	viper.SetDefault("provider", "ollama")
	viper.SetDefault("context.level", "basic")
	viper.SetDefault("ollama.model", "llama3:8b")
	viper.SetDefault("ollama.host", "http://localhost:11434")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error if desired or create default
		} else {
			fmt.Printf("Error reading config file: %s\n", err)
		}
	}

	if err := viper.Unmarshal(&AppConfig); err != nil {
		fmt.Printf("Unable to decode into struct: %v\n", err)
	}
}
