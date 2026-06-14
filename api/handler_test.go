package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/anomalyco/llm-gateway/config"
	"github.com/anomalyco/llm-gateway/db"
)

func newHandlerTestConfig(t *testing.T) *config.Config {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("providers: {}\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return cfg
}

func TestProvidersEndpointReturnsProviderStats(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	providerID, err := database.EnsureProvider("openai")
	if err != nil {
		t.Fatalf("EnsureProvider() error = %v", err)
	}
	sessionID, err := database.CreateSession(providerID, "gpt-4o")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if err := database.CompleteSession(sessionID, 100, 50, 1000, nil); err != nil {
		t.Fatalf("CompleteSession() error = %v", err)
	}

	handler := New(database, newHandlerTestConfig(t))
	req := httptest.NewRequest(http.MethodGet, "/providers", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var stats []db.ProviderStats
	if err := json.NewDecoder(w.Body).Decode(&stats); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("len(stats) = %d, want 1", len(stats))
	}
	if stats[0].ProviderName != "openai" || stats[0].RequestCount != 1 || stats[0].TotalTokens != 150 {
		t.Fatalf("stats[0] = %+v, want openai totals", stats[0])
	}
}

func TestTimeSeriesEndpointReturnsDailyStats(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	providerID, err := database.EnsureProvider("openai")
	if err != nil {
		t.Fatalf("EnsureProvider() error = %v", err)
	}
	sessionID, err := database.CreateSession(providerID, "gpt-4o")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if _, err := database.CreateRequest(sessionID, 100, 50, 1000, nil); err != nil {
		t.Fatalf("CreateRequest() error = %v", err)
	}

	handler := New(database, newHandlerTestConfig(t))
	req := httptest.NewRequest(http.MethodGet, "/timeseries?days=7", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var stats []db.TimeSeriesStats
	if err := json.NewDecoder(w.Body).Decode(&stats); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if len(stats) != 7 {
		t.Fatalf("len(stats) = %d, want 7", len(stats))
	}
	if stats[6].RequestCount != 1 || stats[6].TotalTokens != 150 {
		t.Fatalf("stats[6] = %+v, want today totals", stats[6])
	}
}
