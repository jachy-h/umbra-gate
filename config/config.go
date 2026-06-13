package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type ProviderConfig struct {
	BaseURL  string `yaml:"base_url"`
	APIKey   string `yaml:"api_key"`
	Protocol string `yaml:"protocol"`
}

type StorageConfig struct {
	SavePrompt bool `yaml:"save_prompt"`
}

type Config struct {
	Listen    string                    `yaml:"listen"`
	Providers map[string]ProviderConfig `yaml:"providers"`
	Storage   StorageConfig             `yaml:"storage"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Listen == "" {
		cfg.Listen = "127.0.0.1:4141"
	}
	return &cfg, nil
}
