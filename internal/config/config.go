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
	Storage    Storage    `yaml:"storage"`
	Logging    Logging    `yaml:"logging"`
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

type Storage struct {
	RequestLogsRetentionDays int `yaml:"request_logs_retention_days"`
	MaxDatabaseSizeMB        int `yaml:"max_database_size_mb"`
	LogPruneBatchSize        int `yaml:"log_prune_batch_size"`
	StatsRetentionDays       int `yaml:"stats_retention_days"`
	MaxLinkAttributes        int `yaml:"max_link_attributes"`
	MaxAttributeKeyLength    int `yaml:"max_attribute_key_length"`
	MaxAttributeValueLength  int `yaml:"max_attribute_value_length"`
}

type Logging struct {
	MaxSizeMB  int  `yaml:"max_size_mb"`
	MaxBackups int  `yaml:"max_backups"`
	MaxAgeDays int  `yaml:"max_age_days"`
	Compress   bool `yaml:"compress"`
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
		Storage: Storage{RequestLogsRetentionDays: 7, MaxDatabaseSizeMB: 1024, LogPruneBatchSize: 1000,
			StatsRetentionDays: 365, MaxLinkAttributes: 16, MaxAttributeKeyLength: 64, MaxAttributeValueLength: 256},
		Logging: Logging{MaxSizeMB: 50, MaxBackups: 7, MaxAgeDays: 7, Compress: true},
	}
}

// ResolvePath returns the effective config path without creating the file.
func ResolvePath(path string) (string, error) {
	if path != "" {
		return path, nil
	}
	return DefaultConfigPath()
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
	return os.WriteFile(path, []byte(defaultConfigYAML), 0o600)
}

const defaultConfigYAML = `# UmbraGate configuration. This file is created on first startup.
server:
  addr: ":8787" # Use 127.0.0.1:8787 to bind only locally.
db:
  path: "" # Empty uses ~/.umbragate/umbragate.db.
aggregator:
  interval_seconds: 60 # Hourly statistics aggregation interval; 0 disables the loop.
admin:
  token: "" # Set a long random token before exposing the admin UI outside localhost.

# Data retention and capacity controls. Zero disables a retention setting.
storage:
  request_logs_retention_days: 7 # Raw request audit log retention.
  max_database_size_mb: 1024 # At this size the oldest request logs are pruned.
  log_prune_batch_size: 1000 # Number of oldest request logs deleted for a size prune.
  stats_retention_days: 365 # Aggregated hourly statistics retention.
  max_link_attributes: 16 # Maximum configured statistics dimensions per Link.
  max_attribute_key_length: 64 # Maximum characters in an attribute key.
  max_attribute_value_length: 256 # Maximum characters in a serialized attribute value.

# Used by ` + "`umbragate start`" + `. Logs are at ~/.umbragate/umbragate.log.
logging:
  max_size_mb: 50 # Rotate when the active log reaches this size.
  max_backups: 7 # Keep at most this many rotated files.
  max_age_days: 7 # Delete rotated files older than this many days.
  compress: true # Gzip rotated files.
`
