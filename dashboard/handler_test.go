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

func TestHomeRendersIconStatsAndUsageBreakdowns(t *testing.T) {
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
	if strings.Contains(body, `<h1 style="margin-bottom:24px;font-size:24px;">Sessions</h1>`) || strings.Contains(body, `<h1 style="margin-bottom:24px;font-size:24px;">Models</h1>`) {
		t.Fatalf("home rendered the wrong page content: %s", body)
	}
	for _, want := range []string{
		`class="page-title"`,
		`class="dashboard-metrics"`,
		"tokensByProvider",
		"tokensByModel",
		"noUsageYet",
		"stat-desc",
		"analyticsRange",
		"cdn.jsdelivr.net/npm/chart.js",
		`id="usageTrendChart"`,
		`id="languageToggle"`,
		`data-i18n="dashboard"`,
		"type=\"module\" src=\"/dashboard/static/dashboard/home.js\"",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("home body does not contain %q", want)
		}
	}
}

func TestDashboardStaticModulesAreServed(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	handler := New(database, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/static/dashboard/home.js", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "vue.esm-browser.prod.js") {
		t.Fatalf("static module missing Vue CDN import: %s", w.Body.String())
	}
}

func TestFailuresPageRendersAnalyticsUI(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	handler := New(database, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/failures", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{"class=\"page-title\">Failures", "failureAnalyticsRange", "failureSummary", "failureCategories", "failureProviders", "failureModels", "recentFailures", "/dashboard/static/dashboard/failures.js"} {
		if !strings.Contains(body, want) {
			t.Fatalf("failures page missing %q: %s", want, body)
		}
	}
}

func TestProvidersPageRendersManagementUI(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()
	handler := newWithOptions(database, Options{OpencodeConfigPath: filepath.Join(t.TempDir(), "opencode.json")})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/providers", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{"class=\"page-title\">Providers", "Provider Analytics", "Management", "providerAnalyticsRange", "providerTokenChart", "providerSuccessChart", "providerAnalyticsContainer", "gateway forwarding", "providerTableContainer", "provider-management-table", "/dashboard/static/dashboard/providers.js", "@picocss/pico"} {
		if !strings.Contains(body, want) {
			t.Fatalf("providers page missing %q: %s", want, body)
		}
	}
	for _, notWant := range []string{"id=\"models\"", "Default Model", "Small Model", "Gateway</a>", "Step 1", "Step 2", "Step 3", "providerSelect", "editApiKey", "editBaseUrl", "previewBtn", "applyBtn", "saveBtn", "addBtn", "diffPreview", "Edit Gateway", "Edit Provider", "Add Provider"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("providers page should not contain %q: %s", notWant, body)
		}
	}
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
	if !strings.Contains(w.Body.String(), "http://127.0.0.1:4141/openai") {
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
	if !strings.Contains(string(written), "\"baseURL\": \"http://127.0.0.1:4141/openai\"") {
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
