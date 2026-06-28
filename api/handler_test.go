package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jachy-h/umbra-gate/agents"
	agentclaude "github.com/jachy-h/umbra-gate/agents/claude"
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

func TestSessionsEndpointSupportsPagination(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	providerID, err := database.EnsureProvider("openai")
	if err != nil {
		t.Fatalf("EnsureProvider() error = %v", err)
	}
	for i := 0; i < 4; i++ {
		if _, err := database.CreateSession(providerID, "gpt-4o"); err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
	}

	handler := New(database, newHandlerTestConfig(t))
	req := httptest.NewRequest(http.MethodGet, "/sessions?limit=2&offset=1", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var sessions []db.Session
	if err := json.NewDecoder(w.Body).Decode(&sessions); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("len(sessions) = %d, want 2", len(sessions))
	}
}

func TestAnalyticsBreakdownEndpointReturnsDimensionStats(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	providerID, err := database.EnsureProvider("openai")
	if err != nil {
		t.Fatalf("EnsureProvider() error = %v", err)
	}
	sessionID, err := database.CreateSessionWithMeta(providerID, "gpt-4o", db.SessionMeta{AgentID: "codex", ProjectID: "gateway-redesign"})
	if err != nil {
		t.Fatalf("CreateSessionWithMeta() error = %v", err)
	}
	if err := database.CompleteSession(sessionID, 100, 50, 1000, nil); err != nil {
		t.Fatalf("CompleteSession() error = %v", err)
	}

	handler := New(database, newHandlerTestConfig(t))
	req := httptest.NewRequest(http.MethodGet, "/analytics/breakdown?range=7d&dimension=project", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var breakdown []db.AnalyticsBreakdown
	if err := json.NewDecoder(w.Body).Decode(&breakdown); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if len(breakdown) != 1 || breakdown[0].Name != "gateway-redesign" || breakdown[0].RequestCount != 1 || breakdown[0].TotalTokens != 150 {
		t.Fatalf("breakdown = %+v, want project stats", breakdown)
	}
}

func TestAnalyticsTimeSeriesEndpointSupportsDimension(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	providerID, err := database.EnsureProvider("openai")
	if err != nil {
		t.Fatalf("EnsureProvider() error = %v", err)
	}
	sessionID, err := database.CreateSessionWithMeta(providerID, "gpt-4o", db.SessionMeta{AgentID: "codex"})
	if err != nil {
		t.Fatalf("CreateSessionWithMeta() error = %v", err)
	}
	if err := database.CompleteSession(sessionID, 100, 50, 1000, nil); err != nil {
		t.Fatalf("CompleteSession() error = %v", err)
	}

	handler := New(database, newHandlerTestConfig(t))
	req := httptest.NewRequest(http.MethodGet, "/analytics/timeseries?range=7d&by=agent", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var stats []db.TimeSeriesBreakdown
	if err := json.NewDecoder(w.Body).Decode(&stats); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if len(stats) != 1 || stats[0].Name != "codex" || stats[0].TotalTokens != 150 {
		t.Fatalf("stats = %+v, want codex time series", stats)
	}
}

func TestAgentsEndpointReturnsAgentStatuses(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	claudePath := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(claudePath, []byte(`{"env":{"ANTHROPIC_BASE_URL":"http://127.0.0.1:4141/a/claude-code/anthropic"}}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	cfg := newHandlerTestConfig(t)
	if err := cfg.UpsertProvider("anthropic", config.ProviderConfig{BaseURL: "https://api.anthropic.com"}); err != nil {
		t.Fatalf("UpsertProvider() error = %v", err)
	}
	registry := agents.NewRegistry(agentclaude.Manager{Path: claudePath})
	handler := NewWithAgents(database, cfg, registry)

	req := httptest.NewRequest(http.MethodGet, "/agents", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var statuses []agents.Status
	if err := json.NewDecoder(w.Body).Decode(&statuses); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if len(statuses) != 1 || statuses[0].AgentID != "claude-code" || len(statuses[0].Bindings) != 1 || !statuses[0].Bindings[0].GatewayEnabled {
		t.Fatalf("statuses = %+v, want enabled claude-code", statuses)
	}
}

func TestAgentEnableEndpointsRejectTemporarilyDisabledProxy(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	claudePath := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(claudePath, []byte(`{"env":{}}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	cfg := newHandlerTestConfig(t)
	if err := cfg.UpsertProvider("anthropic", config.ProviderConfig{BaseURL: "https://api.anthropic.com"}); err != nil {
		t.Fatalf("UpsertProvider() error = %v", err)
	}
	registry := agents.NewRegistry(agentclaude.Manager{Path: claudePath})
	handler := NewWithAgents(database, cfg, registry)

	planReq := httptest.NewRequest(http.MethodPost, "/agents/claude-code/plan", strings.NewReader(`{"enabled":true}`))
	planW := httptest.NewRecorder()
	handler.ServeHTTP(planW, planReq)
	if planW.Code != http.StatusConflict {
		t.Fatalf("plan status = %d, want %d; body = %s", planW.Code, http.StatusConflict, planW.Body.String())
	}

	applyReq := httptest.NewRequest(http.MethodPost, "/agents/claude-code/apply", strings.NewReader(`{"enabled":true,"base_checksum":"unused"}`))
	applyW := httptest.NewRecorder()
	handler.ServeHTTP(applyW, applyReq)
	if applyW.Code != http.StatusConflict {
		t.Fatalf("apply status = %d, want %d; body = %s", applyW.Code, http.StatusConflict, applyW.Body.String())
	}
	written, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(written), "http://127.0.0.1:4141/a/claude-code/anthropic") {
		t.Fatalf("disabled proxy was written: %s", string(written))
	}

	if err := os.WriteFile(claudePath, []byte(`{"env":{"ANTHROPIC_BASE_URL":"http://127.0.0.1:4141/a/claude-code/anthropic","ANTHROPIC_AUTH_TOKEN":"PROXY_MANAGED"}}`), 0o600); err != nil {
		t.Fatalf("WriteFile(managed config) error = %v", err)
	}
	disablePlanReq := httptest.NewRequest(http.MethodPost, "/agents/claude-code/plan", strings.NewReader(`{"enabled":false}`))
	disablePlanW := httptest.NewRecorder()
	handler.ServeHTTP(disablePlanW, disablePlanReq)
	if disablePlanW.Code != http.StatusOK {
		t.Fatalf("disable plan status = %d, body = %s", disablePlanW.Code, disablePlanW.Body.String())
	}
	var disablePlan agents.Plan
	if err := json.NewDecoder(disablePlanW.Body).Decode(&disablePlan); err != nil {
		t.Fatalf("Decode(disable plan) error = %v", err)
	}
	disableApplyReq := httptest.NewRequest(http.MethodPost, "/agents/claude-code/apply", strings.NewReader(`{"enabled":false,"base_checksum":"`+disablePlan.BaseChecksum+`"}`))
	disableApplyW := httptest.NewRecorder()
	handler.ServeHTTP(disableApplyW, disableApplyReq)
	if disableApplyW.Code != http.StatusOK {
		t.Fatalf("disable apply status = %d, body = %s", disableApplyW.Code, disableApplyW.Body.String())
	}
	written, err = os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("ReadFile(disabled config) error = %v", err)
	}
	if strings.Contains(string(written), "ANTHROPIC_BASE_URL") || strings.Contains(string(written), "PROXY_MANAGED") {
		t.Fatalf("managed proxy values were not removed: %s", string(written))
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
	var matched bool
	for _, stat := range stats {
		if stat.RequestCount == 1 && stat.TotalTokens == 150 {
			matched = true
			break
		}
	}
	if !matched {
		t.Fatalf("stats = %+v, want one day with request_count=1 and total_tokens=150", stats)
	}
}
