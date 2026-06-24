package opencodeconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMissingConfigReturnsEmptyConfig(t *testing.T) {
	manager := Manager{Path: filepath.Join(t.TempDir(), "opencode.json")}

	cfg, raw, err := manager.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(raw) != 0 {
		t.Fatalf("raw length = %d, want 0", len(raw))
	}
	if cfg["$schema"] != SchemaURL {
		t.Fatalf("schema = %v, want %s", cfg["$schema"], SchemaURL)
	}
}

func TestDiscoverReturnsSupportedConfigFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"opencode.json", "opencode.jsonc", filepath.Join(".opencode", "opencode.json"), filepath.Join(".opencode", "opencode.jsonc")} {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
		if err := os.WriteFile(path, []byte(`{}`), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
	}

	files := Discover(dir)
	if len(files) != 4 {
		t.Fatalf("file count = %d, want 4: %#v", len(files), files)
	}
	for _, file := range files {
		if !file.Exists {
			t.Fatalf("file not marked existing: %#v", file)
		}
	}
}

func TestDiscoverReturnsDefaultCreatableConfigWhenNoneExist(t *testing.T) {
	dir := t.TempDir()

	files := Discover(dir)
	if len(files) != 1 {
		t.Fatalf("file count = %d, want 1: %#v", len(files), files)
	}
	if files[0].Path != filepath.Join(dir, "opencode.json") || files[0].Exists {
		t.Fatalf("default file = %#v", files[0])
	}
}

