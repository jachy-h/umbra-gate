package db

import (
	"database/sql"
	"errors"
	"math"
	"sort"
	"strings"
	"time"
)

type Provider struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

type Session struct {
	ID               int64   `json:"id"`
	ProviderID       int64   `json:"provider_id"`
	ProviderName     string  `json:"provider_name"`
	AgentID          string  `json:"agent_id"`
	ProjectID        string  `json:"project_id"`
	Model            string  `json:"model"`
	Endpoint         string  `json:"endpoint"`
	Stream           bool    `json:"stream"`
	Status           string  `json:"status"`
	StartedAt        string  `json:"started_at"`
	EndedAt          *string `json:"ended_at"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	DurationMs       int64   `json:"duration_ms"`
	ErrorMessage     *string `json:"error_message"`
}

type Request struct {
	ID               int64   `json:"id"`
	SessionID        int64   `json:"session_id"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	DurationMs       int64   `json:"duration_ms"`
	ResponseStatus   int64   `json:"response_status"`
	Endpoint         string  `json:"endpoint"`
	ErrorMessage     *string `json:"error_message"`
	CreatedAt        string  `json:"created_at"`
}

type SessionMeta struct {
	AgentID   string
	ProjectID string
	Endpoint  string
	Stream    bool
}

type RequestLog struct {
	ID              int64  `json:"id"`
	SessionID       int64  `json:"session_id"`
	ProviderName    string `json:"provider_name"`
	Method          string `json:"method"`
	URL             string `json:"url"`
	RequestHeaders  string `json:"request_headers"`
	RequestBody     string `json:"request_body"`
	ResponseStatus  int    `json:"response_status"`
	ResponseHeaders string `json:"response_headers"`
	ResponseBody    string `json:"response_body"`
	DurationMs      int64  `json:"duration_ms"`
	CreatedAt       string `json:"created_at"`
}

// MaxRequestLogs caps the number of HTTP request/response logs we retain.
// Older entries are pruned automatically when this limit is exceeded.
const MaxRequestLogs = 10

type Stats struct {
	TodayRequests int64 `json:"today_requests"`
	MonthRequests int64 `json:"month_requests"`
	TodayTokens   int64 `json:"today_tokens"`
	MonthTokens   int64 `json:"month_tokens"`
	TodaySessions int64 `json:"today_sessions"`
	MonthSessions int64 `json:"month_sessions"`
}

type ModelStats struct {
	Model         string  `json:"model"`
	RequestCount  int64   `json:"request_count"`
	TotalTokens   int64   `json:"total_tokens"`
	AvgDurationMs float64 `json:"avg_duration_ms"`
}

type ProviderStats struct {
	ProviderName  string  `json:"provider_name"`
	RequestCount  int64   `json:"request_count"`
	TotalTokens   int64   `json:"total_tokens"`
	AvgDurationMs float64 `json:"avg_duration_ms"`
}

type ProviderAnalytics struct {
	ProviderName  string  `json:"provider_name"`
	RequestCount  int64   `json:"request_count"`
	TotalTokens   int64   `json:"total_tokens"`
	AvgDurationMs float64 `json:"avg_duration_ms"`
	SuccessCount  int64   `json:"success_count"`
	ErrorCount    int64   `json:"error_count"`
	SuccessRate   float64 `json:"success_rate"`
	ErrorRate     float64 `json:"error_rate"`
}

type ModelAnalytics struct {
	Model         string  `json:"model"`
	RequestCount  int64   `json:"request_count"`
	TotalTokens   int64   `json:"total_tokens"`
	AvgDurationMs float64 `json:"avg_duration_ms"`
	SuccessCount  int64   `json:"success_count"`
	ErrorCount    int64   `json:"error_count"`
	SuccessRate   float64 `json:"success_rate"`
	ErrorRate     float64 `json:"error_rate"`
}

type LatencyAnalytics struct {
	Name             string  `json:"name"`
	RequestCount     int64   `json:"request_count"`
	AvgDurationMs    float64 `json:"avg_duration_ms"`
	MedianDurationMs int64   `json:"median_duration_ms"`
	P95DurationMs    int64   `json:"p95_duration_ms"`
	P99DurationMs    int64   `json:"p99_duration_ms"`
}

