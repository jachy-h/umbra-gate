package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anomalyco/llm-gateway/config"
)

func TestProxyUnknownProvider(t *testing.T) {
	cfg := &config.Config{
		Listen:    "127.0.0.1:4141",
		Providers: map[string]config.ProviderConfig{},
	}
	p := New(cfg, nil)

	req := httptest.NewRequest("POST", "/unknown/v1/chat/completions", nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestProxyDashboardRoute(t *testing.T) {
	cfg := &config.Config{
		Listen:    "127.0.0.1:4141",
		Providers: map[string]config.ProviderConfig{},
	}
	p := New(cfg, nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 (dashboard not handled by proxy), got %d", w.Code)
	}
}
