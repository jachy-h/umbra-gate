package proxy

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/jachy-h/umbra-gate/config"
	"github.com/jachy-h/umbra-gate/db"
)

type capturedRequest struct {
	method  string
	url     *url.URL
	headers http.Header
	body    []byte
}

func newFakeUpstream(t *testing.T, status int, body string) (*httptest.Server, *capturedRequest) {
	t.Helper()
	captured := &capturedRequest{}
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		captured.method = r.Method
		captured.url = r.URL
		captured.headers = r.Header.Clone()
		captured.body, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, captured
}

func newTestDB(t *testing.T) *db.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func newTestConfig(t *testing.T, providerID string, p config.ProviderConfig) *config.Config {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("providers: {}\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if err := cfg.UpsertProvider(providerID, p); err != nil {
		t.Fatalf("UpsertProvider: %v", err)
	}
	return cfg
}

func TestProxyOpenAIPathJoinAndAuth(t *testing.T) {
	upstream, captured := newFakeUpstream(t, 200, `{"model":"gpt-4o","usage":{"prompt_tokens":10,"completion_tokens":20,"total_tokens":30}}`)
	cfg := newTestConfig(t, "openai", config.ProviderConfig{
		Type:      config.ProviderTypeOpenAI,
		BaseURL:   upstream.URL + "/v1/", // trailing slash should be tolerated
		APIKey:    "sk-real",
		APIKeyRaw: "sk-real",
	})
	p := New(cfg, newTestDB(t))

	body := `{"model":"gpt-4o","messages":[]}`
	req := httptest.NewRequest("POST", "/openai/chat/completions?foo=bar", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer client-fake")
	req.Header.Set("X-Custom", "passthrough")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if got := captured.url.Path; got != "/v1/chat/completions" {
		t.Errorf("upstream path = %q, want /v1/chat/completions", got)
	}
	if got := captured.url.RawQuery; got != "foo=bar" {
		t.Errorf("query = %q, want foo=bar", got)
	}
	if got := captured.headers.Get("Authorization"); got != "Bearer sk-real" {
		t.Errorf("Authorization = %q, want Bearer sk-real (gateway must replace client key)", got)
	}
	if got := captured.headers.Get("X-Custom"); got != "passthrough" {
		t.Errorf("client header X-Custom not forwarded: %q", got)
	}
	if !bytes.Equal(captured.body, []byte(body)) {
		t.Errorf("body altered: %s", captured.body)
	}
}

func TestProxyAnthropicPathJoinAndAuth(t *testing.T) {
	upstream, captured := newFakeUpstream(t, 200, `{"model":"claude","usage":{"input_tokens":5,"output_tokens":7}}`)
	cfg := newTestConfig(t, "anthropic", config.ProviderConfig{
		Type:      config.ProviderTypeAnthropic,
		BaseURL:   upstream.URL,
		APIKey:    "ak-real",
		APIKeyRaw: "ak-real",
	})
	p := New(cfg, newTestDB(t))

	req := httptest.NewRequest("POST", "/anthropic/v1/messages", strings.NewReader(`{"model":"claude"}`))
	req.Header.Set("x-api-key", "client-fake")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if got := captured.url.Path; got != "/v1/messages" {
		t.Errorf("upstream path = %q", got)
	}
	if got := captured.headers.Get("x-api-key"); got != "ak-real" {
		t.Errorf("x-api-key = %q, want ak-real", got)
	}
	if got := captured.headers.Get("anthropic-version"); got == "" {
		t.Errorf("anthropic-version should be auto-injected")
	}
	if got := captured.headers.Get("Authorization"); got != "" {
		t.Errorf("Authorization should not be set for anthropic, got %q", got)
	}
}

func TestProxyAnthropicRespectsClientVersion(t *testing.T) {
	upstream, captured := newFakeUpstream(t, 200, `{}`)
	cfg := newTestConfig(t, "anthropic", config.ProviderConfig{
		Type:      config.ProviderTypeAnthropic,
		BaseURL:   upstream.URL,
		APIKey:    "ak",
		APIKeyRaw: "ak",
	})
	p := New(cfg, newTestDB(t))

	req := httptest.NewRequest("POST", "/anthropic/v1/messages", strings.NewReader(`{}`))
	req.Header.Set("anthropic-version", "2024-10-22")
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	if got := captured.headers.Get("anthropic-version"); got != "2024-10-22" {
		t.Errorf("client anthropic-version overwritten: %q", got)
	}
}

func TestProxyHopByHopHeadersStripped(t *testing.T) {
	upstream, captured := newFakeUpstream(t, 200, `{}`)
	cfg := newTestConfig(t, "openai", config.ProviderConfig{
		Type:      config.ProviderTypeOpenAI,
		BaseURL:   upstream.URL,
		APIKey:    "sk",
		APIKeyRaw: "sk",
	})
	p := New(cfg, newTestDB(t))

	req := httptest.NewRequest("POST", "/openai/chat/completions", strings.NewReader(`{}`))
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Proxy-Authorization", "junk")
	req.Header.Set("Te", "trailers")
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	for _, h := range []string{"Connection", "Proxy-Authorization", "Te"} {
		if v := captured.headers.Get(h); v != "" {
			t.Errorf("hop-by-hop header %s leaked upstream: %q", h, v)
		}
	}
}

func TestProxyUnknownProvider(t *testing.T) {
	cfg := newTestConfig(t, "ignored", config.ProviderConfig{
		Type:      config.ProviderTypeOpenAI,
		BaseURL:   "https://example.com",
		APIKey:    "k",
		APIKeyRaw: "k",
	})
	p := New(cfg, newTestDB(t))
	req := httptest.NewRequest("POST", "/unknown/v1/chat/completions", nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestProxyPassthroughPreservesClientAuthorization(t *testing.T) {
	upstream, captured := newFakeUpstream(t, 200, `{"usage":{"prompt_tokens":1,"completion_tokens":2}}`)
	cfg := newTestConfig(t, "github-copilot", config.ProviderConfig{
		BaseURL: upstream.URL,
	})
	p := New(cfg, newTestDB(t))

	req := httptest.NewRequest("POST", "/github-copilot/chat/completions", strings.NewReader(`{"model":"gpt-4o-mini","messages":[]}`))
	req.Header.Set("Authorization", "Bearer client-oauth-token")
	req.Header.Set("Editor-Version", "vscode/1.100.0")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if got := captured.headers.Get("Authorization"); got != "Bearer client-oauth-token" {
		t.Fatalf("Authorization = %q, want client token preserved", got)
	}
	if got := captured.headers.Get("Editor-Version"); got != "vscode/1.100.0" {
		t.Fatalf("Editor-Version = %q", got)
	}
	if got := captured.url.Path; got != "/chat/completions" {
		t.Fatalf("upstream path = %q", got)
	}
}

func TestProxyConfigChangesAreLiveReloaded(t *testing.T) {
	upstream, captured := newFakeUpstream(t, 200, `{}`)
	cfg := newTestConfig(t, "openai", config.ProviderConfig{
		Type:      config.ProviderTypeOpenAI,
		BaseURL:   upstream.URL,
		APIKey:    "sk-old",
		APIKeyRaw: "sk-old",
	})
	p := New(cfg, newTestDB(t))

	// Mutate after construction
	if err := cfg.UpsertProvider("openai", config.ProviderConfig{
		Type:      config.ProviderTypeOpenAI,
		BaseURL:   upstream.URL,
		APIKey:    "sk-new",
		APIKeyRaw: "sk-new",
	}); err != nil {
		t.Fatalf("UpsertProvider: %v", err)
	}

	req := httptest.NewRequest("POST", "/openai/chat/completions", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)
	if got := captured.headers.Get("Authorization"); got != "Bearer sk-new" {
		t.Errorf("did not pick up updated key: %q", got)
	}
}

func TestProxyForwardsUpstreamErrorBody(t *testing.T) {
	upstream, _ := newFakeUpstream(t, 401, `{"error":"bad key"}`)
	cfg := newTestConfig(t, "openai", config.ProviderConfig{
		Type:      config.ProviderTypeOpenAI,
		BaseURL:   upstream.URL,
		APIKey:    "k",
		APIKeyRaw: "k",
	})
	p := New(cfg, newTestDB(t))
	req := httptest.NewRequest("POST", "/openai/chat/completions", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Errorf("status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "bad key") {
		t.Errorf("body = %s", w.Body.String())
	}
}

// --- helpers ---
