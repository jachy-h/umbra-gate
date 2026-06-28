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
		`model_provider = "custom"`,
		`[model_providers.custom]`,
		`base_url = "http://127.0.0.1:4141/a/codex/openai/v1"`,
		`wire_api = "responses"`,
		`requires_openai_auth = true`,
		`[mcp_servers.node_repl]`,
		`command = "node"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q:\n%s", want, text)
		}
	}
}

func TestApplyToTextEnablesGatewayForReservedOpenAI(t *testing.T) {
	raw := []byte(`model_provider = "openai"
model = "gpt-5.5"
`)

	out, err := ApplyToText(raw, ProviderInput{ID: "openai", Gateway: GatewayEnable, GatewayBaseURL: "http://127.0.0.1:4141"})
	if err != nil {
		t.Fatalf("ApplyToText() error = %v", err)
	}
	text := string(out)
	if strings.Contains(text, `[model_providers.openai]`) {
		t.Fatalf("must not create reserved [model_providers.openai] table:\n%s", text)
	}
	for _, want := range []string{
		`model_provider = "custom"`,
		`[model_providers.custom]`,
		`base_url = "http://127.0.0.1:4141/a/codex/openai/v1"`,
		`requires_openai_auth = true`,
		`model = "gpt-5.5"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q:\n%s", want, text)
		}
	}
}

func TestApplyToTextEnablesGatewayProviderInActiveTable(t *testing.T) {
	raw := []byte(`model_provider = "custom"
model = "gpt-5.5"

[model_providers.custom]
name = "MyCustom"
base_url = "https://api.example.com/v1"
env_key = "MY_KEY"
wire_api = "responses"
`)

	out, err := ApplyToText(raw, ProviderInput{ID: "openai", Gateway: GatewayEnable, GatewayBaseURL: "http://127.0.0.1:4141"})
	if err != nil {
		t.Fatalf("ApplyToText() error = %v", err)
	}
	text := string(out)
	for _, want := range []string{
		`model_provider = "custom"`,
		`[model_providers.custom]`,
		`name = "MyCustom"`,
		`base_url = "http://127.0.0.1:4141/a/codex/openai/v1"`,
		`env_key = "MY_KEY"`,
		`wire_api = "responses"`,
		`model = "gpt-5.5"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "https://api.example.com") {
		t.Fatalf("old base_url should be replaced:\n%s", text)
	}
}

func TestApplyToTextReplacesExistingProviderTable(t *testing.T) {
	raw := []byte(`model_provider = "custom"

[model_providers.custom]
name = "Old"
base_url = "https://old.example/v1"
env_key = "OLD_KEY"
wire_api = "chat"

[projects."/tmp"]
trust_level = "trusted"
`)

	out, err := ApplyToText(raw, ProviderInput{ID: "custom", Name: "New", BaseURL: "https://new.example/v1", EnvKey: "NEW_KEY", WireAPI: "responses"})
	if err != nil {
		t.Fatalf("ApplyToText() error = %v", err)
	}
	text := string(out)
	if strings.Contains(text, "https://old.example") || strings.Contains(text, "OLD_KEY") || strings.Contains(text, "wire_api = \"chat\"") {
		t.Fatalf("old provider table was not replaced:\n%s", text)
	}
	if strings.Count(text, "[model_providers.custom]") != 1 {
		t.Fatalf("provider table count wrong:\n%s", text)
	}
	if !strings.Contains(text, `[projects."/tmp"]`) {
		t.Fatalf("unrelated table not preserved:\n%s", text)
	}
}

func TestApplyToTextDisablesGatewayProvider(t *testing.T) {
	raw := []byte(`model_provider = "custom"
model = "gpt-5.5"

[model_providers.custom]
name = "Umbragate"
base_url = "http://127.0.0.1:4141/a/codex/openai/v1"
wire_api = "responses"
requires_openai_auth = true

[features]
js_repl = false
`)

	out, err := ApplyToText(raw, ProviderInput{ID: "openai", Gateway: GatewayDisable, GatewayBaseURL: "http://127.0.0.1:4141"})
	if err != nil {
		t.Fatalf("ApplyToText() error = %v", err)
	}
	text := string(out)
	if strings.Contains(text, "http://127.0.0.1:4141/a/codex/openai/v1") {
		t.Fatalf("gateway base_url not removed:\n%s", text)
	}
	if strings.Contains(text, "model_provider") {
		t.Fatalf("managed model_provider should be removed on disable:\n%s", text)
	}
	if strings.Contains(text, "[model_providers.custom]") {
		t.Fatalf("managed custom table should be removed on disable:\n%s", text)
	}
	for _, want := range []string{
		`model = "gpt-5.5"`,
		`[features]`,
		`js_repl = false`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q:\n%s", want, text)
		}
	}
}

func TestApplyToTextDisablesGatewayProviderInActiveTable(t *testing.T) {
	raw := []byte(`model_provider = "custom"
model = "gpt-5.5"

[model_providers.custom]
name = "MyCustom"
base_url = "http://127.0.0.1:4141/v1"
env_key = "MY_KEY"
wire_api = "responses"
`)

	out, err := ApplyToText(raw, ProviderInput{ID: "openai", Gateway: GatewayDisable, GatewayBaseURL: "http://127.0.0.1:4141"})
	if err != nil {
		t.Fatalf("ApplyToText() error = %v", err)
	}
	text := string(out)
	if strings.Contains(text, "http://127.0.0.1:4141/v1") {
		t.Fatalf("gateway base_url not stripped:\n%s", text)
	}
	for _, want := range []string{
		`model_provider = "custom"`,
		`[model_providers.custom]`,
		`name = "MyCustom"`,
		`env_key = "MY_KEY"`,
		`wire_api = "responses"`,
		`model = "gpt-5.5"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q:\n%s", want, text)
		}
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
	if err := os.WriteFile(path, []byte(`model_provider = "custom"

[model_providers.custom]
name = "Umbragate"
base_url = "http://127.0.0.1:4141/a/codex/openai/v1"
wire_api = "responses"
requires_openai_auth = true
`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	statuses, err := (Manager{Path: path}).Statuses([]string{"openai"}, "http://127.0.0.1:4141")
	if err != nil {
		t.Fatalf("Statuses() error = %v", err)
	}
	if len(statuses) != 1 || !statuses[0].GatewayEnabled {
		t.Fatalf("statuses = %+v, want gateway-enabled via custom provider", statuses)
	}
}
