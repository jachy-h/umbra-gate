package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jachy-h/umbra-gate/agents"
)

func TestPlanEnablesGatewayPreservingUnrelatedSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"permissions":{"allow":["Bash(ls)"]},"env":{"OTHER":"x"}}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	plan, err := (Manager{Path: path}).Plan(agents.Context{GatewayBaseURL: "http://127.0.0.1:4141"}, agents.BindingInput{Enabled: true})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	for _, want := range []string{
		`"permissions"`,
		`"OTHER": "x"`,
		`"ANTHROPIC_BASE_URL": "http://127.0.0.1:4141/a/claude-code/anthropic"`,
		`"ANTHROPIC_AUTH_TOKEN": "PROXY_MANAGED"`,
	} {
		if !strings.Contains(plan.ProposedText, want) {
			t.Fatalf("plan missing %q:\n%s", want, plan.ProposedText)
		}
	}
}

func TestApplyUsesChecksum(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"env":{}}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	manager := Manager{Path: path}
	ctx := agents.Context{GatewayBaseURL: "http://127.0.0.1:4141"}
	plan, err := manager.Plan(ctx, agents.BindingInput{Enabled: true})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"env":{"changed":"yes"}}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := manager.Apply(ctx, agents.BindingInput{Enabled: true}, plan.BaseChecksum); err != ErrStaleConfig {
		t.Fatalf("Apply() error = %v, want ErrStaleConfig", err)
	}
}

func TestStatusDetectsGatewayAndDisableRemovesManagedValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"env":{"ANTHROPIC_BASE_URL":"http://127.0.0.1:4141/a/claude-code/anthropic","ANTHROPIC_AUTH_TOKEN":"PROXY_MANAGED","OTHER":"x"}}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	manager := Manager{Path: path}
	ctx := agents.Context{GatewayBaseURL: "http://127.0.0.1:4141"}
	status, err := manager.Status(ctx)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if len(status.Bindings) != 1 || !status.Bindings[0].GatewayEnabled || !status.Bindings[0].Active {
		t.Fatalf("status = %+v, want enabled binding", status)
	}

	plan, err := manager.Plan(ctx, agents.BindingInput{Enabled: false})
	if err != nil {
		t.Fatalf("Plan(disable) error = %v", err)
	}
	if strings.Contains(plan.ProposedText, "ANTHROPIC_BASE_URL") || strings.Contains(plan.ProposedText, "PROXY_MANAGED") {
		t.Fatalf("managed values should be removed:\n%s", plan.ProposedText)
	}
	if !strings.Contains(plan.ProposedText, `"OTHER": "x"`) {
		t.Fatalf("unrelated env missing:\n%s", plan.ProposedText)
	}
}
