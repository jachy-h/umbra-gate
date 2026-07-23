package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/jachy-h/llm-gateway-lite/internal/config"
	"github.com/jachy-h/llm-gateway-lite/internal/db"
	"github.com/jachy-h/llm-gateway-lite/internal/models"
	"github.com/jachy-h/llm-gateway-lite/internal/stats"
)

func TestStatsEndpointAggregatesLatestRequestsOnDemand(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	stats.New(database).Record(models.RequestLog{
		LinkID: "link-1", Path: "test", ProviderID: "provider-1", ProviderName: "Provider",
		StatusCode: http.StatusOK, Success: true, LatencyMS: 25,
		Attributes: models.Map{"_request_type": "link_validation"}, CreatedAt: time.Now(),
	})
	engine, _ := New(config.Config{}, database)
	request := httptest.NewRequest(http.MethodGet, "/admin/stats", nil)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload struct {
		Stats []map[string]any `json:"stats"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Stats) == 0 || payload.Stats[0]["Total"] != float64(1) {
		t.Fatalf("link test request was not aggregated: %s", response.Body.String())
	}
}

func TestStatsEndpointAggregatesLogsCreatedAfterExistingCursor(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	service := stats.New(database)
	service.Record(models.RequestLog{
		LinkID: "link-1", Path: "test", ProviderID: "provider-1", ProviderName: "Provider",
		StatusCode: http.StatusOK, Success: true, LatencyMS: 25, CreatedAt: time.Now(),
	})
	if err := service.Aggregate(t.Context()); err != nil {
		t.Fatal(err)
	}

	// Older versions stored timestamps in SQLite's space-separated form, while
	// the aggregation cursor used RFC3339. The next log must not be skipped.
	time.Sleep(10 * time.Millisecond)
	legacyCreatedAt := time.Now().UTC().Format("2006-01-02 15:04:05.999999999-07:00")
	if _, err := database.Exec(`INSERT INTO request_logs(id,link_id,path,provider_id,provider_name,created_at)
		VALUES('legacy-log','link-1','test','provider-1','Provider',?)`, legacyCreatedAt); err != nil {
		t.Fatal(err)
	}

	engine, _ := New(config.Config{}, database)
	request := httptest.NewRequest(http.MethodGet, "/admin/stats", nil)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload struct {
		Stats []map[string]any `json:"stats"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	var total float64
	for _, row := range payload.Stats {
		total += row["Total"].(float64)
	}
	if total != 2 {
		t.Fatalf("total = %v, want 2; body = %s", total, response.Body.String())
	}
}
