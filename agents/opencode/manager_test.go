package opencode

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jachy-h/umbra-gate/agents"
)

func TestStatusReadsSelectedDiscoveredConfigFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configDir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	configPath := filepath.Join(configDir, "opencode.jsonc")
	if err := os.WriteFile(configPath, []byte(`{
  "provider": {
    "volcengine": {
      "options": {
        "baseURL": "http://127.0.0.1:4141/a/opencode/volcengine"
      }
    }
  }
}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	status, err := (Manager{}).Status(agents.Context{
		GatewayBaseURL: "http://127.0.0.1:4141",
		ProviderIDs:    []string{"volcengine"},
	})
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if len(status.Bindings) != 1 {
		t.Fatalf("bindings = %+v, want one binding", status.Bindings)
	}
	binding := status.Bindings[0]
	if binding.ConfigPath != configPath {
		t.Fatalf("ConfigPath = %q, want %q", binding.ConfigPath, configPath)
	}
	if !binding.GatewayEnabled {
		t.Fatalf("GatewayEnabled = false, binding = %+v", binding)
	}
}
