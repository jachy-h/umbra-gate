package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTempConfig(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

func TestLoadValidConfig(t *testing.T) {
	t.Setenv("VOLC_KEY", "vk-secret")
	path := writeTempConfig(t, `
listen: "127.0.0.1:5000"
providers:
  volcengine:
    type: openai
    base_url: https://ark.example.com/v3/
    api_key: ${VOLC_KEY}
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Listen() != "127.0.0.1:5000" {
		t.Errorf("listen = %q", cfg.Listen())
	}
	p, ok := cfg.Provider("volcengine")
	if !ok {
		t.Fatalf("provider missing")
	}
	if p.Type != ProviderTypeOpenAI {
		t.Errorf("type = %q, want %q", p.Type, ProviderTypeOpenAI)
	}
	if p.BaseURL != "https://ark.example.com/v3" {
		t.Errorf("base_url = %q (should trim trailing slash)", p.BaseURL)
	}
	if p.APIKey != "vk-secret" {
		t.Errorf("api_key = %q, want expanded value", p.APIKey)
	}
	if p.APIKeyRaw != "${VOLC_KEY}" {
		t.Errorf("api_key_raw = %q, want literal", p.APIKeyRaw)
	}
}

func TestLoadDefaultsListen(t *testing.T) {
	path := writeTempConfig(t, `
providers: {}
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Listen() != "127.0.0.1:4141" {
		t.Errorf("default listen = %q", cfg.Listen())
	}
}

func TestLoadFailsOnMissingEnv(t *testing.T) {
	os.Unsetenv("MISSING_KEY_XYZ")
	path := writeTempConfig(t, `
providers:
  p1:
    type: openai
    base_url: https://api.example.com
    api_key: ${MISSING_KEY_XYZ}
`)
	_, err := Load(path)
	if err == nil {
		t.Fatalf("expected error for missing env var")
	}
	if !strings.Contains(err.Error(), "MISSING_KEY_XYZ") {
		t.Errorf("error should mention env var name: %v", err)
	}
}

func TestLoadFailsOnUnknownType(t *testing.T) {
	path := writeTempConfig(t, `
providers:
  p1:
    type: bogus
    base_url: https://api.example.com
    api_key: x
`)
	_, err := Load(path)
	if err == nil {
		t.Fatalf("expected error for unknown type")
	}
}

func TestLoadFailsOnMissingFields(t *testing.T) {
	cases := map[string]string{
		"missing base_url": `
providers:
  p1:
    type: openai
    api_key: x
`,
		"invalid base_url": `
providers:
  p1:
    type: openai
    base_url: "://broken"
    api_key: x
`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeTempConfig(t, body)
			if _, err := Load(path); err == nil {
				t.Fatalf("expected error for %s", name)
			}
		})
	}
}

func TestLoadPassthroughDefaults(t *testing.T) {
	// type and api_key are optional; defaults to passthrough with empty type.
	path := writeTempConfig(t, `
providers:
  zen:
    base_url: https://opencode.ai/zen/v1
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	p, ok := cfg.Provider("zen")
	if !ok {
		t.Fatalf("provider missing")
	}
	if p.Type != "" {
		t.Errorf("type = %q, want empty (passthrough)", p.Type)
	}
	if p.BaseURL != "https://opencode.ai/zen/v1" {
		t.Errorf("base_url = %q", p.BaseURL)
	}
	if p.APIKey != "" {
		t.Errorf("api_key should be empty, got %q", p.APIKey)
	}
}

func TestLoadAcceptsAnthropicType(t *testing.T) {
	path := writeTempConfig(t, `
providers:
  ant:
    type: anthropic
    base_url: https://api.anthropic.com
    api_key: literal-key
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	p, _ := cfg.Provider("ant")
	if p.Type != ProviderTypeAnthropic {
		t.Errorf("type = %q", p.Type)
	}
	if p.APIKey != "literal-key" || p.APIKeyRaw != "literal-key" {
		t.Errorf("literal key handling wrong: api_key=%q raw=%q", p.APIKey, p.APIKeyRaw)
	}
}

func TestSaveRoundTripPreservesEnvRef(t *testing.T) {
	t.Setenv("VOLC_KEY", "vk-secret")
	path := writeTempConfig(t, `
listen: "127.0.0.1:5000"
providers:
  volcengine:
    type: openai
    base_url: https://ark.example.com/v3
    api_key: ${VOLC_KEY}
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "${VOLC_KEY}") {
		t.Errorf("saved file lost env reference:\n%s", data)
	}
	if strings.Contains(string(data), "vk-secret") {
		t.Errorf("saved file leaked expanded secret:\n%s", data)
	}
}

func TestUpsertProviderAndDelete(t *testing.T) {
	path := writeTempConfig(t, `
providers: {}
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	err = cfg.UpsertProvider("openai", ProviderConfig{
		Type:      ProviderTypeOpenAI,
		BaseURL:   "https://api.openai.com/",
		APIKey:    "sk-real",
		APIKeyRaw: "sk-real",
	})
	if err != nil {
		t.Fatalf("UpsertProvider: %v", err)
	}
	p, ok := cfg.Provider("openai")
	if !ok || p.BaseURL != "https://api.openai.com" || p.APIKey != "sk-real" {
		t.Errorf("upsert result: %+v ok=%v", p, ok)
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "openai") {
		t.Errorf("save missing provider:\n%s", data)
	}

	if err := cfg.DeleteProvider("openai"); err != nil {
		t.Fatalf("DeleteProvider: %v", err)
	}
	if _, ok := cfg.Provider("openai"); ok {
		t.Errorf("provider still present after delete")
	}
	if err := cfg.DeleteProvider("nonexistent"); err == nil {
		t.Errorf("delete nonexistent should error")
	}
}

func TestUpsertProviderRejectsInvalid(t *testing.T) {
	path := writeTempConfig(t, `providers: {}`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cases := map[string]ProviderConfig{
		"bad type":       {Type: "junk", BaseURL: "https://x", APIKey: "k", APIKeyRaw: "k"},
		"empty base_url": {Type: ProviderTypeOpenAI, APIKey: "k", APIKeyRaw: "k"},
		"bad base_url":   {Type: ProviderTypeOpenAI, BaseURL: "://x", APIKey: "k", APIKeyRaw: "k"},
	}
	for name, p := range cases {
		t.Run(name, func(t *testing.T) {
			if err := cfg.UpsertProvider("p", p); err == nil {
				t.Errorf("expected error for %s", name)
			}
		})
	}
	if err := cfg.UpsertProvider("", ProviderConfig{Type: ProviderTypeOpenAI, BaseURL: "https://x", APIKey: "k", APIKeyRaw: "k"}); err == nil {
		t.Errorf("empty id should error")
	}
}

func TestProviderIDsSorted(t *testing.T) {
	path := writeTempConfig(t, `
providers:
  zeta:
    type: openai
    base_url: https://a
    api_key: k
  alpha:
    type: openai
    base_url: https://b
    api_key: k
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	ids := cfg.ProviderIDs()
	if len(ids) != 2 || ids[0] != "alpha" || ids[1] != "zeta" {
		t.Errorf("ProviderIDs = %v, want [alpha zeta]", ids)
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	path := writeTempConfig(t, `providers: {}`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			cfg.Provider("x")
			cfg.ProviderIDs()
		}
		close(done)
	}()
	for i := 0; i < 100; i++ {
		_ = cfg.UpsertProvider("x", ProviderConfig{
			Type: ProviderTypeOpenAI, BaseURL: "https://x", APIKey: "k", APIKeyRaw: "k",
		})
	}
	<-done
}
