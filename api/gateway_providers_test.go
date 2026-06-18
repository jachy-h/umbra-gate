package api

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

func newGatewayTestSetup(t *testing.T, yamlBody string) (*Handler, *config.Config, string) {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(yamlBody), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	database, err := db.Open(filepath.Join(dir, "router.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return New(database, cfg), cfg, configPath
}

func TestGatewayProvidersList(t *testing.T) {
	t.Setenv("FOO_KEY", "secret123")
	h, _, _ := newGatewayTestSetup(t, `
providers:
  openai:
    type: openai
    base_url: https://api.openai.com
    api_key: ${FOO_KEY}
`)
	req := httptest.NewRequest(http.MethodGet, "/gateway/providers", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	var out []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("json: %v body=%s", err, w.Body.String())
	}
	if len(out) != 1 {
		t.Fatalf("len = %d", len(out))
	}
	got := out[0]
	if got["id"] != "openai" || got["type"] != "openai" || got["base_url"] != "https://api.openai.com" {
		t.Errorf("entry = %+v", got)
	}
	if got["api_key"] != "" {
		t.Errorf("api_key should not be returned, got %q", got["api_key"])
	}
	if got["api_key_source"] != "${FOO_KEY}" {
		t.Errorf("api_key_source = %q, want literal env reference", got["api_key_source"])
	}
	if got["has_api_key"] != true {
		t.Errorf("has_api_key = %v, want true", got["has_api_key"])
	}
	// Body must never include the secret
	if bytes.Contains(w.Body.Bytes(), []byte("secret123")) {
		t.Errorf("response leaked secret: %s", w.Body.String())
	}
}

func TestGatewayProvidersCreate(t *testing.T) {
	h, cfg, configPath := newGatewayTestSetup(t, "providers: {}\n")
	body := `{
		"id": "openai",
		"type": "openai",
		"base_url": "https://api.openai.com",
		"api_key": "sk-real"
	}`
	req := httptest.NewRequest(http.MethodPost, "/gateway/providers", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	p, ok := cfg.Provider("openai")
	if !ok {
		t.Fatalf("provider not stored")
	}
	if p.APIKey != "sk-real" || p.APIKeyRaw != "sk-real" || p.BaseURL != "https://api.openai.com" {
		t.Errorf("provider stored wrong: %+v", p)
	}
	data, _ := os.ReadFile(configPath)
	if !strings.Contains(string(data), "openai") {
		t.Errorf("config not persisted:\n%s", data)
	}
}

func TestGatewayProvidersCreateRejectsDuplicate(t *testing.T) {
	h, _, _ := newGatewayTestSetup(t, `
providers:
  openai:
    type: openai
    base_url: https://api.openai.com
    api_key: k
`)
	body := `{"id":"openai","type":"openai","base_url":"https://api.openai.com","api_key":"k"}`
	req := httptest.NewRequest(http.MethodPost, "/gateway/providers", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Errorf("status = %d body=%s", w.Code, w.Body.String())
	}
}

func TestGatewayProvidersCreateValidates(t *testing.T) {
	h, _, _ := newGatewayTestSetup(t, "providers: {}\n")
	cases := map[string]string{
		"missing id":     `{"type":"openai","base_url":"https://x","api_key":"k"}`,
		"bad type":       `{"id":"p","type":"bogus","base_url":"https://x","api_key":"k"}`,
		"missing base":   `{"id":"p","type":"openai","api_key":"k"}`,
		"missing key":    `{"id":"p","type":"openai","base_url":"https://x"}`,
		"unset env ref":  `{"id":"p","type":"openai","base_url":"https://x","api_key":"${UNSET_VAR_AAA}"}`,
		"malformed json": `{`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/gateway/providers", strings.NewReader(body))
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code < 400 || w.Code >= 500 {
				t.Errorf("status = %d body=%s", w.Code, w.Body.String())
			}
		})
	}
}

func TestGatewayProvidersUpdatePreservesKeyWhenBlank(t *testing.T) {
	h, cfg, _ := newGatewayTestSetup(t, `
providers:
  openai:
    type: openai
    base_url: https://api.openai.com
    api_key: existing
`)
	body := `{
		"type": "openai",
		"base_url": "https://api.openai.com/v1",
		"api_key": ""
	}`
	req := httptest.NewRequest(http.MethodPut, "/gateway/providers/openai", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	p, _ := cfg.Provider("openai")
	if p.APIKey != "existing" {
		t.Errorf("api_key clobbered: %q", p.APIKey)
	}
	if p.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("base_url not updated: %q", p.BaseURL)
	}
}

func TestGatewayProvidersUpdateChangesKey(t *testing.T) {
	h, cfg, _ := newGatewayTestSetup(t, `
providers:
  openai:
    type: openai
    base_url: https://api.openai.com
    api_key: old
`)
	body := `{"type":"openai","base_url":"https://api.openai.com","api_key":"new"}`
	req := httptest.NewRequest(http.MethodPut, "/gateway/providers/openai", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	p, _ := cfg.Provider("openai")
	if p.APIKey != "new" {
		t.Errorf("api_key not updated: %q", p.APIKey)
	}
}

func TestGatewayProvidersUpdateNotFound(t *testing.T) {
	h, _, _ := newGatewayTestSetup(t, "providers: {}\n")
	body := `{"type":"openai","base_url":"https://x","api_key":"k"}`
	req := httptest.NewRequest(http.MethodPut, "/gateway/providers/missing", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d", w.Code)
	}
}

func TestGatewayProvidersDelete(t *testing.T) {
	h, cfg, configPath := newGatewayTestSetup(t, `
providers:
  openai:
    type: openai
    base_url: https://api.openai.com
    api_key: k
`)
	req := httptest.NewRequest(http.MethodDelete, "/gateway/providers/openai", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if _, ok := cfg.Provider("openai"); ok {
		t.Errorf("provider still in memory")
	}
	data, _ := os.ReadFile(configPath)
	if strings.Contains(string(data), "openai:") {
		t.Errorf("config still has provider:\n%s", data)
	}
}

func TestGatewayProvidersDeleteNotFound(t *testing.T) {
	h, _, _ := newGatewayTestSetup(t, "providers: {}\n")
	req := httptest.NewRequest(http.MethodDelete, "/gateway/providers/missing", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d", w.Code)
	}
}


