package db

import (
	"path/filepath"
	"testing"
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
