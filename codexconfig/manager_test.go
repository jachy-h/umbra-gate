package codexconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyToTextEnablesGatewayProvider(t *testing.T) {
	raw := []byte(`model = "gpt-5.5"

[mcp_servers.node_repl]
command = "node"
`)

	out, err := ApplyToText(raw, ProviderInput{ID: "openai", Gateway: GatewayEnable, GatewayBaseURL: "http://127.0.0.1:4141"})
	if err != nil {
		t.Fatalf("ApplyToText() error = %v", err)
	}
	text := string(out)
	for _, want := range []string{
		`model = "gpt-5.5"`,
		`model_provider = "openai"`,
		`[mcp_servers.node_repl]`,
		`[model_providers.openai]`,
		`name = "Umbragate OpenAI"`,
		`base_url = "http://127.0.0.1:4141/a/codex/openai/v1"`,
		`env_key = "OPENAI_API_KEY"`,
		`wire_api = "responses"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q:\n%s", want, text)
		}
	}
}

func TestApplyToTextReplacesExistingProviderTable(t *testing.T) {
	raw := []byte(`model_provider = "openai"

[model_providers.openai]
name = "Old"
base_url = "https://old.example/v1"
env_key = "OLD_KEY"
wire_api = "chat"

[projects."/tmp"]
trust_level = "trusted"
`)

	out, err := ApplyToText(raw, ProviderInput{ID: "openai", Gateway: GatewayEnable, GatewayBaseURL: "http://127.0.0.1:4141"})
	if err != nil {
		t.Fatalf("ApplyToText() error = %v", err)
	}
	text := string(out)
	if strings.Contains(text, "https://old.example") || strings.Contains(text, "OLD_KEY") {
		t.Fatalf("old provider table was not replaced:\n%s", text)
	}
	if strings.Count(text, "[model_providers.openai]") != 1 {
		t.Fatalf("provider table count wrong:\n%s", text)
	}
	if !strings.Contains(text, `[projects."/tmp"]`) {
		t.Fatalf("unrelated table not preserved:\n%s", text)
	}
}

func TestApplyToTextDisablesGatewayProvider(t *testing.T) {
	raw := []byte(`model_provider = "openai"
model = "gpt-5.5"

[model_providers.openai]
name = "Umbragate OpenAI"
base_url = "http://127.0.0.1:4141/a/codex/openai/v1"
env_key = "OPENAI_API_KEY"
wire_api = "responses"

[features]
js_repl = false
`)

	out, err := ApplyToText(raw, ProviderInput{ID: "openai", Gateway: GatewayDisable})
	if err != nil {
		t.Fatalf("ApplyToText() error = %v", err)
	}
	text := string(out)
	if strings.Contains(text, "model_provider") || strings.Contains(text, "[model_providers.openai]") {
		t.Fatalf("gateway provider not removed:\n%s", text)
	}
	if !strings.Contains(text, `model = "gpt-5.5"`) || !strings.Contains(text, `[features]`) {
		t.Fatalf("unrelated config not preserved:\n%s", text)
	}
}

func TestManagerApplyUsesChecksum(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(`model = "gpt-5.5"`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	manager := Manager{Path: path}
	plan, err := manager.Plan(ProviderInput{ID: "openai", Gateway: GatewayEnable, GatewayBaseURL: "http://127.0.0.1:4141"})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(`model = "changed"`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := manager.Apply(ProviderInput{ID: "openai", Gateway: GatewayEnable}, plan.BaseChecksum); err != ErrStaleConfig {
		t.Fatalf("Apply() error = %v, want ErrStaleConfig", err)
	}
}

func TestStatusesDetectGatewayProvider(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(`model_provider = "openai"

[model_providers.openai]
name = "Umbragate OpenAI"
base_url = "http://127.0.0.1:4141/openai/v1"
env_key = "OPENAI_API_KEY"
wire_api = "responses"
`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	statuses, err := (Manager{Path: path}).Statuses([]string{"openai"}, "http://127.0.0.1:4141")
	if err != nil {
		t.Fatalf("Statuses() error = %v", err)
	}
	if len(statuses) != 1 || !statuses[0].Active || !statuses[0].GatewayEnabled || !statuses[0].Configured {
		t.Fatalf("statuses = %+v, want active configured gateway provider", statuses)
	}
}
