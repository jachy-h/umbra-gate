package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/jachy-h/umbra-gate/config"
	"github.com/jachy-h/umbra-gate/db"
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

func TestOverviewEndpointReturnsSummaryStats(t *testing.T) {
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
	req := httptest.NewRequest(http.MethodGet, "/overview?range=7d", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var overview db.OverviewStats
	if err := json.NewDecoder(w.Body).Decode(&overview); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if overview.TotalSessions != 1 || overview.TotalRequests != 1 || overview.TotalTokens != 150 || overview.SuccessRate != 1 {
		t.Fatalf("overview = %+v, want one successful request", overview)
	}
}

func TestFailuresEndpointReturnsFailureAnalytics(t *testing.T) {
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
	errMessage := "upstream returned 500"
	if err := database.CompleteSession(sessionID, 0, 0, 1000, &errMessage); err != nil {
		t.Fatalf("CompleteSession() error = %v", err)
	}

	handler := New(database, newHandlerTestConfig(t))
	req := httptest.NewRequest(http.MethodGet, "/failures?range=7d", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var analytics db.FailureAnalytics
	if err := json.NewDecoder(w.Body).Decode(&analytics); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if analytics.TotalFailures != 1 || len(analytics.Categories) != 1 || analytics.Categories[0].Category != "upstream_5xx" || len(analytics.Recent) != 1 {
		t.Fatalf("analytics = %+v, want one upstream_5xx failure", analytics)
	}
}

func TestLatencyEndpointReturnsProviderPercentiles(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	providerID, err := database.EnsureProvider("openai")
	if err != nil {
		t.Fatalf("EnsureProvider() error = %v", err)
	}
	for _, duration := range []int64{100, 200, 300} {
		sessionID, err := database.CreateSession(providerID, "gpt-4o")
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		if err := database.CompleteSession(sessionID, 100, 50, duration, nil); err != nil {
			t.Fatalf("CompleteSession() error = %v", err)
		}
	}

	handler := New(database, newHandlerTestConfig(t))
	req := httptest.NewRequest(http.MethodGet, "/latency?range=7d&by=provider", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var analytics []db.LatencyAnalytics
	if err := json.NewDecoder(w.Body).Decode(&analytics); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if len(analytics) != 1 || analytics[0].Name != "openai" || analytics[0].RequestCount != 3 || analytics[0].P95DurationMs != 300 {
		t.Fatalf("analytics = %+v, want openai latency", analytics)
	}
}

func TestProviderAnalyticsEndpointReturnsReliabilityStats(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	providerID, err := database.EnsureProvider("openai")
	if err != nil {
		t.Fatalf("EnsureProvider() error = %v", err)
	}
	successID, err := database.CreateSession(providerID, "gpt-4o")
	if err != nil {
		t.Fatalf("CreateSession(success) error = %v", err)
	}
	if err := database.CompleteSession(successID, 100, 50, 1000, nil); err != nil {
		t.Fatalf("CompleteSession(success) error = %v", err)
	}
	errorID, err := database.CreateSession(providerID, "gpt-4o")
	if err != nil {
		t.Fatalf("CreateSession(error) error = %v", err)
	}
	errMessage := "upstream failed"
	if err := database.CompleteSession(errorID, 0, 0, 2000, &errMessage); err != nil {
		t.Fatalf("CompleteSession(error) error = %v", err)
	}

	handler := New(database, newHandlerTestConfig(t))
	req := httptest.NewRequest(http.MethodGet, "/providers/analytics?range=7d", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var analytics []db.ProviderAnalytics
	if err := json.NewDecoder(w.Body).Decode(&analytics); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if len(analytics) != 1 || analytics[0].ProviderName != "openai" || analytics[0].RequestCount != 2 || analytics[0].SuccessCount != 1 || analytics[0].ErrorCount != 1 || analytics[0].SuccessRate != 0.5 {
		t.Fatalf("analytics = %+v, want openai mixed reliability", analytics)
	}
}

func TestModelAnalyticsEndpointReturnsReliabilityStats(t *testing.T) {
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
	req := httptest.NewRequest(http.MethodGet, "/models/analytics?range=7d", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var analytics []db.ModelAnalytics
	if err := json.NewDecoder(w.Body).Decode(&analytics); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if len(analytics) != 1 || analytics[0].Model != "gpt-4o" || analytics[0].RequestCount != 1 || analytics[0].SuccessCount != 1 || analytics[0].SuccessRate != 1 {
		t.Fatalf("analytics = %+v, want gpt-4o success", analytics)
	}
}

func TestTimeSeriesEndpointSupportsRangeParameter(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	handler := New(database, newHandlerTestConfig(t))
	req := httptest.NewRequest(http.MethodGet, "/timeseries?range=30d", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var stats []db.TimeSeriesStats
	if err := json.NewDecoder(w.Body).Decode(&stats); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if len(stats) != 30 {
		t.Fatalf("len(stats) = %d, want 30", len(stats))
	}
}

func TestTimeSeriesEndpointFallsBackForInvalidRange(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	handler := New(database, newHandlerTestConfig(t))
	req := httptest.NewRequest(http.MethodGet, "/timeseries?range=invalid", nil)
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
