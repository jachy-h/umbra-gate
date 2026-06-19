package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type ProviderType string

const (
	ProviderTypeOpenAI    ProviderType = "openai"
	ProviderTypeAnthropic ProviderType = "anthropic"
)

func (t ProviderType) Valid() bool {
	if t == "" {
		return true
	}
	switch t {
	case ProviderTypeOpenAI, ProviderTypeAnthropic:
		return true
	}
	return false
}

// ProviderConfig is the in-memory provider definition.
//
// APIKey holds the resolved (env-expanded) value used at request time.
// APIKeyRaw holds the original literal — including ${ENV} references — so we
// can round-trip through Save without leaking secrets into the YAML file.
type ProviderConfig struct {
	Type      ProviderType
	BaseURL   string
	APIKey    string
	APIKeyRaw string
}

// Config is the validated, in-memory representation of config.yaml.
//
// Mutating methods (Upsert/Delete/Save) and read methods are safe to call
// concurrently. The proxy reads providers via Provider/ProviderIDs while
// the dashboard mutates them.
type Config struct {
	mu        sync.RWMutex
	path      string
	listen    string
	providers map[string]ProviderConfig
}

// rawConfig matches the YAML file layout exactly. Used only for (un)marshal.
type rawConfig struct {
	Listen    string                       `yaml:"listen,omitempty"`
	Providers map[string]rawProviderConfig `yaml:"providers"`
}

type rawProviderConfig struct {
	Type    string `yaml:"type"`
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key"`
}

const defaultListen = "127.0.0.1:4141"

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	cfg := &Config{
		path:      path,
		listen:    raw.Listen,
		providers: map[string]ProviderConfig{},
	}
	if cfg.listen == "" {
		cfg.listen = defaultListen
	}
	for id, rp := range raw.Providers {
		p, err := buildProvider(rp)
		if err != nil {
			return nil, fmt.Errorf("provider %q: %w", id, err)
		}
		cfg.providers[id] = p
	}
	return cfg, nil
}

func buildProvider(rp rawProviderConfig) (ProviderConfig, error) {
	pt := ProviderType(strings.TrimSpace(rp.Type))
	if !pt.Valid() {
		return ProviderConfig{}, fmt.Errorf("unknown type %q (must be openai, anthropic, or empty)", rp.Type)
	}
	baseURL := strings.TrimSpace(rp.BaseURL)
	if baseURL == "" {
		return ProviderConfig{}, errors.New("base_url is required")
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if u, err := url.Parse(baseURL); err != nil || u.Scheme == "" || u.Host == "" {
		return ProviderConfig{}, fmt.Errorf("base_url is not a valid absolute URL: %q", rp.BaseURL)
	}
	rawKey := strings.TrimSpace(rp.APIKey)
	var resolved string
	if rawKey != "" {
		var err error
		resolved, err = expandEnv(rawKey)
		if err != nil {
			return ProviderConfig{}, err
		}
	}
	return ProviderConfig{
		Type:      pt,
		BaseURL:   baseURL,
		APIKey:    resolved,
		APIKeyRaw: rawKey,
	}, nil
}

var envRefPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// expandEnv replaces ${VAR} occurrences with their environment values and
// errors out if any referenced variable is unset (empty string is allowed
// only if literally provided, not via missing env).
func expandEnv(s string) (string, error) {
	var missing []string
	out := envRefPattern.ReplaceAllStringFunc(s, func(match string) string {
		name := match[2 : len(match)-1]
		val, ok := os.LookupEnv(name)
		if !ok {
			missing = append(missing, name)
			return match
		}
		return val
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("environment variable %s not set", strings.Join(missing, ", "))
	}
	return out, nil
}

func (c *Config) Path() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.path
}

func (c *Config) Listen() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.listen
}

func (c *Config) Provider(id string) (ProviderConfig, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	p, ok := c.providers[id]
	return p, ok
}

func (c *Config) ProviderIDs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ids := make([]string, 0, len(c.providers))
	for id := range c.providers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// UpsertProvider validates and stores a provider in memory. Caller must
// invoke Save separately to persist to disk.
func (c *Config) UpsertProvider(id string, p ProviderConfig) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("provider id is required")
	}
	validated, err := buildProvider(rawProviderConfig{
		Type:    string(p.Type),
		BaseURL: p.BaseURL,
		APIKey:  firstNonEmpty(p.APIKeyRaw, p.APIKey),
	})
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.providers[id] = validated
	return nil
}

func (c *Config) DeleteProvider(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.providers[id]; !ok {
		return fmt.Errorf("provider %q not found", id)
	}
	delete(c.providers, id)
	return nil
}

// Save atomically writes the current config to disk. Existing files are
// backed up to <path>.<timestamp>.bak before replacement.
//
// API keys that came from ${ENV} references are written back as the literal
// ${ENV} string, never the expanded secret.
func (c *Config) Save() error {
	c.mu.RLock()
	raw := rawConfig{
		Listen:    c.listen,
		Providers: map[string]rawProviderConfig{},
	}
	for id, p := range c.providers {
		raw.Providers[id] = rawProviderConfig{
			Type:    string(p.Type),
			BaseURL: p.BaseURL,
			APIKey:  p.APIKeyRaw,
		}
	}
	path := c.path
	c.mu.RUnlock()

	if path == "" {
		return errors.New("config path not set")
	}

	data, err := yaml.Marshal(&raw)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		original, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		backup := fmt.Sprintf("%s.%s.bak", path, time.Now().Format("20060102-150405"))
		if err := os.WriteFile(backup, original, 0o600); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