func TestLoadParsesJSONC(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.jsonc")
	content := []byte("{\n  // comment\n  \"provider\": {\n    \"openai\": {\n      \"options\": {\"apiKey\": \"secret\",},\n    },\n  },\n}\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	manager := Manager{Path: path}

	cfg, _, err := manager.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	provider := cfg["provider"].(map[string]any)
	openai := provider["openai"].(map[string]any)
	options := openai["options"].(map[string]any)
	if options["apiKey"] != "secret" {
		t.Fatalf("apiKey = %v", options["apiKey"])
	}
}

func TestPlanDoesNotWriteModelsOrDefaultModels(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.json")
	if err := os.WriteFile(path, []byte(`{"provider":{}}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	manager := Manager{Path: path}

	plan, err := manager.Plan(ProviderInput{ID: "kimi", APIKey: "key", Models: []string{"moonshot-v1-8k"}, DefaultModel: "kimi/moonshot-v1-8k", SmallModel: "kimi/moonshot-v1-8k"})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if _, ok := plan.Proposed["model"]; ok {
		t.Fatalf("model should not be written: %#v", plan.Proposed)
	}
	if _, ok := plan.Proposed["small_model"]; ok {
		t.Fatalf("small_model should not be written: %#v", plan.Proposed)
	}
	provider := plan.Proposed["provider"].(map[string]any)
	kimi := provider["kimi"].(map[string]any)
	if _, ok := kimi["models"]; ok {
		t.Fatalf("models should not be written: %#v", kimi)
	}
}

func TestPlanGatewayToggleSetsAndRemovesGatewayBaseURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.json")
	if err := os.WriteFile(path, []byte(`{"provider":{"openai":{"options":{"apiKey":"key"}}}}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	manager := Manager{Path: path}

	plan, err := manager.Plan(ProviderInput{ID: "openai", Gateway: GatewayEnable, GatewayBaseURL: "http://127.0.0.1:4141"})
	if err != nil {
		t.Fatalf("Plan() enable error = %v", err)
	}
	provider := plan.Proposed["provider"].(map[string]any)
	openai := provider["openai"].(map[string]any)
	options := openai["options"].(map[string]any)
	if options["baseURL"] != "http://127.0.0.1:4141/a/opencode/openai" {
		t.Fatalf("baseURL = %v", options["baseURL"])
	}

	if err := os.WriteFile(path, []byte(`{"provider":{"openai":{"options":{"apiKey":"key","baseURL":"http://127.0.0.1:4141/a/opencode/openai"}}}}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	plan, err = manager.Plan(ProviderInput{ID: "openai", Gateway: GatewayDisable, GatewayBaseURL: "http://127.0.0.1:4141"})
	if err != nil {
		t.Fatalf("Plan() disable error = %v", err)
	}
	provider = plan.Proposed["provider"].(map[string]any)
	openai = provider["openai"].(map[string]any)
	options = openai["options"].(map[string]any)
	if _, ok := options["baseURL"]; ok {
		t.Fatalf("gateway baseURL should be removed: %#v", options)
	}
}

func TestGatewayURLMatchesExistingGatewayForms(t *testing.T) {
	tests := []string{
		"http://127.0.0.1:4141/a/opencode/openai",
		"http://127.0.0.1:4141/openai",
		"http://localhost:4141/a/opencode/openrouter",
	}
	for _, baseURL := range tests {
		id := "openai"
		if strings.Contains(baseURL, "openrouter") {
			id = "openrouter"
		}
		if !GatewayURLMatches(baseURL, "http://127.0.0.1:4141", id) {
			t.Fatalf("GatewayURLMatches(%q) = false, want true", baseURL)
		}
	}
	if GatewayURLMatches("https://api.openai.com/v1", "http://127.0.0.1:4141", "openai") {
		t.Fatal("upstream URL should not match gateway")
	}
	if GatewayURLMatches("http://127.0.0.1:4141/a/opencode/deepseek", "http://127.0.0.1:4141", "openai") {
		t.Fatal("different provider gateway URL should not match")
	}
}

func TestPlanPreservesUnrelatedFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.json")
	if err := os.WriteFile(path, []byte(`{"$schema":"https://opencode.ai/config.json","username":"alice","provider":{"openai":{"options":{"apiKey":"old"}}}}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	manager := Manager{Path: path}

	plan, err := manager.Plan(ProviderInput{ID: "deepseek", APIKey: "new-key", DefaultModel: "deepseek/deepseek-chat"})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}

	if plan.Proposed["username"] != "alice" {
		t.Fatalf("username was not preserved: %#v", plan.Proposed)
	}
	if _, ok := plan.Proposed["model"]; ok {
		t.Fatalf("model should not be written: %#v", plan.Proposed)
	}
	provider := plan.Proposed["provider"].(map[string]any)
	if _, ok := provider["openai"]; !ok {
		t.Fatalf("existing provider missing: %#v", provider)
	}
	deepseek := provider["deepseek"].(map[string]any)
	options := deepseek["options"].(map[string]any)
	if options["apiKey"] != "new-key" {
		t.Fatalf("apiKey = %v", options["apiKey"])
	}
}

func TestPlanMasksAPIKeysInDiff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.json")
	if err := os.WriteFile(path, []byte(`{"provider":{"openai":{"options":{"apiKey":"old-secret"}}}}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	manager := Manager{Path: path}

	plan, err := manager.Plan(ProviderInput{ID: "openai", APIKey: "new-secret"})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}

	if strings.Contains(plan.Diff, "old-secret") || strings.Contains(plan.Diff, "new-secret") {
		t.Fatalf("diff leaked secret: %s", plan.Diff)
	}
	if !strings.Contains(plan.Diff, "********") {
		t.Fatalf("diff did not mask api key: %s", plan.Diff)
	}
}

func TestApplyRejectsStaleChecksum(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.json")
	if err := os.WriteFile(path, []byte(`{"provider":{}}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	manager := Manager{Path: path}
	plan, err := manager.Plan(ProviderInput{ID: "openai", APIKey: "key"})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"username":"changed"}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	err = manager.Apply(ProviderInput{ID: "openai", APIKey: "key"}, plan.BaseChecksum)
	if err == nil {
		t.Fatal("Apply() error = nil, want stale checksum error")
	}
}

func TestApplyCreatesBackupAndWritesAtomically(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "opencode.json")
	if err := os.WriteFile(path, []byte(`{"username":"alice","provider":{}}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	manager := Manager{Path: path}
	plan, err := manager.Plan(ProviderInput{ID: "kimi", APIKey: "key", BaseURL: "https://api.moonshot.cn/v1", Models: []string{"moonshot-v1-8k"}})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}

	if err := manager.Apply(ProviderInput{ID: "kimi", APIKey: "key", BaseURL: "https://api.moonshot.cn/v1", Models: []string{"moonshot-v1-8k"}}, plan.BaseChecksum); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	for _, want := range []string{"\"username\": \"alice\"", "\"kimi\"", "\"apiKey\": \"key\"", "\"baseURL\": \"https://api.moonshot.cn/v1\""} {
		if !strings.Contains(string(written), want) {
			t.Fatalf("written config missing %q: %s", want, string(written))
		}
	}
	matches, err := filepath.Glob(filepath.Join(dir, "opencode.json.*.bak"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("backup count = %d, want 1", len(matches))
	}
}
