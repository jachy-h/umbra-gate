package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server     Server     `yaml:"server"`
	DB         DB         `yaml:"db"`
	Aggregator Aggregator `yaml:"aggregator"`
	Admin      Admin      `yaml:"admin"`
}

type Server struct {
	Addr string `yaml:"addr"`
}

type DB struct {
	Path string `yaml:"path"`
}

type Aggregator struct {
	IntervalSeconds int `yaml:"interval_seconds"`
}

type Admin struct {
	Token string `yaml:"token"`
}

func Default() Config {
	return Config{
		Server:     Server{Addr: ":8787"},
		DB:         DB{Path: "gateway.db"},
		Aggregator: Aggregator{IntervalSeconds: 60},
		Admin:      Admin{Token: ""},
	}
}

func Load(path string) (Config, error) {
	c := Default()
	if path == "" {
		return c, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	if err := yaml.Unmarshal(b, &c); err != nil {
		return c, err
	}
	return c, nil
}
