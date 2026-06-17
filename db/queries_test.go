package db

import (
	"path/filepath"
	"testing"
	"time"
)

func TestParseAnalyticsRange(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want AnalyticsRange
	}{
		{name: "empty defaults", raw: "", want: AnalyticsRange{Value: "7d", Days: 7}},
		{name: "24h", raw: "24h", want: AnalyticsRange{Value: "24h", Days: 1}},
		{name: "7d", raw: "7d", want: AnalyticsRange{Value: "7d", Days: 7}},
		{name: "30d", raw: "30d", want: AnalyticsRange{Value: "30d", Days: 30}},
		{name: "90d", raw: "90d", want: AnalyticsRange{Value: "90d", Days: 90}},
		{name: "invalid defaults", raw: "365d", want: AnalyticsRange{Value: "7d", Days: 7}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseAnalyticsRange(tt.raw)
			if got != tt.want {
				t.Fatalf("ParseAnalyticsRange(%q) = %+v, want %+v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestAnalyticsRangeStartTime(t *testing.T) {
	now := time.Date(2026, 6, 17, 15, 30, 0, 0, time.UTC)

	if got := ParseAnalyticsRange("24h").StartTime(now); !got.Equal(now.Add(-24 * time.Hour)) {
		t.Fatalf("24h StartTime = %s, want %s", got, now.Add(-24*time.Hour))
	}
	want := time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC)
	if got := ParseAnalyticsRange("7d").StartTime(now); !got.Equal(want) {
		t.Fatalf("7d StartTime = %s, want %s", got, want)
	}
}

func TestGetTimeSeriesStatsForRangeUsesAnalyticsRange(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	stats, err := database.GetTimeSeriesStatsForRange("30d")
	if err != nil {
		t.Fatalf("GetTimeSeriesStatsForRange() error = %v", err)
	}
	if len(stats) != 30 {
		t.Fatalf("len(stats) = %d, want 30", len(stats))
	}

	fallback, err := database.GetTimeSeriesStatsForRange("invalid")
	if err != nil {
		t.Fatalf("GetTimeSeriesStatsForRange(invalid) error = %v", err)
	}
	if len(fallback) != 7 {
		t.Fatalf("len(fallback) = %d, want 7", len(fallback))
	}
}

func TestGetProviderStatsAggregatesSuccessfulSessionsByProvider(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	openaiID, err := database.EnsureProvider("openai")
	if err != nil {
		t.Fatalf("EnsureProvider(openai) error = %v", err)
	}
	anthropicID, err := database.EnsureProvider("anthropic")
	if err != nil {
		t.Fatalf("EnsureProvider(anthropic) error = %v", err)
	}

	openaiSession, err := database.CreateSession(openaiID, "gpt-4o")
	if err != nil {
		t.Fatalf("CreateSession(openai) error = %v", err)
	}
	if err := database.CompleteSession(openaiSession, 100, 50, 2000, nil); err != nil {
		t.Fatalf("CompleteSession(openai) error = %v", err)
	}

	anthropicSession, err := database.CreateSession(anthropicID, "claude")
	if err != nil {
		t.Fatalf("CreateSession(anthropic) error = %v", err)
	}
	if err := database.CompleteSession(anthropicSession, 500, 100, 4000, nil); err != nil {
		t.Fatalf("CompleteSession(anthropic) error = %v", err)
	}

	failedSession, err := database.CreateSession(openaiID, "gpt-4o")
	if err != nil {
		t.Fatalf("CreateSession(failed) error = %v", err)
	}
	errMessage := "upstream failed"
	if err := database.CompleteSession(failedSession, 900, 900, 1000, &errMessage); err != nil {
		t.Fatalf("CompleteSession(failed) error = %v", err)
	}

	stats, err := database.GetProviderStats()
	if err != nil {
		t.Fatalf("GetProviderStats() error = %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("len(stats) = %d, want 2", len(stats))
	}
	if stats[0].ProviderName != "anthropic" || stats[0].RequestCount != 1 || stats[0].TotalTokens != 600 || stats[0].AvgDurationMs != 4000 {
		t.Fatalf("stats[0] = %+v, want anthropic totals", stats[0])
	}
	if stats[1].ProviderName != "openai" || stats[1].RequestCount != 1 || stats[1].TotalTokens != 150 || stats[1].AvgDurationMs != 2000 {
		t.Fatalf("stats[1] = %+v, want openai totals", stats[1])
	}
}

func TestGetLatencyAnalyticsByProvider(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	providerID, err := database.EnsureProvider("openai")
	if err != nil {
		t.Fatalf("EnsureProvider() error = %v", err)
	}
	for _, duration := range []int64{100, 200, 300, 400, 500} {
		sessionID, err := database.CreateSession(providerID, "gpt-4o")
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		if err := database.CompleteSession(sessionID, 10, 5, duration, nil); err != nil {
			t.Fatalf("CompleteSession() error = %v", err)
		}
	}
	errorID, err := database.CreateSession(providerID, "gpt-4o")
	if err != nil {
		t.Fatalf("CreateSession(error) error = %v", err)
	}
	errMessage := "upstream failed"
	if err := database.CompleteSession(errorID, 0, 0, 9999, &errMessage); err != nil {
		t.Fatalf("CompleteSession(error) error = %v", err)
	}

	analytics, err := database.GetLatencyAnalytics("7d", "provider")
	if err != nil {
		t.Fatalf("GetLatencyAnalytics() error = %v", err)
	}
	if len(analytics) != 1 {
		t.Fatalf("len(analytics) = %d, want 1", len(analytics))
	}
	if analytics[0].Name != "openai" || analytics[0].RequestCount != 5 || analytics[0].AvgDurationMs != 300 || analytics[0].MedianDurationMs != 300 || analytics[0].P95DurationMs != 500 || analytics[0].P99DurationMs != 500 {
		t.Fatalf("analytics[0] = %+v, want provider latency percentiles", analytics[0])
	}
}

func TestGetLatencyAnalyticsByModel(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	providerID, err := database.EnsureProvider("openai")
	if err != nil {
		t.Fatalf("EnsureProvider() error = %v", err)
	}
	for _, row := range []struct {
		model    string
		duration int64
	}{
		{model: "gpt-4o", duration: 100},
		{model: "gpt-4o", duration: 300},
		{model: "gpt-4.1", duration: 900},
	} {
		sessionID, err := database.CreateSession(providerID, row.model)
		if err != nil {
			t.Fatalf("CreateSession(%s) error = %v", row.model, err)
		}
		if err := database.CompleteSession(sessionID, 10, 5, row.duration, nil); err != nil {
			t.Fatalf("CompleteSession(%s) error = %v", row.model, err)
		}
	}

	analytics, err := database.GetLatencyAnalytics("7d", "model")
	if err != nil {
		t.Fatalf("GetLatencyAnalytics() error = %v", err)
	}
	if len(analytics) != 2 {
		t.Fatalf("len(analytics) = %d, want 2", len(analytics))
	}
	if analytics[0].Name != "gpt-4.1" || analytics[0].RequestCount != 1 || analytics[0].P95DurationMs != 900 {
		t.Fatalf("analytics[0] = %+v, want gpt-4.1 latency", analytics[0])
	}
	if analytics[1].Name != "gpt-4o" || analytics[1].RequestCount != 2 || analytics[1].AvgDurationMs != 200 || analytics[1].MedianDurationMs != 100 || analytics[1].P95DurationMs != 300 {
		t.Fatalf("analytics[1] = %+v, want gpt-4o latency", analytics[1])
	}
}

func TestGetOverviewStatsAggregatesRangeSummary(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	providerID, err := database.EnsureProvider("openai")
	if err != nil {
		t.Fatalf("EnsureProvider() error = %v", err)
	}

	for _, row := range []struct {
		promptTokens     int64
		completionTokens int64
		durationMs       int64
		errMessage       *string
	}{
		{promptTokens: 100, completionTokens: 50, durationMs: 100},
		{promptTokens: 200, completionTokens: 25, durationMs: 300},
		{durationMs: 999, errMessage: stringPtr("upstream failed")},
	} {
		sessionID, err := database.CreateSession(providerID, "gpt-4o")
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		if err := database.CompleteSession(sessionID, row.promptTokens, row.completionTokens, row.durationMs, row.errMessage); err != nil {
			t.Fatalf("CompleteSession() error = %v", err)
		}
	}

	overview, err := database.GetOverviewStats("7d")
	if err != nil {
		t.Fatalf("GetOverviewStats() error = %v", err)
	}
	if overview.TotalSessions != 3 || overview.TotalRequests != 3 || overview.TotalTokens != 375 {
		t.Fatalf("overview totals = %+v, want 3 sessions/requests and 375 tokens", overview)
	}
	if overview.SuccessCount != 2 || overview.ErrorCount != 1 || overview.SuccessRate != float64(2)/float64(3) {
		t.Fatalf("overview reliability = %+v, want 2 success and 1 error", overview)
	}
	if overview.AvgDurationMs != 200 || overview.P95DurationMs != 300 {
		t.Fatalf("overview latency = %+v, want avg 200 and p95 300", overview)
	}
}

func TestGetFailureAnalyticsAggregatesFailures(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	openaiID, err := database.EnsureProvider("openai")
	if err != nil {
		t.Fatalf("EnsureProvider(openai) error = %v", err)
	}
	anthropicID, err := database.EnsureProvider("anthropic")
	if err != nil {
		t.Fatalf("EnsureProvider(anthropic) error = %v", err)
	}

	cases := []struct {
		providerID int64
		model      string
		message    string
		startedAt  string
	}{
		{providerID: openaiID, model: "gpt-4o", message: "context deadline exceeded", startedAt: "2026-06-17 12:03:00"},
		{providerID: openaiID, model: "gpt-4o", message: "upstream returned 500", startedAt: "2026-06-17 12:02:00"},
		{providerID: anthropicID, model: "claude", message: "connection refused", startedAt: "2026-06-17 12:01:00"},
	}
	for _, row := range cases {
		sessionID, err := database.CreateSession(row.providerID, row.model)
		if err != nil {
			t.Fatalf("CreateSession(%s) error = %v", row.model, err)
		}
		if err := database.CompleteSession(sessionID, 0, 0, 1000, stringPtr(row.message)); err != nil {
			t.Fatalf("CompleteSession(%s) error = %v", row.model, err)
		}
		if _, err := database.conn.Exec(`UPDATE sessions SET started_at = ? WHERE id = ?`, row.startedAt, sessionID); err != nil {
			t.Fatalf("update started_at error = %v", err)
		}
	}

	successID, err := database.CreateSession(openaiID, "gpt-4o")
	if err != nil {
		t.Fatalf("CreateSession(success) error = %v", err)
	}
	if err := database.CompleteSession(successID, 100, 50, 1000, nil); err != nil {
		t.Fatalf("CompleteSession(success) error = %v", err)
	}

	oldID, err := database.CreateSession(openaiID, "gpt-4o")
	if err != nil {
		t.Fatalf("CreateSession(old) error = %v", err)
	}
	if err := database.CompleteSession(oldID, 0, 0, 1000, stringPtr("old timeout")); err != nil {
		t.Fatalf("CompleteSession(old) error = %v", err)
	}
	oldStartedAt := time.Now().AddDate(0, 0, -10).Format("2006-01-02 15:04:05")
	if _, err := database.conn.Exec(`UPDATE sessions SET started_at = ? WHERE id = ?`, oldStartedAt, oldID); err != nil {
		t.Fatalf("update old started_at error = %v", err)
	}

	analytics, err := database.GetFailureAnalytics("7d")
	if err != nil {
		t.Fatalf("GetFailureAnalytics() error = %v", err)
	}
	if analytics.TotalFailures != 3 {
		t.Fatalf("TotalFailures = %d, want 3", analytics.TotalFailures)
	}
	if len(analytics.Categories) != 3 {
		t.Fatalf("len(Categories) = %d, want 3", len(analytics.Categories))
	}
	if analytics.Categories[0].Category != "network_error" || analytics.Categories[1].Category != "timeout" || analytics.Categories[2].Category != "upstream_5xx" {
		t.Fatalf("Categories = %+v, want sorted categories", analytics.Categories)
	}
	if len(analytics.Providers) != 2 || analytics.Providers[0].Name != "openai" || analytics.Providers[0].Count != 2 || analytics.Providers[1].Name != "anthropic" || analytics.Providers[1].Count != 1 {
		t.Fatalf("Providers = %+v, want openai then anthropic", analytics.Providers)
	}
	if len(analytics.Models) != 2 || analytics.Models[0].Name != "gpt-4o" || analytics.Models[0].Count != 2 || analytics.Models[1].Name != "claude" || analytics.Models[1].Count != 1 {
		t.Fatalf("Models = %+v, want gpt-4o then claude", analytics.Models)
	}
	if len(analytics.Recent) != 3 || analytics.Recent[0].ErrorMessage == nil || *analytics.Recent[0].ErrorMessage != "context deadline exceeded" {
		t.Fatalf("Recent = %+v, want newest failure first", analytics.Recent)
	}
}

func TestGetProviderAnalyticsAggregatesReliability(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	openaiID, err := database.EnsureProvider("openai")
	if err != nil {
		t.Fatalf("EnsureProvider(openai) error = %v", err)
	}
	anthropicID, err := database.EnsureProvider("anthropic")
	if err != nil {
		t.Fatalf("EnsureProvider(anthropic) error = %v", err)
	}

	openaiSuccess, err := database.CreateSession(openaiID, "gpt-4o")
	if err != nil {
		t.Fatalf("CreateSession(openai success) error = %v", err)
	}
	if err := database.CompleteSession(openaiSuccess, 100, 50, 1000, nil); err != nil {
		t.Fatalf("CompleteSession(openai success) error = %v", err)
	}

	openaiError, err := database.CreateSession(openaiID, "gpt-4o")
	if err != nil {
		t.Fatalf("CreateSession(openai error) error = %v", err)
	}
	errMessage := "upstream failed"
	if err := database.CompleteSession(openaiError, 0, 0, 2000, &errMessage); err != nil {
		t.Fatalf("CompleteSession(openai error) error = %v", err)
	}

	anthropicSuccess, err := database.CreateSession(anthropicID, "claude")
	if err != nil {
		t.Fatalf("CreateSession(anthropic) error = %v", err)
	}
	if err := database.CompleteSession(anthropicSuccess, 500, 100, 3000, nil); err != nil {
		t.Fatalf("CompleteSession(anthropic) error = %v", err)
	}

	oldSession, err := database.CreateSession(openaiID, "gpt-4o")
	if err != nil {
		t.Fatalf("CreateSession(old) error = %v", err)
	}
	if err := database.CompleteSession(oldSession, 1000, 1000, 4000, nil); err != nil {
		t.Fatalf("CompleteSession(old) error = %v", err)
	}
	oldStartedAt := time.Now().AddDate(0, 0, -10).Format("2006-01-02 15:04:05")
	if _, err := database.conn.Exec(`UPDATE sessions SET started_at = ? WHERE id = ?`, oldStartedAt, oldSession); err != nil {
		t.Fatalf("update old session error = %v", err)
	}

	analytics, err := database.GetProviderAnalytics("7d")
	if err != nil {
		t.Fatalf("GetProviderAnalytics() error = %v", err)
	}
	if len(analytics) != 2 {
		t.Fatalf("len(analytics) = %d, want 2", len(analytics))
	}
	if analytics[0].ProviderName != "anthropic" || analytics[0].RequestCount != 1 || analytics[0].SuccessCount != 1 || analytics[0].ErrorCount != 0 || analytics[0].SuccessRate != 1 {
		t.Fatalf("analytics[0] = %+v, want anthropic success", analytics[0])
	}
	if analytics[1].ProviderName != "openai" || analytics[1].RequestCount != 2 || analytics[1].SuccessCount != 1 || analytics[1].ErrorCount != 1 || analytics[1].SuccessRate != 0.5 || analytics[1].ErrorRate != 0.5 || analytics[1].TotalTokens != 150 {
		t.Fatalf("analytics[1] = %+v, want openai mixed reliability", analytics[1])
	}
}

func TestGetModelAnalyticsAggregatesReliability(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	providerID, err := database.EnsureProvider("openai")
	if err != nil {
		t.Fatalf("EnsureProvider() error = %v", err)
	}

	for _, row := range []struct {
		model            string
		promptTokens     int64
		completionTokens int64
		durationMs       int64
		errMessage       *string
	}{
		{model: "gpt-4o", promptTokens: 100, completionTokens: 50, durationMs: 1000},
		{model: "gpt-4o", durationMs: 2000, errMessage: stringPtr("upstream failed")},
		{model: "gpt-4.1", promptTokens: 20, completionTokens: 10, durationMs: 500},
	} {
		sessionID, err := database.CreateSession(providerID, row.model)
		if err != nil {
			t.Fatalf("CreateSession(%s) error = %v", row.model, err)
		}
		if err := database.CompleteSession(sessionID, row.promptTokens, row.completionTokens, row.durationMs, row.errMessage); err != nil {
			t.Fatalf("CompleteSession(%s) error = %v", row.model, err)
		}
	}

	analytics, err := database.GetModelAnalytics("7d")
	if err != nil {
		t.Fatalf("GetModelAnalytics() error = %v", err)
	}
	if len(analytics) != 2 {
		t.Fatalf("len(analytics) = %d, want 2", len(analytics))
	}
	if analytics[0].Model != "gpt-4o" || analytics[0].RequestCount != 2 || analytics[0].SuccessCount != 1 || analytics[0].ErrorCount != 1 || analytics[0].SuccessRate != 0.5 || analytics[0].TotalTokens != 150 {
		t.Fatalf("analytics[0] = %+v, want gpt-4o mixed reliability", analytics[0])
	}
	if analytics[1].Model != "gpt-4.1" || analytics[1].RequestCount != 1 || analytics[1].SuccessCount != 1 || analytics[1].ErrorCount != 0 || analytics[1].SuccessRate != 1 || analytics[1].TotalTokens != 30 {
		t.Fatalf("analytics[1] = %+v, want gpt-4.1 success", analytics[1])
	}
}

func stringPtr(s string) *string {
	return &s
}

func TestGetTimeSeriesStatsFillsLastSevenDays(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "router.db"))
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

	today := time.Now()
	oldest := today.AddDate(0, 0, -6).Format("2006-01-02")
	yesterday := today.AddDate(0, 0, -1).Format("2006-01-02")
	todayDate := today.Format("2006-01-02")

	for _, row := range []struct {
		date             string
		promptTokens     int64
		completionTokens int64
	}{
		{yesterday, 100, 50},
		{yesterday, 10, 5},
		{todayDate, 200, 25},
	} {
		_, err := database.conn.Exec(
			`INSERT INTO requests (session_id, prompt_tokens, completion_tokens, duration_ms, created_at) VALUES (?, ?, ?, 100, ?)`,
			sessionID, row.promptTokens, row.completionTokens, row.date+" 12:00:00",
		)
		if err != nil {
			t.Fatalf("insert request error = %v", err)
		}
	}

	stats, err := database.GetTimeSeriesStats(7)
	if err != nil {
		t.Fatalf("GetTimeSeriesStats() error = %v", err)
	}
	if len(stats) != 7 {
		t.Fatalf("len(stats) = %d, want 7", len(stats))
	}
	if stats[0].Date != oldest || stats[0].RequestCount != 0 || stats[0].TotalTokens != 0 {
		t.Fatalf("stats[0] = %+v, want empty oldest day %s", stats[0], oldest)
	}
	if stats[5].Date != yesterday || stats[5].RequestCount != 2 || stats[5].TotalTokens != 165 {
		t.Fatalf("stats[5] = %+v, want yesterday totals", stats[5])
	}
	if stats[6].Date != todayDate || stats[6].RequestCount != 1 || stats[6].TotalTokens != 225 {
		t.Fatalf("stats[6] = %+v, want today totals", stats[6])
	}
}

func TestRequestLogsRetainOnlyMostRecent(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	providerID, err := database.EnsureProvider("opencode")
	if err != nil {
		t.Fatalf("EnsureProvider error = %v", err)
	}
	sessionID, err := database.CreateSession(providerID, "test-model")
	if err != nil {
		t.Fatalf("CreateSession error = %v", err)
	}

	for i := 0; i < MaxRequestLogs+5; i++ {
		if _, err := database.InsertRequestLog(RequestLog{
			SessionID:      sessionID,
			ProviderName:   "opencode",
			Method:         "POST",
			URL:            "https://example.com/v1/chat/completions",
			RequestHeaders: "Content-Type: application/json",
			RequestBody:    "{}",
			ResponseStatus: 200,
			ResponseBody:   "{}",
			DurationMs:     100,
		}); err != nil {
			t.Fatalf("InsertRequestLog error = %v", err)
		}
	}

	logs, err := database.ListRecentRequestLogs()
	if err != nil {
		t.Fatalf("ListRecentRequestLogs error = %v", err)
	}
	if len(logs) != MaxRequestLogs {
		t.Fatalf("retained %d logs, want %d", len(logs), MaxRequestLogs)
	}

	// Newest first; the IDs should be the last MaxRequestLogs IDs we inserted.
	wantNewest := int64(MaxRequestLogs + 5)
	if logs[0].ID != wantNewest {
		t.Fatalf("logs[0].ID = %d, want %d", logs[0].ID, wantNewest)
	}
}

func TestGetRequestLogBySessionReturnsLatest(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	providerID, _ := database.EnsureProvider("opencode")
	sessionID, _ := database.CreateSession(providerID, "test-model")

	first, err := database.InsertRequestLog(RequestLog{
		SessionID:    sessionID,
		ProviderName: "opencode",
		Method:       "POST",
		ResponseBody: "first",
	})
	if err != nil {
		t.Fatalf("InsertRequestLog error = %v", err)
	}
	second, err := database.InsertRequestLog(RequestLog{
		SessionID:    sessionID,
		ProviderName: "opencode",
		Method:       "POST",
		ResponseBody: "second",
	})
	if err != nil {
		t.Fatalf("InsertRequestLog error = %v", err)
	}
	if second <= first {
		t.Fatalf("second id %d should be greater than first %d", second, first)
	}

	got, err := database.GetRequestLogBySession(sessionID)
	if err != nil {
		t.Fatalf("GetRequestLogBySession error = %v", err)
	}
	if got.ResponseBody != "second" {
		t.Fatalf("got body %q, want %q", got.ResponseBody, "second")
	}
}
