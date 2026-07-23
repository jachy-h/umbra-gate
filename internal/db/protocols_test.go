package db

import (
	"path/filepath"
	"testing"

	"github.com/jachy-h/llm-gateway-lite/internal/models"
)

func TestProviderSeedsIncludeOnlyDeepSeekAndOpenCode(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	providers, err := database.ListProviders()
	if err != nil {
		t.Fatal(err)
	}
	if len(providers) != 3 {
		t.Fatalf("provider count = %d, want 3", len(providers))
	}
	for _, provider := range providers {
		if !provider.Builtin || (provider.ID != "deepseek" && provider.ID != "opencode" && provider.ID != "opencode-go") {
			t.Fatalf("unexpected built-in provider: %+v", provider)
		}
	}
}

func TestOpenCodeGoSeedDeclaresAsymmetricOpenAIEndpoint(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	provider, err := database.GetProvider("opencode-go")
	if err != nil {
		t.Fatal(err)
	}
	if len(provider.Endpoints) != 1 {
		t.Fatalf("OpenCode Go endpoints = %+v", provider.Endpoints)
	}
	endpoint := provider.Endpoints[0]
	if endpoint.Protocol != models.ProtocolOpenAI ||
		endpoint.RequestFormat != models.FormatChatCompletions ||
		endpoint.ResponseFormat != models.FormatResponses ||
		endpoint.BaseURL != "https://opencode.ai/zen/go/v1/responses" {
		t.Fatalf("OpenCode Go endpoint contract = %+v", endpoint)
	}
}

func TestDeepSeekSeedSupportsOpenAIAndAnthropicProtocols(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	provider, err := database.GetProvider("deepseek")
	if err != nil {
		t.Fatal(err)
	}
	if provider.BaseURL != "https://api.deepseek.com" {
		t.Fatalf("DeepSeek base URL = %q", provider.BaseURL)
	}
	want := map[string]string{
		models.ProtocolOpenAI:    "https://api.deepseek.com",
		models.ProtocolAnthropic: "https://api.deepseek.com/anthropic",
	}
	for _, endpoint := range provider.Endpoints {
		if expected, exists := want[endpoint.Protocol]; exists {
			if endpoint.BaseURL != expected {
				t.Fatalf("DeepSeek %s URL = %q, want %q", endpoint.Protocol, endpoint.BaseURL, expected)
			}
			delete(want, endpoint.Protocol)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing DeepSeek protocol endpoints: %v", want)
	}
}

func TestOpenCodeSeedUsesResponsesEndpoint(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	provider, err := database.GetProvider("opencode")
	if err != nil {
		t.Fatal(err)
	}
	if provider.BaseURL != "https://opencode.ai/zen/v1/responses" || len(provider.Endpoints) != 1 || provider.Endpoints[0].BaseURL != provider.BaseURL {
		t.Fatalf("OpenCode endpoint = provider=%q endpoints=%+v", provider.BaseURL, provider.Endpoints)
	}
}

func TestProviderSeedResetRunsOnlyOncePerSeedVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.db")
	database, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`DELETE FROM stats_meta WHERE key='provider_seed_version'`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO providers(id,name,type,base_url,endpoints_json) VALUES('old','Old','custom','https://old.example/v1','[]')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO providers(id,name,type,base_url,endpoints_json,builtin) VALUES('old-builtin','Old Built-in','custom','https://old-builtin.example/v1','[]',1)`); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	database, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.GetProvider("old"); err != nil {
		t.Fatalf("custom provider lookup error = %v, want nil", err)
	}
	if _, err := database.GetProvider("old-builtin"); err != ErrNotFound {
		t.Fatalf("removed built-in provider lookup error = %v, want ErrNotFound", err)
	}
	if _, err := database.Exec(`INSERT INTO providers(id,name,type,base_url,endpoints_json) VALUES('custom','Custom','custom','https://custom.example/v1','[]')`); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	database, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := database.GetProvider("custom"); err != nil {
		t.Fatalf("custom provider lookup error = %v, want nil", err)
	}
}
