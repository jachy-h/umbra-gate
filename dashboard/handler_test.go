package dashboard

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jachy-h/umbra-gate/config"
	"github.com/jachy-h/umbra-gate/db"
)

func TestDashboardServesViteSPAShell(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	handler := New(database, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	for _, want := range []string{
		"<title>Umbragate</title>",
		`<div id="app"></div>`,
		`type="module" crossorigin src="/dashboard/assets/`,
		`rel="stylesheet" crossorigin href="/dashboard/assets/`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("home body does not contain %q", want)
		}
	}
	for _, notWant := range []string{"/dashboard/static/dashboard/", "vue.esm-browser.prod.js", "Personal AI Router"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("SPA shell should not contain old dashboard artifact %q: %s", notWant, body)
		}
	}
}

func TestDashboardDeepLinksFallBackToSPAShell(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	handler := New(database, nil)
	for _, route := range []string{"/dashboard/providers", "/dashboard/agents", "/dashboard/sessions/12"} {
		req := httptest.NewRequest(http.MethodGet, route, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body = %s", route, w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), `<div id="app"></div>`) {
			t.Fatalf("%s did not return SPA shell: %s", route, w.Body.String())
		}
	}
}

func TestDashboardViteAssetsAreServed(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()
	handler := New(database, nil)

	indexReq := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	indexW := httptest.NewRecorder()
	handler.ServeHTTP(indexW, indexReq)
	assetPath := extractDashboardAssetPath(t, indexW.Body.String(), ".js")

	req := httptest.NewRequest(http.MethodGet, assetPath, nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{"/api/providers/analytics", "/api/analytics/breakdown", "/api/agents", "createApp"} {
		if !strings.Contains(body, want) {
			t.Fatalf("Vite asset missing %q", want)
		}
	}
	if strings.Contains(body, "/api/gateway/providers") {
		t.Fatalf("statistics-only dashboard bundle should not request provider management API")
	}
}

func extractDashboardAssetPath(t *testing.T, body, suffix string) string {
	t.Helper()
	start := strings.Index(body, "/dashboard/assets/")
	for start >= 0 {
		rest := body[start:]
		end := strings.IndexAny(rest, `"'`)
		if end < 0 {
			t.Fatalf("asset path not terminated in %s", body)
		}
		candidate := rest[:end]
		if strings.HasSuffix(candidate, suffix) {
			return candidate
		}
		next := strings.Index(rest[len(candidate):], "/dashboard/assets/")
		if next < 0 {
			break
		}
		start += len(candidate) + next
	}
	t.Fatalf("no %s dashboard asset found in %s", suffix, body)
	return ""
}

func TestProviderConfigUsesOpencodeProviderListOnly(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()
	configPath := filepath.Join(t.TempDir(), "opencode.json")
	if err := os.WriteFile(configPath, []byte(`{"provider":{"cmdonly":{"options":{"apiKey":"secret"}}}}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	handler := newWithOptions(database, Options{
		OpencodeConfigPath: configPath,
		ProviderListCommand: func() ([]byte, error) {
			return []byte(`[{"id":"cmdonly","name":"Command Only"},{"id":"other","name":"Other Provider"}]`), nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/providers/config", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{"\"providers\"", "\"cmdonly\"", "Command Only", "\"other\"", "\"configured\":true", "\"has_api_key\":true", "\"gateway_enabled\":false", "********"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
	for _, notWant := range []string{"\"catalog\"", "\"openai\"", "\"anthropic\"", "secret"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("body should not contain %q: %s", notWant, body)
		}
	}
}

func TestProviderConfigEndpointReturnsCatalogAndMaskedConfig(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()
	configPath := filepath.Join(t.TempDir(), "opencode.json")
	if err := os.WriteFile(configPath, []byte(`{"provider":{"openai":{"options":{"apiKey":"secret"}}}}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	handler := newWithOptions(database, Options{OpencodeConfigPath: configPath, ProviderListCommand: func() ([]byte, error) {
		return []byte(`[{"id":"openai","name":"OpenAI"},{"id":"anthropic","name":"Anthropic"},{"id":"deepseek","name":"DeepSeek"},{"id":"glm","name":"GLM"},{"id":"kimi","name":"Kimi"}]`), nil
	}})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/providers/config", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{"\"files\"", "\"providers\"", "\"openai\"", "\"anthropic\"", "\"deepseek\"", "\"glm\"", "\"kimi\"", "\"built_in\":true", "\"configured\":true", "\"has_api_key\":true", "********"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
	if strings.Contains(body, "secret") {
		t.Fatalf("body leaked api key: %s", body)
	}
}

func TestCanonicalProviderIDNormalizesKnownDisplayNames(t *testing.T) {
	cases := map[string]string{
		"OpenCode Zen":   "opencode",
		"GitHub Copilot": "github-copilot",
		"Velcengine":     "volcengine",
		"Volcengine":     "volcengine",
	}
	for name, want := range cases {
		if got := canonicalProviderID(name); got != want {
			t.Fatalf("canonicalProviderID(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestProviderStatusesNormalizeKnownDisplayNames(t *testing.T) {
	statuses := providerStatuses(map[string]any{
		"provider": map[string]any{
			"volcengine": map[string]any{
				"name": "Velcengine",
			},
		},
	}, []providerListEntry{{ID: "volcengine", Name: "Velcengine"}}, "http://127.0.0.1:4141")

	if len(statuses) != 1 {
		t.Fatalf("len(statuses) = %d, want 1", len(statuses))
	}
	if got := statuses[0].Name; got != "Volcengine" {
		t.Fatalf("status name = %q, want Volcengine", got)
	}
}

func TestProviderDiffDoesNotWriteConfig(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()
	configPath := filepath.Join(t.TempDir(), "opencode.json")
	original := []byte(`{"provider":{}}`)
	if err := os.WriteFile(configPath, original, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	handler := newWithOptions(database, Options{OpencodeConfigPath: configPath})
	payload := []byte(`{"path":"` + configPath + `","id":"openai","api_key":"secret","models":["gpt-4o"],"default_model":"openai/gpt-4o"}`)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/providers/diff", bytes.NewReader(payload))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "--- current") || !strings.Contains(w.Body.String(), "+++ proposed") {
		t.Fatalf("diff response missing unified diff: %s", w.Body.String())
	}
	written, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(written) != string(original) {
		t.Fatalf("config was written during diff: %s", string(written))
	}
	if strings.Contains(w.Body.String(), "gpt-4o") || strings.Contains(w.Body.String(), "default_model") {
		t.Fatalf("diff should not include model config: %s", w.Body.String())
	}
}

func TestProviderDiffCanEnableGatewayBaseURL(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()
	configPath := filepath.Join(t.TempDir(), "opencode.json")
	if err := os.WriteFile(configPath, []byte(`{"provider":{"openai":{"options":{"apiKey":"secret"}}}}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	handler := newWithOptions(database, Options{OpencodeConfigPath: configPath, GatewayBaseURL: "http://127.0.0.1:4141"})
	payload := []byte(`{"path":"` + configPath + `","id":"openai","gateway":"enable"}`)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/providers/diff", bytes.NewReader(payload))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "http://127.0.0.1:4141/a/opencode/openai") {
		t.Fatalf("diff missing gateway baseURL: %s", w.Body.String())
	}
}

func TestProviderApplyWritesWithChecksum(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()
	configPath := filepath.Join(t.TempDir(), "opencode.json")
	if err := os.WriteFile(configPath, []byte(`{"provider":{}}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	handler := newWithOptions(database, Options{OpencodeConfigPath: configPath})
	payload := []byte(`{"path":"` + configPath + `","id":"openai","api_key":"secret"}`)
	diffReq := httptest.NewRequest(http.MethodPost, "/dashboard/providers/diff", bytes.NewReader(payload))
	diffW := httptest.NewRecorder()
	handler.ServeHTTP(diffW, diffReq)
	var diffResp struct {
		BaseChecksum string `json:"base_checksum"`
	}
	if err := json.Unmarshal(diffW.Body.Bytes(), &diffResp); err != nil {
		t.Fatalf("Unmarshal() error = %v, body = %s", err, diffW.Body.String())
	}
	applyPayload := []byte(`{"path":"` + configPath + `","id":"openai","api_key":"secret","base_checksum":"` + diffResp.BaseChecksum + `"}`)

	applyReq := httptest.NewRequest(http.MethodPost, "/dashboard/providers/apply", bytes.NewReader(applyPayload))
	applyW := httptest.NewRecorder()
	handler.ServeHTTP(applyW, applyReq)

	if applyW.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", applyW.Code, applyW.Body.String())
	}
	written, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(written), "\"apiKey\": \"secret\"") {
		t.Fatalf("config not written: %s", string(written))
	}
}

func TestProviderGatewayTogglesAtomically(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()
	configPath := filepath.Join(t.TempDir(), "opencode.json")
	if err := os.WriteFile(configPath, []byte(`{"provider":{"openai":{"options":{"apiKey":"secret","baseURL":"https://api.openai.com/v1"}}}}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	gatewayCfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(gatewayCfgPath, []byte("providers: {}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	gatewayCfg, err := config.Load(gatewayCfgPath)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	handler := newWithOptions(database, Options{OpencodeConfigPath: configPath, GatewayBaseURL: "http://127.0.0.1:4141", GatewayConfig: gatewayCfg})

	enableReq := httptest.NewRequest(http.MethodPost, "/dashboard/providers/gateway", bytes.NewReader([]byte(`{"path":"`+configPath+`","id":"openai","enabled":true}`)))
	enableW := httptest.NewRecorder()
	handler.ServeHTTP(enableW, enableReq)
	if enableW.Code != http.StatusOK {
		t.Fatalf("enable status = %d, body = %s", enableW.Code, enableW.Body.String())
	}
	written, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(written), "\"baseURL\": \"http://127.0.0.1:4141/a/opencode/openai\"") {
		t.Fatalf("baseURL not set after enable: %s", string(written))
	}
	if _, ok := gatewayCfg.Provider("openai"); !ok {
		t.Fatal("gateway config.yaml missing provider after enable")
	}

	disableReq := httptest.NewRequest(http.MethodPost, "/dashboard/providers/gateway", bytes.NewReader([]byte(`{"path":"`+configPath+`","id":"openai","enabled":false}`)))
	disableW := httptest.NewRecorder()
	handler.ServeHTTP(disableW, disableReq)
	if disableW.Code != http.StatusOK {
		t.Fatalf("disable status = %d, body = %s", disableW.Code, disableW.Body.String())
	}
	written2, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(written2), "127.0.0.1:4141") {
		t.Fatalf("baseURL not removed after disable: %s", string(written2))
	}
	if _, ok := gatewayCfg.Provider("openai"); ok {
		t.Fatal("gateway config.yaml should not have provider after disable")
	}
}

func TestProviderGatewayRejectsMissingID(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()
	handler := newWithOptions(database, Options{OpencodeConfigPath: filepath.Join(t.TempDir(), "opencode.json")})

	req := httptest.NewRequest(http.MethodPost, "/dashboard/providers/gateway", bytes.NewReader([]byte(`{"enabled":true}`)))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", w.Code, w.Body.String())
	}
}

func TestCodexConfigEndpointReturnsGatewayStatus(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()
	dir := t.TempDir()
	codexPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(codexPath, []byte(`model_provider = "custom"

[model_providers.custom]
name = "Umbragate"
base_url = "http://127.0.0.1:4141/a/codex/openai/v1"
wire_api = "responses"
requires_openai_auth = true
`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	gatewayCfgPath := filepath.Join(dir, "gateway.yaml")
	if err := os.WriteFile(gatewayCfgPath, []byte("providers:\n  openai:\n    base_url: https://api.openai.com\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	gatewayCfg, err := config.Load(gatewayCfgPath)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	handler := newWithOptions(database, Options{CodexConfigPath: codexPath, GatewayBaseURL: "http://127.0.0.1:4141", GatewayConfig: gatewayCfg})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/codex/config", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{`"providers"`, `"openai"`, `"gateway_enabled":true`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestCodexGatewayRejectsEnableWhileProxyIsDisabled(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()
	dir := t.TempDir()
	codexPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(codexPath, []byte(`model = "gpt-5.5"

[features]
js_repl = false
`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	gatewayCfgPath := filepath.Join(dir, "gateway.yaml")
	if err := os.WriteFile(gatewayCfgPath, []byte("providers: {}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	gatewayCfg, err := config.Load(gatewayCfgPath)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	handler := newWithOptions(database, Options{CodexConfigPath: codexPath, GatewayBaseURL: "http://127.0.0.1:4141", GatewayConfig: gatewayCfg})

	enableReq := httptest.NewRequest(http.MethodPost, "/dashboard/codex/gateway", bytes.NewReader([]byte(`{"path":"`+codexPath+`","id":"openai","enabled":true}`)))
	enableW := httptest.NewRecorder()
	handler.ServeHTTP(enableW, enableReq)
	if enableW.Code != http.StatusConflict {
		t.Fatalf("enable status = %d, want %d; body = %s", enableW.Code, http.StatusConflict, enableW.Body.String())
	}
	written, err := os.ReadFile(codexPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(written), "127.0.0.1:4141") || strings.Contains(string(written), "model_provider") {
		t.Fatalf("disabled proxy changed codex config:\n%s", string(written))
	}
	if _, ok := gatewayCfg.Provider("openai"); ok {
		t.Fatal("disabled proxy registered a gateway provider")
	}
}

func TestCodexGatewayRejectsNonOpenAIProvider(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()
	dir := t.TempDir()
	codexPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(codexPath, []byte(`model = "gpt-5.5"`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	handler := newWithOptions(database, Options{CodexConfigPath: codexPath, GatewayBaseURL: "http://127.0.0.1:4141"})

	req := httptest.NewRequest(http.MethodPost, "/dashboard/codex/gateway", bytes.NewReader([]byte(`{"path":"`+codexPath+`","id":"deepseek","enabled":true}`)))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", w.Code, w.Body.String())
	}
}
