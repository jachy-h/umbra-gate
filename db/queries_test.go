package db

import (
	"path/filepath"
	"testing"
	"time"
)

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
