package codex

import (
	"path/filepath"
	"testing"

	"github.com/jachy-h/umbra-gate/agents"
)

func TestStatusDefaultsToOfficialOpenAIProviderOnly(t *testing.T) {
	status, err := (Manager{Path: filepath.Join(t.TempDir(), "config.toml")}).Status(agents.Context{
		GatewayBaseURL: "http://127.0.0.1:4141",
		ProviderIDs:    []string{"openai", "anthropic", "openrouter"},
	})
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if len(status.Bindings) != 1 || status.Bindings[0].ProviderID != "openai" {
		t.Fatalf("bindings = %+v, want only openai", status.Bindings)
	}
}
