package stats

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jachy-h/llm-gateway-lite/internal/db"
	"github.com/jachy-h/llm-gateway-lite/internal/models"
)

type Service struct {
	DB *db.DB
}

func New(d *db.DB) *Service { return &Service{DB: d} }

// Record writes a request log synchronously. Failures are logged but do not
// propagate to the caller so request handling stays unaffected.
func (s *Service) Record(l models.RequestLog) {
	if l.ID == "" {
		l.ID = uuid.NewString()
	}
	if l.CreatedAt.IsZero() {
		l.CreatedAt = time.Now()
	}
	if err := s.DB.InsertLog(l); err != nil {
		log.Printf("stats: record log: %v", err)
	}
}

// Aggregate consumes logs created after the last aggregation cursor and folds
// them into the stats_hourly table, bucketed per proxy link and per attribute.
func (s *Service) Aggregate(ctx context.Context) error {
	since, _ := s.DB.GetMeta("last_aggregated_at")
	if since == "" {
		since = time.Unix(0, 0).UTC().Format(time.RFC3339Nano)
	}
	end := time.Now().UTC()
	rows, err := s.DB.Query(`
		SELECT link_id, provider_id, model, status_code, latency_ms, success, attributes_json, created_at
		FROM request_logs
		WHERE julianday(created_at) > julianday(?) AND julianday(created_at) <= julianday(?)
		ORDER BY created_at`, since, end.Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	type key struct {
		linkID, providerID, attrKey, attrValue, period string
	}
	buckets := map[key]models.Stats{}
	for rows.Next() {
		var (
			linkID, providerID, model string
			attrsJSON, createdAt      string
			statusCode, latencyMs     int
			success                   int
		)
		if err := rows.Scan(&linkID, &providerID, &model, &statusCode, &latencyMs, &success, &attrsJSON, &createdAt); err != nil {
			rows.Close()
			return err
		}
		t, err := time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			rows.Close()
			return err
		}
		period := t.UTC().Format("2006-01-02T15")
		attrs := models.Map{}
		_ = json.Unmarshal([]byte(attrsJSON), &attrs)

		bump := func(ak, av string) {
			k := key{linkID, providerID, ak, av, period}
			b := buckets[k]
			b.LinkID = linkID
			b.ProviderID = providerID
			b.Period = period
			b.Total++
			if success == 1 {
				b.Success++
			} else {
				b.Failure++
			}
			b.TotalLatMS += int64(latencyMs)
			buckets[k] = b
		}
		bump("", "") // link-level aggregate ignoring attributes
		for ak, av := range attrs {
			bump(ak, toStr(av))
		}
	}
	rows.Close()

	for k, b := range buckets {
		if err := s.DB.UpsertStats(b, k.attrKey, k.attrValue); err != nil {
			return err
		}
	}
	return s.DB.SetMeta("last_aggregated_at", end.Format(time.RFC3339Nano))
}

func toStr(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// Run starts a background aggregation loop until ctx is canceled.
func (s *Service) Run(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := s.Aggregate(ctx); err != nil {
				log.Printf("stats: aggregate: %v", err)
			}
		}
	}
}
