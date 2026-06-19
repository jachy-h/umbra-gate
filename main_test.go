package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jachy-h/umbra-gate/config"
)

func TestPrintBannerShowsUnifiedProviderList(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test")

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg, err := config.Load(writeTempConfigFile(t, configPath, `
listen: "127.0.0.1:4141"
providers:
  anthropic:
    type: anthropic
    base_url: https://api.anthropic.com
    api_key: literal-key
  openai:
    type: openai
    base_url: https://api.openai.com
    api_key: ${OPENAI_API_KEY}
  zen:
    base_url: https://opencode.ai/api
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	var buf bytes.Buffer
	printBanner(&buf, cfg, t.TempDir())
	output := buf.String()

	if !strings.Contains(output, "  ▶ Providers  (3):") {
		t.Fatalf("expected unified providers header, got:\n%s", output)
	}
	if strings.Contains(output, "Passthrough") {
		t.Fatalf("expected no separate passthrough section, got:\n%s", output)
	}
	for _, want := range []string{
		"anthropic",
		"https://api.anthropic.com",
		"(anthropic)",
		"openai",
		"https://api.openai.com",
		"(openai)",
		"zen",
		"https://opencode.ai/api",
		"(passthrough)",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected banner to contain %q, got:\n%s", want, output)
		}
	}
}

func TestStartupProviderLabels(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg, err := config.Load(writeTempConfigFile(t, configPath, `
providers:
  openai:
    type: openai
    base_url: https://api.openai.com
    api_key: literal-key
  zen:
    base_url: https://opencode.ai/api
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	labels := startupProviderLabels(startupProviderRows(cfg))
	joined := strings.Join(labels, ",")
	if joined != "openai(openai),zen(passthrough)" {
		t.Fatalf("labels = %q", joined)
	}
}

func TestEnsureConfigFileWritesDefaultConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := ensureConfigFile(path); err != nil {
		t.Fatalf("ensureConfigFile: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	text := string(data)
	for _, want := range []string{"listen: 127.0.0.1:4141", "github-copilot:", "https://api.githubcopilot.com", "openai:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("default config missing %q:\n%s", want, text)
		}
	}
}

func writeTempConfigFile(t *testing.T, path, contents string) string {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}
