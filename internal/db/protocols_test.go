package db

import (
	"path/filepath"
	"testing"

	"github.com/jachy-h/llm-gateway-lite/internal/models"
)

func TestOpenRouterSeedSupportsChatAndResponsesProtocols(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	provider, err := database.GetProvider("openrouter")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		models.FormatChatCompletions: "https://openrouter.ai/api/v1",
		models.FormatResponses:       "https://openrouter.ai/api/v1/responses",
	}
	for _, endpoint := range provider.Endpoints {
		if endpoint.Protocol != models.ProtocolOpenAI {
			t.Fatalf("OpenRouter endpoint protocol = %q", endpoint.Protocol)
		}
		if expected, exists := want[endpoint.ResponseFormat]; exists {
			if endpoint.BaseURL != expected {
				t.Fatalf("%s URL = %q, want %q", endpoint.ResponseFormat, endpoint.BaseURL, expected)
			}
			delete(want, endpoint.ResponseFormat)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing OpenRouter protocol endpoints: %v", want)
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
		endpoint.BaseURL != "https://opencode.ai/zen/go/v1" {
		t.Fatalf("OpenCode Go endpoint contract = %+v", endpoint)
	}
}

func TestProviderSeedsUseCurrentCCSwitchEndpoints(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	want := map[string]string{
		"qwen":     "https://dashscope.aliyuncs.com/compatible-mode/v1",
		"minimax":  "https://api.minimaxi.com/v1",
		"opencode": "https://opencode.ai/zen/v1",
	}
	for id, baseURL := range want {
		provider, err := database.GetProvider(id)
		if err != nil {
			t.Fatalf("GetProvider(%q): %v", id, err)
		}
		if provider.BaseURL != baseURL || len(provider.Endpoints) == 0 || provider.Endpoints[0].BaseURL != baseURL {
			t.Fatalf("%s endpoint = provider=%q endpoints=%+v, want %q", id, provider.BaseURL, provider.Endpoints, baseURL)
		}
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
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	database, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.GetProvider("old"); err != ErrNotFound {
		t.Fatalf("stale provider lookup error = %v, want ErrNotFound", err)
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

func TestGeminiSeedUsesOpenAICompatibleEndpoint(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	provider, err := database.GetProvider("google")
	if err != nil {
		t.Fatal(err)
	}
	if len(provider.Endpoints) != 1 ||
		provider.Endpoints[0].Protocol != models.ProtocolOpenAI ||
		provider.Endpoints[0].BaseURL != "https://generativelanguage.googleapis.com/v1beta/openai" {
		t.Fatalf("Gemini OpenAI-compatible endpoint = %+v", provider.Endpoints)
	}
}