type OverviewStats struct {
	TotalRequests int64   `json:"total_requests"`
	TotalSessions int64   `json:"total_sessions"`
	TotalTokens   int64   `json:"total_tokens"`
	SuccessCount  int64   `json:"success_count"`
	ErrorCount    int64   `json:"error_count"`
	SuccessRate   float64 `json:"success_rate"`
	AvgDurationMs float64 `json:"avg_duration_ms"`
	P95DurationMs int64   `json:"p95_duration_ms"`
}

type AnalyticsBreakdown struct {
	Name          string  `json:"name"`
	RequestCount  int64   `json:"request_count"`
	TotalTokens   int64   `json:"total_tokens"`
	AvgDurationMs float64 `json:"avg_duration_ms"`
	SuccessCount  int64   `json:"success_count"`
	ErrorCount    int64   `json:"error_count"`
	SuccessRate   float64 `json:"success_rate"`
	ErrorRate     float64 `json:"error_rate"`
}

type FailureAnalytics struct {
	TotalFailures int64              `json:"total_failures"`
	Categories    []FailureCategory  `json:"categories"`
	Agents        []FailureDimension `json:"agents"`
	Providers     []FailureDimension `json:"providers"`
	Models        []FailureDimension `json:"models"`
	Endpoints     []FailureDimension `json:"endpoints"`
	Recent        []Session          `json:"recent"`
}

type FailureCategory struct {
	Category string `json:"category"`
	Count    int64  `json:"count"`
}

type FailureDimension struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

type TimeSeriesStats struct {
	Date         string `json:"date"`
	RequestCount int64  `json:"request_count"`
	TotalTokens  int64  `json:"total_tokens"`
}

type TimeSeriesBreakdown struct {
	Date         string `json:"date"`
	Name         string `json:"name"`
	RequestCount int64  `json:"request_count"`
	TotalTokens  int64  `json:"total_tokens"`
}

type AnalyticsRange struct {
	Value string
	Days  int
}

func ParseAnalyticsRange(raw string) AnalyticsRange {
	switch raw {
	case "24h":
		return AnalyticsRange{Value: "24h", Days: 1}
	case "30d":
		return AnalyticsRange{Value: "30d", Days: 30}
	case "90d":
		return AnalyticsRange{Value: "90d", Days: 90}
	case "7d", "":
		return AnalyticsRange{Value: "7d", Days: 7}
	default:
		return AnalyticsRange{Value: "7d", Days: 7}
	}
}

func (r AnalyticsRange) StartTime(now time.Time) time.Time {
	if r.Value == "24h" {
		return now.Add(-24 * time.Hour)
	}
	start := now.AddDate(0, 0, -(r.Days - 1))
	return time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
}

