package db

import (
	"database/sql"
	"errors"
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
	Model            string  `json:"model"`
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
	ErrorMessage     *string `json:"error_message"`
	CreatedAt        string  `json:"created_at"`
}

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
	res, err := d.conn.Exec(
		"INSERT INTO sessions (provider_id, model, status, started_at) VALUES (?, ?, 'pending', datetime('now'))",
		providerID, model,
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
	res, err := d.conn.Exec(
		`INSERT INTO requests (session_id, prompt_tokens, completion_tokens, duration_ms, error_message, created_at) VALUES (?, ?, ?, ?, ?, datetime('now'))`,
		sessionID, promptTokens, completionTokens, durationMs, errMsg,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
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
		`SELECT s.id, s.provider_id, p.name, s.model, s.status, s.started_at, s.ended_at,
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
		if err := rows.Scan(&s.ID, &s.ProviderID, &s.ProviderName, &s.Model, &s.Status,
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
		`SELECT s.id, s.provider_id, p.name, s.model, s.status, s.started_at, s.ended_at,
		        s.prompt_tokens, s.completion_tokens, s.duration_ms, s.error_message
		 FROM sessions s JOIN providers p ON s.provider_id = p.id WHERE s.id = ?`, id,
	).Scan(&s.ID, &s.ProviderID, &s.ProviderName, &s.Model, &s.Status,
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
