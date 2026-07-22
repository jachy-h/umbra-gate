package config

import (
	"os"
	"path/filepath"

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

// AppName is the directory name (relative to the user home directory) that
// holds runtime state: the SQLite database and the editable config.yaml.
const AppName = "umbragate"

// AppDir returns the directory used to store the database and config across
// operating systems. It resolves to ~/.umbragate on macOS / Linux and to
// %USERPROFILE%\.umbragate on Windows via os.UserHomeDir.
func AppDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "."+AppName), nil
}

// DefaultConfigPath returns the canonical location of the user config file.
func DefaultConfigPath() (string, error) {
	d, err := AppDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "config.yaml"), nil
}

// DefaultDBPath returns the canonical location of the SQLite database.
func DefaultDBPath() (string, error) {
	d, err := AppDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, AppName+".db"), nil
}

func Default() Config {
	dbPath, _ := DefaultDBPath()
	return Config{
		Server:     Server{Addr: ":8787"},
		DB:         DB{Path: dbPath},
		Aggregator: Aggregator{IntervalSeconds: 60},
		Admin:      Admin{Token: ""},
	}
}

// Load resolves the configuration.
//
// If path is empty, the config file at ~/.umbragate/config.yaml is used. When
// that file does not yet exist, it is created from the built-in defaults so
// the user can tweak it on subsequent runs. Values omitted from the file are
// filled with the defaults (including the database location under the same
// app directory).
func Load(path string) (Config, error) {
	c := Default()

	appDir, err := AppDir()
	if err != nil {
		return c, err
	}
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return c, err
	}

	if path == "" {
		path = filepath.Join(appDir, "config.yaml")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := writeDefault(path); err != nil {
				return c, err
			}
			return c, nil
		} else if err != nil {
			return c, err
		}
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	if err := yaml.Unmarshal(b, &c); err != nil {
		return c, err
	}
	if c.DB.Path == "" {
		dbPath, _ := DefaultDBPath()
		c.DB.Path = dbPath
	}
	return c, nil
}

func writeDefault(path string) error {
	c := Default()
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}