func (d *DB) EnsureProvider(name string) (int64, error) {
	var id int64
	err := d.conn.QueryRow("SELECT id FROM providers WHERE name = ?", name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	d.conn.Exec("INSERT OR IGNORE INTO providers (name) VALUES (?)", name)
	err = d.conn.QueryRow("SELECT id FROM providers WHERE name = ?", name).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (d *DB) CreateSession(providerID int64, model string) (int64, error) {
	return d.CreateSessionWithMeta(providerID, model, SessionMeta{})
}

func (d *DB) CreateSessionWithMeta(providerID int64, model string, meta SessionMeta) (int64, error) {
	meta = normalizeSessionMeta(meta)
	res, err := d.conn.Exec(
		`INSERT INTO sessions
		 (provider_id, model, agent_id, project_id, endpoint, stream, status, started_at)
		 VALUES (?, ?, ?, ?, ?, ?, 'pending', datetime('now'))`,
		providerID, model, meta.AgentID, meta.ProjectID, meta.Endpoint, boolInt(meta.Stream),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) CompleteSession(sessionID int64, promptTokens, completionTokens int64, durationMs int64, errMsg *string) error {
	status := "success"
	if errMsg != nil {
		status = "error"
	}
	_, err := d.conn.Exec(
		`UPDATE sessions SET status=?, ended_at=datetime('now'), prompt_tokens=?, completion_tokens=?, duration_ms=?, error_message=? WHERE id=?`,
		status, promptTokens, completionTokens, durationMs, errMsg, sessionID,
	)
	return err
}

func (d *DB) CreateRequest(sessionID int64, promptTokens, completionTokens int64, durationMs int64, errMsg *string) (int64, error) {
	return d.CreateRequestWithMeta(sessionID, promptTokens, completionTokens, durationMs, 0, "", errMsg)
}

func (d *DB) CreateRequestWithMeta(sessionID int64, promptTokens, completionTokens int64, durationMs int64, responseStatus int64, endpoint string, errMsg *string) (int64, error) {
	res, err := d.conn.Exec(
		`INSERT INTO requests
		 (session_id, prompt_tokens, completion_tokens, duration_ms, response_status, endpoint, error_message, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now'))`,
		sessionID, promptTokens, completionTokens, durationMs, responseStatus, strings.TrimSpace(endpoint), errMsg,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func normalizeSessionMeta(meta SessionMeta) SessionMeta {
	meta.AgentID = strings.TrimSpace(meta.AgentID)
	if meta.AgentID == "" {
		meta.AgentID = "unknown"
	}
	meta.ProjectID = strings.TrimSpace(meta.ProjectID)
	meta.Endpoint = strings.TrimSpace(meta.Endpoint)
	return meta
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func (d *DB) GetStats() (*Stats, error) {
	s := &Stats{}
	now := time.Now()
	today := now.Format("2006-01-02")
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")

	if err := d.conn.QueryRow("SELECT COUNT(*) FROM sessions WHERE date(started_at) = ?", today).Scan(&s.TodaySessions); err != nil {
		return nil, err
	}
	if err := d.conn.QueryRow("SELECT COUNT(*) FROM sessions WHERE date(started_at) >= ?", monthStart).Scan(&s.MonthSessions); err != nil {
		return nil, err
	}
	if err := d.conn.QueryRow("SELECT COUNT(*) FROM requests WHERE date(created_at) = ?", today).Scan(&s.TodayRequests); err != nil {
		return nil, err
	}
	if err := d.conn.QueryRow("SELECT COUNT(*) FROM requests WHERE date(created_at) >= ?", monthStart).Scan(&s.MonthRequests); err != nil {
		return nil, err
	}
	if err := d.conn.QueryRow("SELECT COALESCE(SUM(prompt_tokens + completion_tokens), 0) FROM requests WHERE date(created_at) = ?", today).Scan(&s.TodayTokens); err != nil {
		return nil, err
	}
	if err := d.conn.QueryRow("SELECT COALESCE(SUM(prompt_tokens + completion_tokens), 0) FROM requests WHERE date(created_at) >= ?", monthStart).Scan(&s.MonthTokens); err != nil {
		return nil, err
	}

	return s, nil
}

func (d *DB) ListSessions(limit, offset int) ([]Session, error) {
	rows, err := d.conn.Query(
		`SELECT s.id, s.provider_id, p.name, s.agent_id, s.project_id, s.model, s.endpoint, s.stream, s.status, s.started_at, s.ended_at,
		        s.prompt_tokens, s.completion_tokens, s.duration_ms, s.error_message
		 FROM sessions s JOIN providers p ON s.provider_id = p.id
		 ORDER BY s.started_at DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.ProviderID, &s.ProviderName, &s.AgentID, &s.ProjectID, &s.Model, &s.Endpoint, &s.Stream, &s.Status,
			&s.StartedAt, &s.EndedAt, &s.PromptTokens, &s.CompletionTokens, &s.DurationMs, &s.ErrorMessage); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return sessions, nil
}

func (d *DB) GetSession(id int64) (*Session, error) {
	s := &Session{}
	err := d.conn.QueryRow(
		`SELECT s.id, s.provider_id, p.name, s.agent_id, s.project_id, s.model, s.endpoint, s.stream, s.status, s.started_at, s.ended_at,
		        s.prompt_tokens, s.completion_tokens, s.duration_ms, s.error_message
		 FROM sessions s JOIN providers p ON s.provider_id = p.id WHERE s.id = ?`, id,
	).Scan(&s.ID, &s.ProviderID, &s.ProviderName, &s.AgentID, &s.ProjectID, &s.Model, &s.Endpoint, &s.Stream, &s.Status,
		&s.StartedAt, &s.EndedAt, &s.PromptTokens, &s.CompletionTokens, &s.DurationMs, &s.ErrorMessage)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (d *DB) GetModelStats() ([]ModelStats, error) {
	rows, err := d.conn.Query(
		`SELECT model, COUNT(*) as cnt, COALESCE(SUM(prompt_tokens + completion_tokens), 0) as total_tokens,
		        COALESCE(AVG(duration_ms), 0) as avg_duration
		 FROM sessions WHERE status = 'success'
		 GROUP BY model ORDER BY cnt DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []ModelStats
	for rows.Next() {
		var s ModelStats
		if err := rows.Scan(&s.Model, &s.RequestCount, &s.TotalTokens, &s.AvgDurationMs); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return stats, nil
}

func (d *DB) GetProviderStats() ([]ProviderStats, error) {
	rows, err := d.conn.Query(
		`SELECT p.name, COUNT(*) as cnt, COALESCE(SUM(s.prompt_tokens + s.completion_tokens), 0) as total_tokens,
		        COALESCE(AVG(s.duration_ms), 0) as avg_duration
		 FROM sessions s JOIN providers p ON s.provider_id = p.id
		 WHERE s.status = 'success'
		 GROUP BY p.name ORDER BY total_tokens DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []ProviderStats
	for rows.Next() {
		var s ProviderStats
		if err := rows.Scan(&s.ProviderName, &s.RequestCount, &s.TotalTokens, &s.AvgDurationMs); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return stats, nil
}

func (d *DB) GetOverviewStats(rawRange string) (*OverviewStats, error) {
	r := ParseAnalyticsRange(rawRange)
	start := r.StartTime(time.Now()).Format("2006-01-02 15:04:05")
	rows, err := d.conn.Query(
		`SELECT status, prompt_tokens, completion_tokens, duration_ms
		 FROM sessions
		 WHERE datetime(started_at) >= datetime(?)`, start,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	overview := &OverviewStats{}
	var successDurations []int64
	var successDurationTotal int64
	for rows.Next() {
		var status string
		var promptTokens, completionTokens, durationMs int64
		if err := rows.Scan(&status, &promptTokens, &completionTokens, &durationMs); err != nil {
			return nil, err
		}
		overview.TotalSessions++
		overview.TotalRequests++
		overview.TotalTokens += promptTokens + completionTokens
		if status == "success" {
			overview.SuccessCount++
			successDurations = append(successDurations, durationMs)
			successDurationTotal += durationMs
		} else if status == "error" {
			overview.ErrorCount++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if overview.TotalSessions > 0 {
		overview.SuccessRate = float64(overview.SuccessCount) / float64(overview.TotalSessions)
	}
	if len(successDurations) > 0 {
		sort.Slice(successDurations, func(i, j int) bool { return successDurations[i] < successDurations[j] })
		overview.AvgDurationMs = float64(successDurationTotal) / float64(len(successDurations))
		overview.P95DurationMs = percentileDuration(successDurations, 0.95)
	}
	return overview, nil
}

func (d *DB) GetAnalyticsBreakdown(rawRange, dimension string) ([]AnalyticsBreakdown, error) {
	r := ParseAnalyticsRange(rawRange)
	start := r.StartTime(time.Now()).Format("2006-01-02 15:04:05")
	selectExpr, joinExpr := analyticsDimensionExpr(dimension)
	rows, err := d.conn.Query(
		`SELECT `+selectExpr+`,
		        COUNT(*) AS cnt,
		        COALESCE(SUM(s.prompt_tokens + s.completion_tokens), 0) AS total_tokens,
		        COALESCE(AVG(s.duration_ms), 0) AS avg_duration,
		        SUM(CASE WHEN s.status = 'success' THEN 1 ELSE 0 END) AS success_count,
		        SUM(CASE WHEN s.status = 'error' THEN 1 ELSE 0 END) AS error_count
		 FROM sessions s `+joinExpr+`
		 WHERE datetime(s.started_at) >= datetime(?)
		 GROUP BY 1
		 ORDER BY cnt DESC, total_tokens DESC`, start,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var breakdown []AnalyticsBreakdown
	for rows.Next() {
		var row AnalyticsBreakdown
		if err := rows.Scan(&row.Name, &row.RequestCount, &row.TotalTokens, &row.AvgDurationMs, &row.SuccessCount, &row.ErrorCount); err != nil {
			return nil, err
		}
		if row.RequestCount > 0 {
			row.SuccessRate = float64(row.SuccessCount) / float64(row.RequestCount)
			row.ErrorRate = float64(row.ErrorCount) / float64(row.RequestCount)
		}
		breakdown = append(breakdown, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return breakdown, nil
}

func analyticsDimensionExpr(dimension string) (string, string) {
	switch strings.ToLower(strings.TrimSpace(dimension)) {
	case "provider":
		return "p.name", "JOIN providers p ON s.provider_id = p.id"
	case "model":
		return "COALESCE(NULLIF(s.model, ''), 'unknown')", ""
	case "project":
		return "COALESCE(NULLIF(s.project_id, ''), 'unknown')", ""
	case "endpoint":
		return "COALESCE(NULLIF(s.endpoint, ''), 'unknown')", ""
	case "status":
		return "COALESCE(NULLIF(s.status, ''), 'unknown')", ""
	case "agent", "":
		return "COALESCE(NULLIF(s.agent_id, ''), 'unknown')", ""
	default:
		return "COALESCE(NULLIF(s.agent_id, ''), 'unknown')", ""
	}
}

func (d *DB) GetFailureAnalytics(rawRange string) (*FailureAnalytics, error) {
	r := ParseAnalyticsRange(rawRange)
	start := r.StartTime(time.Now()).Format("2006-01-02 15:04:05")
	rows, err := d.conn.Query(
		`SELECT s.id, s.provider_id, p.name, s.agent_id, s.project_id, s.model, s.endpoint, s.stream, s.status, s.started_at, s.ended_at,
		        s.prompt_tokens, s.completion_tokens, s.duration_ms, s.error_message
		 FROM sessions s JOIN providers p ON s.provider_id = p.id
		 WHERE s.status = 'error' AND datetime(s.started_at) >= datetime(?)
		 ORDER BY s.started_at DESC`, start,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	analytics := &FailureAnalytics{}
	categories := map[string]int64{}
	agents := map[string]int64{}
	providers := map[string]int64{}
	models := map[string]int64{}
	endpoints := map[string]int64{}
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.ProviderID, &s.ProviderName, &s.AgentID, &s.ProjectID, &s.Model, &s.Endpoint, &s.Stream, &s.Status, &s.StartedAt, &s.EndedAt, &s.PromptTokens, &s.CompletionTokens, &s.DurationMs, &s.ErrorMessage); err != nil {
			return nil, err
		}
		analytics.TotalFailures++
		categories[classifyError(s.ErrorMessage)]++
		agents[firstNonEmpty(s.AgentID, "unknown")]++
		providers[s.ProviderName]++
		models[s.Model]++
		endpoints[firstNonEmpty(s.Endpoint, "unknown")]++
		if len(analytics.Recent) < 20 {
			analytics.Recent = append(analytics.Recent, s)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	analytics.Categories = failureCategories(categories)
	analytics.Agents = failureDimensions(agents)
	analytics.Providers = failureDimensions(providers)
	analytics.Models = failureDimensions(models)
	analytics.Endpoints = failureDimensions(endpoints)
	return analytics, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func classifyError(message *string) string {
	if message == nil {
		return "unknown"
	}
	m := strings.ToLower(*message)
	if strings.Contains(m, "timeout") || strings.Contains(m, "deadline") {
		return "timeout"
	}
	if strings.Contains(m, "connection") || strings.Contains(m, "refused") || strings.Contains(m, "network") || strings.Contains(m, "no such host") {
		return "network_error"
	}
	if strings.Contains(m, "500") || strings.Contains(m, "502") || strings.Contains(m, "503") || strings.Contains(m, "504") || strings.Contains(m, "5xx") {
		return "upstream_5xx"
	}
	if strings.Contains(m, "400") || strings.Contains(m, "401") || strings.Contains(m, "403") || strings.Contains(m, "404") || strings.Contains(m, "429") || strings.Contains(m, "4xx") {
		return "upstream_4xx"
	}
	return "unknown"
}

func failureCategories(counts map[string]int64) []FailureCategory {
	categories := make([]FailureCategory, 0, len(counts))
	for category, count := range counts {
		categories = append(categories, FailureCategory{Category: category, Count: count})
	}
	sort.Slice(categories, func(i, j int) bool {
		if categories[i].Count == categories[j].Count {
			return categories[i].Category < categories[j].Category
		}
		return categories[i].Count > categories[j].Count
	})
	return categories
}

func failureDimensions(counts map[string]int64) []FailureDimension {
	dimensions := make([]FailureDimension, 0, len(counts))
	for name, count := range counts {
		dimensions = append(dimensions, FailureDimension{Name: name, Count: count})
	}
	sort.Slice(dimensions, func(i, j int) bool {
		if dimensions[i].Count == dimensions[j].Count {
			return dimensions[i].Name < dimensions[j].Name
		}
		return dimensions[i].Count > dimensions[j].Count
	})
	return dimensions
}

func (d *DB) GetLatencyAnalytics(rawRange, by string) ([]LatencyAnalytics, error) {
	r := ParseAnalyticsRange(rawRange)
	start := r.StartTime(time.Now()).Format("2006-01-02 15:04:05")
	selectExpr, joinExpr := analyticsDimensionExpr(by)
	rows, err := d.conn.Query(
		`SELECT `+selectExpr+`, s.duration_ms
		 FROM sessions s `+joinExpr+`
		 WHERE s.status = 'success' AND datetime(s.started_at) >= datetime(?)
		 ORDER BY s.duration_ms ASC`, start,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	grouped := map[string][]int64{}
	for rows.Next() {
		var name string
		var duration int64
		if err := rows.Scan(&name, &duration); err != nil {
			return nil, err
		}
		grouped[name] = append(grouped[name], duration)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	analytics := make([]LatencyAnalytics, 0, len(grouped))
	for name, durations := range grouped {
		sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
		var total int64
		for _, duration := range durations {
			total += duration
		}
		analytics = append(analytics, LatencyAnalytics{
			Name:             name,
			RequestCount:     int64(len(durations)),
			AvgDurationMs:    float64(total) / float64(len(durations)),
			MedianDurationMs: percentileDuration(durations, 0.5),
			P95DurationMs:    percentileDuration(durations, 0.95),
			P99DurationMs:    percentileDuration(durations, 0.99),
		})
	}
	sort.Slice(analytics, func(i, j int) bool {
		if analytics[i].P95DurationMs == analytics[j].P95DurationMs {
			return analytics[i].Name < analytics[j].Name
		}
		return analytics[i].P95DurationMs > analytics[j].P95DurationMs
	})
	return analytics, nil
}

func percentileDuration(values []int64, p float64) int64 {
	if len(values) == 0 {
		return 0
	}
	idx := int(math.Ceil(p*float64(len(values)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(values) {
		idx = len(values) - 1
	}
	return values[idx]
}

func (d *DB) GetProviderAnalytics(rawRange string) ([]ProviderAnalytics, error) {
	r := ParseAnalyticsRange(rawRange)
	start := r.StartTime(time.Now()).Format("2006-01-02 15:04:05")
	rows, err := d.conn.Query(
		`SELECT p.name, COUNT(*) as cnt,
		        COALESCE(SUM(s.prompt_tokens + s.completion_tokens), 0) as total_tokens,
		        COALESCE(AVG(s.duration_ms), 0) as avg_duration,
		        SUM(CASE WHEN s.status = 'success' THEN 1 ELSE 0 END) as success_count,
		        SUM(CASE WHEN s.status = 'error' THEN 1 ELSE 0 END) as error_count
		 FROM sessions s JOIN providers p ON s.provider_id = p.id
		 WHERE datetime(s.started_at) >= datetime(?)
		 GROUP BY p.name ORDER BY total_tokens DESC, cnt DESC`, start,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var analytics []ProviderAnalytics
	for rows.Next() {
		var s ProviderAnalytics
		if err := rows.Scan(&s.ProviderName, &s.RequestCount, &s.TotalTokens, &s.AvgDurationMs, &s.SuccessCount, &s.ErrorCount); err != nil {
			return nil, err
		}
		if s.RequestCount > 0 {
			s.SuccessRate = float64(s.SuccessCount) / float64(s.RequestCount)
			s.ErrorRate = float64(s.ErrorCount) / float64(s.RequestCount)
		}
		analytics = append(analytics, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return analytics, nil
}

func (d *DB) GetModelAnalytics(rawRange string) ([]ModelAnalytics, error) {
	r := ParseAnalyticsRange(rawRange)
	start := r.StartTime(time.Now()).Format("2006-01-02 15:04:05")
	rows, err := d.conn.Query(
		`SELECT COALESCE(NULLIF(s.model, ''), 'unknown') as model, COUNT(*) as cnt,
		        COALESCE(SUM(s.prompt_tokens + s.completion_tokens), 0) as total_tokens,
		        COALESCE(AVG(s.duration_ms), 0) as avg_duration,
		        COALESCE(SUM(CASE WHEN s.status = 'success' THEN 1 ELSE 0 END), 0) as success_count,
		        COALESCE(SUM(CASE WHEN s.status = 'error' THEN 1 ELSE 0 END), 0) as error_count
		 FROM sessions s
		 WHERE datetime(s.started_at) >= datetime(?)
		 GROUP BY COALESCE(NULLIF(s.model, ''), 'unknown') ORDER BY total_tokens DESC, cnt DESC`, start,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var analytics []ModelAnalytics
	for rows.Next() {
		var s ModelAnalytics
		if err := rows.Scan(&s.Model, &s.RequestCount, &s.TotalTokens, &s.AvgDurationMs, &s.SuccessCount, &s.ErrorCount); err != nil {
			return nil, err
		}
		if s.RequestCount > 0 {
			s.SuccessRate = float64(s.SuccessCount) / float64(s.RequestCount)
			s.ErrorRate = float64(s.ErrorCount) / float64(s.RequestCount)
		}
		analytics = append(analytics, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return analytics, nil
}

func (d *DB) GetTimeSeriesStats(days int) ([]TimeSeriesStats, error) {
	if days <= 0 {
		days = 7
	}
	if days > 90 {
		days = 90
	}
	return d.getTimeSeriesStats(AnalyticsRange{Value: "custom", Days: days})
}

func (d *DB) GetTimeSeriesStatsForRange(rawRange string) ([]TimeSeriesStats, error) {
	return d.getTimeSeriesStats(ParseAnalyticsRange(rawRange))
}

func (d *DB) GetTimeSeriesBreakdown(rawRange, dimension string) ([]TimeSeriesBreakdown, error) {
	r := ParseAnalyticsRange(rawRange)
	start := r.StartTime(time.Now()).Format("2006-01-02 15:04:05")
	selectExpr, joinExpr := analyticsDimensionExpr(dimension)
	rows, err := d.conn.Query(
		`SELECT date(s.started_at), `+selectExpr+`, COUNT(*) AS cnt,
		        COALESCE(SUM(s.prompt_tokens + s.completion_tokens), 0) AS total_tokens
		 FROM sessions s `+joinExpr+`
		 WHERE datetime(s.started_at) >= datetime(?)
		 GROUP BY date(s.started_at), `+selectExpr+`
		 ORDER BY date(s.started_at) ASC, cnt DESC`, start,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []TimeSeriesBreakdown
	for rows.Next() {
		var stat TimeSeriesBreakdown
		if err := rows.Scan(&stat.Date, &stat.Name, &stat.RequestCount, &stat.TotalTokens); err != nil {
			return nil, err
		}
		stats = append(stats, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return stats, nil
}

func (d *DB) getTimeSeriesStats(r AnalyticsRange) ([]TimeSeriesStats, error) {
	today := time.Now()
	start := today.AddDate(0, 0, -(r.Days - 1))
	byDate := make(map[string]TimeSeriesStats, r.Days)
	stats := make([]TimeSeriesStats, 0, r.Days)
	for i := 0; i < r.Days; i++ {
		date := start.AddDate(0, 0, i).Format("2006-01-02")
		stat := TimeSeriesStats{Date: date}
		byDate[date] = stat
		stats = append(stats, stat)
	}

	rows, err := d.conn.Query(
		`SELECT date(created_at), COUNT(*) as cnt, COALESCE(SUM(prompt_tokens + completion_tokens), 0) as total_tokens
		 FROM requests
		 WHERE date(created_at) >= ?
		 GROUP BY date(created_at)
		 ORDER BY date(created_at) ASC`, start.Format("2006-01-02"),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var stat TimeSeriesStats
		if err := rows.Scan(&stat.Date, &stat.RequestCount, &stat.TotalTokens); err != nil {
			return nil, err
		}
		byDate[stat.Date] = stat
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range stats {
		stats[i] = byDate[stats[i].Date]
	}
	return stats, nil
}

// InsertRequestLog stores a captured HTTP request/response pair and prunes the
// table to retain only the most recent MaxRequestLogs rows.
func (d *DB) InsertRequestLog(log RequestLog) (int64, error) {
	res, err := d.conn.Exec(
		`INSERT INTO request_logs
		 (session_id, provider_name, method, url, request_headers, request_body,
		  response_status, response_headers, response_body, duration_ms, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))`,
		log.SessionID, log.ProviderName, log.Method, log.URL,
		log.RequestHeaders, log.RequestBody,
		log.ResponseStatus, log.ResponseHeaders, log.ResponseBody,
		log.DurationMs,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if _, err := d.conn.Exec(
		`DELETE FROM request_logs WHERE id NOT IN (
			SELECT id FROM request_logs ORDER BY id DESC LIMIT ?
		)`, MaxRequestLogs,
	); err != nil {
		return id, err
	}
	return id, nil
}

func (d *DB) GetRequestLogBySession(sessionID int64) (*RequestLog, error) {
	log := &RequestLog{}
	err := d.conn.QueryRow(
		`SELECT id, session_id, provider_name, method, url,
		        request_headers, request_body,
		        response_status, response_headers, response_body,
		        duration_ms, created_at
		 FROM request_logs WHERE session_id = ?
		 ORDER BY id DESC LIMIT 1`, sessionID,
	).Scan(&log.ID, &log.SessionID, &log.ProviderName, &log.Method, &log.URL,
		&log.RequestHeaders, &log.RequestBody,
		&log.ResponseStatus, &log.ResponseHeaders, &log.ResponseBody,
		&log.DurationMs, &log.CreatedAt)
	if err != nil {
		return nil, err
	}
	return log, nil
}

func (d *DB) ListRecentRequestLogs() ([]RequestLog, error) {
	rows, err := d.conn.Query(
		`SELECT id, session_id, provider_name, method, url,
		        request_headers, request_body,
		        response_status, response_headers, response_body,
		        duration_ms, created_at
		 FROM request_logs ORDER BY id DESC LIMIT ?`, MaxRequestLogs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []RequestLog
	for rows.Next() {
		var l RequestLog
		if err := rows.Scan(&l.ID, &l.SessionID, &l.ProviderName, &l.Method, &l.URL,
			&l.RequestHeaders, &l.RequestBody,
			&l.ResponseStatus, &l.ResponseHeaders, &l.ResponseBody,
			&l.DurationMs, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}
