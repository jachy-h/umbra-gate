package claude

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jachy-h/umbra-gate/agents"
)

const (
	agentID         = "claude-code"
	displayName     = "Claude Code"
	providerID      = "anthropic"
	baseURLEnvKey   = "ANTHROPIC_BASE_URL"
	authTokenEnvKey = "ANTHROPIC_AUTH_TOKEN"
	authPlaceholder = "PROXY_MANAGED"
)

var ErrStaleConfig = errors.New("claude config changed since diff was generated")

type Manager struct {
	Path string
}

func (m Manager) ID() string {
	return agentID
}

func (m Manager) DisplayName() string {
	return displayName
}

func (m Manager) Discover() ([]agents.ConfigFile, error) {
	path := m.path()
	_, err := os.Stat(path)
	return []agents.ConfigFile{{
		Path:     path,
		Label:    "settings.json",
		Exists:   err == nil,
		Selected: true,
	}}, nil
}

func (m Manager) Status(ctx agents.Context) (*agents.Status, error) {
	cfg, _, err := m.load()
	if err != nil {
		return nil, err
	}
	files, err := m.Discover()
	if err != nil {
		return nil, err
	}
	liveBaseURL := envString(cfg, baseURLEnvKey)
	return &agents.Status{
		AgentID:               m.ID(),
		DisplayName:           m.DisplayName(),
		ConfigFiles:           files,
		GatewayCapable:        false,
		GatewayDisabledReason: "Proxy support is temporarily disabled while a reliable integration is evaluated.",
		FineGrained:           false,
		ProxyMethod:           "Environment Variable",
		Bindings: []agents.BindingStatus{{
			ProviderID:     providerID,
			Configured:     liveBaseURL != "",
			Active:         liveBaseURL != "",
			GatewayEnabled: gatewayURLMatches(liveBaseURL, ctx.GatewayBaseURL),
			GatewayBaseURL: gatewayURL(ctx.GatewayBaseURL),
			LiveBaseURL:    liveBaseURL,
			ConfigPath:     m.path(),
		}},
	}, nil
}

func (m Manager) Plan(ctx agents.Context, input agents.BindingInput) (*agents.Plan, error) {
	cfg, raw, err := m.load()
	if err != nil {
		return nil, err
	}
	proposed := cloneMap(cfg)
	applyInput(proposed, ctx, input)
	currentJSON, err := marshal(cfg)
	if err != nil {
		return nil, err
	}
	proposedJSON, err := marshal(proposed)
	if err != nil {
		return nil, err
	}
	return &agents.Plan{
		BaseChecksum: checksum(raw),
		Diff:         unifiedDiff(currentJSON, proposedJSON),
		Current:      string(currentJSON),
		ProposedText: string(proposedJSON),
	}, nil
}

func (m Manager) Apply(ctx agents.Context, input agents.BindingInput, baseChecksum string) error {
	_, raw, err := m.load()
	if err != nil {
		return err
	}
	if checksum(raw) != baseChecksum {
		return ErrStaleConfig
	}
	plan, err := m.Plan(ctx, input)
	if err != nil {
		return err
	}
	path := m.path()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if existing, err := os.ReadFile(path); err == nil {
		backup := fmt.Sprintf("%s.%s.bak", path, time.Now().Format("20060102-150405"))
		if err := os.WriteFile(backup, existing, 0o600); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".claude-settings-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.WriteString(plan.ProposedText); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".claude", "settings.json")
	}
	return filepath.Join(home, ".claude", "settings.json")
}

func (m Manager) path() string {
	if strings.TrimSpace(m.Path) != "" {
		return m.Path
	}
	return DefaultPath()
}

func (m Manager) load() (map[string]any, []byte, error) {
	raw, err := os.ReadFile(m.path())
	if errors.Is(err, os.ErrNotExist) {
		return map[string]any{}, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, nil, err
	}
	if cfg == nil {
		cfg = map[string]any{}
	}
	return cfg, raw, nil
}

func applyInput(cfg map[string]any, ctx agents.Context, input agents.BindingInput) {
	env := objectAt(cfg, "env")
	if input.Enabled {
		env[baseURLEnvKey] = gatewayURL(firstNonEmpty(input.GatewayBaseURL, ctx.GatewayBaseURL))
		env[authTokenEnvKey] = authPlaceholder
		return
	}
	if current, _ := env[baseURLEnvKey].(string); gatewayURLMatches(current, firstNonEmpty(input.GatewayBaseURL, ctx.GatewayBaseURL)) {
		delete(env, baseURLEnvKey)
		if token, _ := env[authTokenEnvKey].(string); token == authPlaceholder || token == "local" {
			delete(env, authTokenEnvKey)
		}
	}
}

func gatewayURL(baseURL string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = "http://127.0.0.1:4141"
	}
	return base + "/a/claude-code/anthropic"
}

func gatewayURLMatches(baseURL, gatewayBaseURL string) bool {
	current := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	base := strings.TrimRight(strings.TrimSpace(gatewayBaseURL), "/")
	return current == gatewayURL(gatewayBaseURL) ||
		current == base+"/anthropic"
}

func objectAt(parent map[string]any, key string) map[string]any {
	if existing, ok := parent[key].(map[string]any); ok {
		return existing
	}
	next := map[string]any{}
	parent[key] = next
	return next
}

func envString(cfg map[string]any, key string) string {
	env, _ := cfg["env"].(map[string]any)
	value, _ := env[key].(string)
	return value
}

func cloneMap(src map[string]any) map[string]any {
	data, _ := json.Marshal(src)
	var out map[string]any
	_ = json.Unmarshal(data, &out)
	if out == nil {
		out = map[string]any{}
	}
	return out
}

func marshal(cfg map[string]any) ([]byte, error) {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func checksum(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func unifiedDiff(current, proposed []byte) string {
	if bytes.Equal(current, proposed) {
		return ""
	}
	return "--- current\n+++ proposed\n" + string(proposed)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
