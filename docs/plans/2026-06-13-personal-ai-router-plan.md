# Personal AI Router — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a local-first LLM Gateway that proxies requests to OpenAI/Anthropic-compatible providers and records usage statistics in SQLite, with a dashboard.

**Architecture:** Single Go binary using `net/http` stdlib. Path-prefix routing to providers. SQLite via `modernc.org/sqlite`. Go `html/template` + `embed` for dashboard. SSE streaming via `http.Flusher`.

**Tech Stack:** Go 1.22+, `gopkg.in/yaml.v3`, `modernc.org/sqlite`, `net/http`, `html/template`, `log/slog`

**Design doc:** `docs/plans/2026-06-13-umbragate-design.md`

---

## Phase 1: Core Proxy + Stats

### Task 1: Initialize Go module and project structure

**Files:**
- Create: `go.mod`
- Create: `main.go` (skeleton)
- Create: `config.yaml` (default config)
- Create: `config/config.go`
- Create: `db/db.go`
- Create: `db/queries.go`
- Create: `proxy/proxy.go`
- Create: `proxy/openai.go`
- Create: `dashboard/handler.go`
- Create: `api/handler.go`

**Step 1: Initialize Go module**

Run: `go mod init github.com/jachy-h/umbra-gate`
Expected: Creates `go.mod`

**Step 2: Create default config.yaml**

```yaml
listen: "127.0.0.1:4141"

providers:
  volcengine:
    base_url: https://ark.cn-beijing.volces.com/api/coding/v3
    api_key: ""
    protocol: openai-compatible

storage:
  save_prompt: false
```

**Step 3: Create config/config.go skeleton**

```go
package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type ProviderConfig struct {
	BaseURL  string `yaml:"base_url"`
	APIKey   string `yaml:"api_key"`
	Protocol string `yaml:"protocol"`
}

type StorageConfig struct {
	SavePrompt bool `yaml:"save_prompt"`
}

type Config struct {
	Listen    string                    `yaml:"listen"`
	Providers map[string]ProviderConfig `yaml:"providers"`
	Storage   StorageConfig             `yaml:"storage"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Listen == "" {
		cfg.Listen = "127.0.0.1:4141"
	}
	return &cfg, nil
}
```

**Step 4: Create db/db.go skeleton**

```go
package db

import (
	"database/sql"
	_ "modernc.org/sqlite"
)

type DB struct {
	conn *sql.DB
}

func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := conn.Ping(); err != nil {
		return nil, err
	}
	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, err
	}
	return db, nil
}

func (d *DB) Close() error {
	return d.conn.Close()
}
```

**Step 5: Create empty skeleton files for remaining packages**

Create `db/queries.go`, `proxy/proxy.go`, `proxy/openai.go`, `dashboard/handler.go`, `api/handler.go` with just `package` declarations.

**Step 6: Create main.go skeleton**

```go
package main

import (
	"log/slog"
	"os"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))
	slog.Info("starting umbragate")

	// TODO: load config, init db, start server
}
```

**Step 7: Add dependencies**

Run: `go get gopkg.in/yaml.v3 modernc.org/sqlite`
Expected: Downloads dependencies, updates go.mod and go.sum

**Step 8: Verify it compiles**

Run: `go build ./...`
Expected: No errors

**Step 9: Commit**

```bash
git add -A && git commit -m "chore: initialize Go module and project structure"
```

---

### Task 2: Database migrations

**Files:**
- Modify: `db/db.go`

**Step 1: Add migration logic to db/db.go**

Add the `migrate` method and schema:

```go
func (d *DB) migrate() error {
	_, err := d.conn.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`)
	if err != nil {
		return err
	}

	var currentVersion int
	err = d.conn.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&currentVersion)
	if err != nil {
		return err
	}

	migrations := []struct {
		version int
		sql     string
	}{
		{1, `CREATE TABLE IF NOT EXISTS providers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`},
		{2, `CREATE TABLE IF NOT EXISTS sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider_id INTEGER NOT NULL REFERENCES providers(id),
			model TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			started_at TEXT NOT NULL DEFAULT (datetime('now')),
			ended_at TEXT,
			prompt_tokens INTEGER DEFAULT 0,
			completion_tokens INTEGER DEFAULT 0,
			duration_ms INTEGER DEFAULT 0,
			error_message TEXT
		)`},
		{3, `CREATE TABLE IF NOT EXISTS requests (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id INTEGER NOT NULL REFERENCES sessions(id),
			prompt_tokens INTEGER DEFAULT 0,
			completion_tokens INTEGER DEFAULT 0,
			duration_ms INTEGER DEFAULT 0,
			error_message TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`},
	}

	for _, m := range migrations {
		if m.version > currentVersion {
			_, err = d.conn.Exec(m.sql)
			if err != nil {
				return err
			}
			_, err = d.conn.Exec("INSERT INTO schema_migrations (version) VALUES (?)", m.version)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
```

**Step 2: Verify it compiles**

Run: `go build ./...`
Expected: No errors

**Step 3: Commit**

```bash
git add db/db.go && git commit -m "feat: add database migrations for providers, sessions, requests"
```

---

### Task 3: Provider CRUD and session/request queries

**Files:**
- Modify: `db/queries.go`

**Step 1: Implement queries**

```go
package db

import "time"

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
	TodayRequests      int64 `json:"today_requests"`
	MonthRequests      int64 `json:"month_requests"`
	TodayTokens        int64 `json:"today_tokens"`
	MonthTokens        int64 `json:"month_tokens"`
	TodaySessions      int64 `json:"today_sessions"`
	MonthSessions      int64 `json:"month_sessions"`
}

type ModelStats struct {
	Model            string  `json:"model"`
	RequestCount     int64   `json:"request_count"`
	TotalTokens      int64   `json:"total_tokens"`
	AvgDurationMs    float64 `json:"avg_duration_ms"`
}

func (d *DB) EnsureProvider(name string) (int64, error) {
	var id int64
	err := d.conn.QueryRow("SELECT id FROM providers WHERE name = ?", name).Scan(&id)
	if err == nil {
		return id, nil
	}
	res, err := d.conn.Exec("INSERT INTO providers (name) VALUES (?)", name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
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

	d.conn.QueryRow("SELECT COUNT(*) FROM sessions WHERE date(started_at) = ?", today).Scan(&s.TodaySessions)
	d.conn.QueryRow("SELECT COUNT(*) FROM sessions WHERE date(started_at) >= ?", monthStart).Scan(&s.MonthSessions)
	d.conn.QueryRow("SELECT COUNT(*) FROM requests WHERE date(created_at) = ?", today).Scan(&s.TodayRequests)
	d.conn.QueryRow("SELECT COUNT(*) FROM requests WHERE date(created_at) >= ?", monthStart).Scan(&s.MonthRequests)
	d.conn.QueryRow("SELECT COALESCE(SUM(prompt_tokens + completion_tokens), 0) FROM requests WHERE date(created_at) = ?", today).Scan(&s.TodayTokens)
	d.conn.QueryRow("SELECT COALESCE(SUM(prompt_tokens + completion_tokens), 0) FROM requests WHERE date(created_at) >= ?", monthStart).Scan(&s.MonthTokens)

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
		rows.Scan(&s.ID, &s.ProviderID, &s.ProviderName, &s.Model, &s.Status,
			&s.StartedAt, &s.EndedAt, &s.PromptTokens, &s.CompletionTokens, &s.DurationMs, &s.ErrorMessage)
		sessions = append(sessions, s)
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
		rows.Scan(&s.Model, &s.RequestCount, &s.TotalTokens, &s.AvgDurationMs)
		stats = append(stats, s)
	}
	return stats, nil
}
```

**Step 2: Verify it compiles**

Run: `go build ./...`
Expected: No errors

**Step 3: Commit**

```bash
git add db/queries.go && git commit -m "feat: add database queries for providers, sessions, requests, stats"
```

---

### Task 4: OpenAI-compatible proxy (non-streaming)

**Files:**
- Modify: `proxy/proxy.go`
- Modify: `proxy/openai.go`

**Step 1: Implement proxy core in proxy/proxy.go**

```go
package proxy

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/jachy-h/umbra-gate/config"
	"github.com/jachy-h/umbra-gate/db"
)

type Proxy struct {
	cfg *config.Config
	db  *db.DB
}

func New(cfg *config.Config, database *db.DB) *Proxy {
	return &Proxy{cfg: cfg, db: database}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}

	providerName := parts[0]
	providerCfg, ok := p.cfg.Providers[providerName]
	if !ok {
		slog.Warn("unknown provider", "provider", providerName)
		http.Error(w, "unknown provider", http.StatusNotFound)
		return
	}

	remainingPath := "/"
	if len(parts) > 1 {
		remainingPath = "/" + parts[1]
	}

	target, err := url.Parse(providerCfg.BaseURL)
	if err != nil {
		slog.Error("invalid provider base_url", "provider", providerName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	switch providerCfg.Protocol {
	case "openai-compatible":
		p.handleOpenAI(w, r, providerName, &providerCfg, target, remainingPath)
	case "anthropic":
		p.handleAnthropic(w, r, providerName, &providerCfg, target, remainingPath)
	default:
		slog.Warn("unknown protocol", "protocol", providerCfg.Protocol)
		http.Error(w, "unknown protocol", http.StatusInternalServerError)
	}
}

func (p *Proxy) singleHostReverseProxy(target *url.URL) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(target)
	origDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		origDirector(req)
		req.Host = target.Host
	}
	return proxy
}
```

**Step 2: Implement OpenAI handler in proxy/openai.go**

```go
package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/jachy-h/umbra-gate/config"
)

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openAIResponse struct {
	Model  string       `json:"model"`
	Usage  openAIUsage  `json:"usage"`
}

type openAIStreamChunk struct {
	Model string       `json:"model"`
	Usage *openAIUsage `json:"usage"`
}

func (p *Proxy) handleOpenAI(w http.ResponseWriter, r *http.Request, providerName string, providerCfg *config.ProviderConfig, target *url.URL, path string) {
	startTime := time.Now()

	bodyBytes, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		slog.Error("failed to read request body", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	model := extractModel(bodyBytes)
	slog.Info("request", "provider", providerName, "model", model, "path", path)

	providerID, err := p.db.EnsureProvider(providerName)
	if err != nil {
		slog.Error("failed to ensure provider", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	sessionID, err := p.db.CreateSession(providerID, model)
	if err != nil {
		slog.Error("failed to create session", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	isStreaming := isStreamRequest(bodyBytes)

	if isStreaming {
		p.proxyOpenAIStream(w, r, providerCfg, target, path, bodyBytes, sessionID, startTime)
	} else {
		p.proxyOpenAINonStream(w, r, providerCfg, target, path, bodyBytes, sessionID, startTime)
	}
}

func (p *Proxy) proxyOpenAINonStream(w http.ResponseWriter, r *http.Request, providerCfg *config.ProviderConfig, target *url.URL, path string, bodyBytes []byte, sessionID int64, startTime time.Time) {
	upstreamURL := target.String() + path
	req, err := http.NewRequestWithContext(r.Context(), "POST", upstreamURL, bytes.NewReader(bodyBytes))
	if err != nil {
		slog.Error("failed to create upstream request", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+providerCfg.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		errMsg := err.Error()
		p.db.CompleteSession(sessionID, 0, 0, time.Since(startTime).Milliseconds(), &errMsg)
		slog.Error("upstream request failed", "error", err)
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		errMsg := err.Error()
		p.db.CompleteSession(sessionID, 0, 0, time.Since(startTime).Milliseconds(), &errMsg)
		slog.Error("failed to read upstream response", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var oaiResp openAIResponse
	json.Unmarshal(respBody, &oaiResp)

	durationMs := time.Since(startTime).Milliseconds()
	promptTokens := int64(oaiResp.Usage.PromptTokens)
	completionTokens := int64(oaiResp.Usage.CompletionTokens)

	p.db.CompleteSession(sessionID, promptTokens, completionTokens, durationMs, nil)
	p.db.CreateRequest(sessionID, promptTokens, completionTokens, durationMs, nil)

	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
}

func extractModel(body []byte) string {
	var req struct {
		Model string `json:"model"`
	}
	json.Unmarshal(body, &req)
	if req.Model == "" {
		return "unknown"
	}
	return req.Model
}

func isStreamRequest(body []byte) bool {
	var req struct {
		Stream bool `json:"stream"`
	}
	json.Unmarshal(body, &req)
	return req.Stream
}
```

**Step 3: Verify it compiles**

Run: `go build ./...`
Expected: No errors

**Step 4: Commit**

```bash
git add proxy/proxy.go proxy/openai.go && git commit -m "feat: add OpenAI-compatible non-streaming proxy"
```

---

### Task 5: Wire up main.go — config loading, DB init, server start

**Files:**
- Modify: `main.go`

**Step 1: Implement main.go**

```go
package main

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/jachy-h/umbra-gate/api"
	"github.com/jachy-h/umbra-gate/config"
	"github.com/jachy-h/umbra-gate/dashboard"
	"github.com/jachy-h/umbra-gate/db"
	"github.com/jachy-h/umbra-gate/proxy"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	execDir, err := os.Getwd()
	if err != nil {
		slog.Error("failed to get working directory", "error", err)
		os.Exit(1)
	}

	configPath := filepath.Join(execDir, "config.yaml")
	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("failed to load config", "path", configPath, "error", err)
		os.Exit(1)
	}

	dbPath := filepath.Join(execDir, "router.db")
	database, err := db.Open(dbPath)
	if err != nil {
		slog.Error("failed to open database", "path", dbPath, "error", err)
		os.Exit(1)
	}
	defer database.Close()

	proxyHandler := proxy.New(cfg, database)
	apiHandler := api.New(database)
	dashHandler := dashboard.New(database)

	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", apiHandler))
	mux.Handle("/dashboard", dashHandler)
	mux.Handle("/dashboard/", dashHandler)
	mux.Handle("/", proxyHandler)

	slog.Info("starting server", "listen", cfg.Listen)
	if err := http.ListenAndServe(cfg.Listen, mux); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
```

**Step 2: Verify it compiles**

Run: `go build ./...`
Expected: Error — api.New, dashboard.New not defined yet. This is expected.

**Step 3: Commit**

```bash
git add main.go && git commit -m "feat: wire up main.go with config, db, server"
```

---

### Task 6: API handlers

**Files:**
- Modify: `api/handler.go`

**Step 1: Implement API handlers**

```go
package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/jachy-h/umbra-gate/db"
)

type Handler struct {
	db *db.DB
}

func New(database *db.DB) *Handler {
	return &Handler{db: database}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.URL.Path {
	case "/stats":
		h.handleStats(w, r)
	case "/sessions":
		h.handleSessions(w, r)
	case "/models":
		h.handleModels(w, r)
	default:
		if len(r.URL.Path) > 10 && r.URL.Path[:10] == "/sessions/" {
			h.handleSessionDetail(w, r)
			return
		}
		http.NotFound(w, r)
	}
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.db.GetStats()
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(stats)
}

func (h *Handler) handleSessions(w http.ResponseWriter, r *http.Request) {
	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		offset, _ = strconv.Atoi(o)
	}

	sessions, err := h.db.ListSessions(limit, offset)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	if sessions == nil {
		sessions = []db.Session{}
	}
	json.NewEncoder(w).Encode(sessions)
}

func (h *Handler) handleSessionDetail(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[10:]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}

	session, err := h.db.GetSession(id)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(session)
}

func (h *Handler) handleModels(w http.ResponseWriter, r *http.Request) {
	stats, err := h.db.GetModelStats()
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	if stats == nil {
		stats = []db.ModelStats{}
	}
	json.NewEncoder(w).Encode(stats)
}
```

**Step 2: Verify it compiles**

Run: `go build ./...`
Expected: Error — dashboard.New not defined yet. API package should compile.

**Step 3: Commit**

```bash
git add api/handler.go && git commit -m "feat: add API handlers for stats, sessions, models"
```

---

### Task 7: Dashboard HTML templates

**Files:**
- Create: `dashboard/templates/layout.html`
- Create: `dashboard/templates/home.html`
- Create: `dashboard/templates/sessions.html`
- Create: `dashboard/templates/session_detail.html`
- Create: `dashboard/templates/models.html`
- Modify: `dashboard/handler.go`

**Step 1: Create layout.html**

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Personal AI Router</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #f5f5f5; color: #333; }
        nav { background: #1a1a2e; color: white; padding: 0 24px; display: flex; align-items: center; height: 56px; }
        nav a { color: #a0a0c0; text-decoration: none; margin-right: 24px; font-size: 14px; }
        nav a:hover, nav a.active { color: white; }
        nav .brand { font-weight: 700; font-size: 16px; color: white; margin-right: 32px; }
        main { max-width: 960px; margin: 32px auto; padding: 0 24px; }
        .card { background: white; border-radius: 8px; padding: 24px; margin-bottom: 16px; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
        .card h2 { font-size: 18px; margin-bottom: 16px; color: #1a1a2e; }
        .stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 16px; }
        .stat { background: white; border-radius: 8px; padding: 20px; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
        .stat .label { font-size: 12px; color: #888; text-transform: uppercase; letter-spacing: 0.5px; }
        .stat .value { font-size: 28px; font-weight: 700; color: #1a1a2e; margin-top: 4px; }
        table { width: 100%; border-collapse: collapse; }
        th, td { text-align: left; padding: 12px 16px; border-bottom: 1px solid #eee; font-size: 14px; }
        th { color: #888; font-weight: 600; font-size: 12px; text-transform: uppercase; }
        tr:hover { background: #fafafa; }
        .badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 12px; }
        .badge-success { background: #e6f9ed; color: #0d7d44; }
        .badge-error { background: #fde8e8; color: #c53030; }
        .badge-pending { background: #fef3c7; color: #92400e; }
        .chart-container { width: 100%; height: 300px; }
        .back-link { display: inline-block; margin-bottom: 16px; color: #555; text-decoration: none; font-size: 14px; }
        .back-link:hover { color: #1a1a2e; }
        .detail-grid { display: grid; grid-template-columns: 150px 1fr; gap: 8px 16px; }
        .detail-grid .key { color: #888; font-size: 14px; }
        .detail-grid .val { font-size: 14px; font-weight: 500; }
    </style>
</head>
<body>
    <nav>
        <span class="brand">AI Router</span>
        <a href="/dashboard" class="{{if eq .Active "home"}}active{{end}}">Home</a>
        <a href="/dashboard/sessions" class="{{if eq .Active "sessions"}}active{{end}}">Sessions</a>
        <a href="/dashboard/models" class="{{if eq .Active "models"}}active{{end}}">Models</a>
    </nav>
    <main>
        {{block "content" .}}{{end}}
    </main>
</body>
</html>
```

**Step 2: Create home.html**

```html
{{define "content"}}
<h1 style="margin-bottom:24px;font-size:24px;">Dashboard</h1>

<div class="stats">
    <div class="stat">
        <div class="label">Today Requests</div>
        <div class="value">{{.Stats.TodayRequests}}</div>
    </div>
    <div class="stat">
        <div class="label">Month Requests</div>
        <div class="value">{{.Stats.MonthRequests}}</div>
    </div>
    <div class="stat">
        <div class="label">Today Tokens</div>
        <div class="value">{{formatNum .Stats.TodayTokens}}</div>
    </div>
    <div class="stat">
        <div class="label">Month Tokens</div>
        <div class="value">{{formatNum .Stats.MonthTokens}}</div>
    </div>
</div>

<div class="card" style="margin-top:24px;">
    <h2>Top Models</h2>
    <div class="chart-container">
        <canvas id="modelChart"></canvas>
    </div>
</div>

<script>
fetch('/api/models')
    .then(r => r.json())
    .then(data => {
        const ctx = document.getElementById('modelChart').getContext('2d');
        const labels = data.slice(0, 10).map(d => d.model);
        const values = data.slice(0, 10).map(d => d.total_tokens);
        drawBarChart(ctx, labels, values, 'Tokens by Model');
    });

function drawBarChart(ctx, labels, values, title) {
    const w = ctx.canvas.width = ctx.canvas.parentElement.clientWidth;
    const h = ctx.canvas.height = 280;
    const pad = { top: 20, right: 20, bottom: 60, left: 80 };
    const chartW = w - pad.left - pad.right;
    const chartH = h - pad.top - pad.bottom;
    const maxVal = Math.max(...values, 1);

    ctx.fillStyle = '#f5f5f5';
    ctx.fillRect(0, 0, w, h);

    const barW = Math.min(40, chartW / labels.length * 0.7);
    const gap = chartW / labels.length;

    values.forEach((v, i) => {
        const barH = (v / maxVal) * chartH;
        const x = pad.left + i * gap + (gap - barW) / 2;
        const y = pad.top + chartH - barH;
        ctx.fillStyle = '#4f46e5';
        ctx.fillRect(x, y, barW, barH);
    });

    ctx.fillStyle = '#888';
    ctx.font = '11px sans-serif';
    ctx.textAlign = 'center';
    labels.forEach((l, i) => {
        const x = pad.left + i * gap + gap / 2;
        ctx.fillText(l.length > 15 ? l.slice(0,15)+'...' : l, x, h - 10);
    });
}
</script>
{{end}}
```

**Step 3: Create sessions.html**

```html
{{define "content"}}
<h1 style="margin-bottom:24px;font-size:24px;">Sessions</h1>

<div class="card">
    <table>
        <thead>
            <tr>
                <th>Time</th>
                <th>Provider</th>
                <th>Model</th>
                <th>Tokens</th>
                <th>Duration</th>
                <th>Status</th>
                <th></th>
            </tr>
        </thead>
        <tbody id="sessionsBody">
            <tr><td colspan="7">Loading...</td></tr>
        </tbody>
    </table>
</div>

<script>
fetch('/api/sessions')
    .then(r => r.json())
    .then(data => {
        const tbody = document.getElementById('sessionsBody');
        tbody.innerHTML = data.map(s => `
            <tr>
                <td>${fmtTime(s.started_at)}</td>
                <td>${s.provider_name}</td>
                <td>${s.model}</td>
                <td>${s.prompt_tokens + s.completion_tokens}</td>
                <td>${fmtDuration(s.duration_ms)}</td>
                <td><span class="badge badge-${s.status}">${s.status}</span></td>
                <td><a href="/dashboard/sessions/${s.id}">View</a></td>
            </tr>
        `).join('');
    });

function fmtTime(t) { return new Date(t + 'Z').toLocaleString(); }
function fmtDuration(ms) { return ms >= 1000 ? (ms/1000).toFixed(1)+'s' : ms+'ms'; }
</script>
{{end}}
```

**Step 4: Create session_detail.html**

```html
{{define "content"}}
<a href="/dashboard/sessions" class="back-link">← Back to Sessions</a>
<h1 style="margin-bottom:24px;font-size:24px;">Session #<span id="sessionId"></span></h1>

<div class="card">
    <div class="detail-grid" id="detailGrid"></div>
</div>

<script>
const id = window.location.pathname.split('/').pop();
document.getElementById('sessionId').textContent = id;

fetch('/api/sessions/' + id)
    .then(r => r.json())
    .then(s => {
        document.getElementById('detailGrid').innerHTML = `
            <div class="key">Provider</div><div class="val">${s.provider_name}</div>
            <div class="key">Model</div><div class="val">${s.model}</div>
            <div class="key">Status</div><div class="val"><span class="badge badge-${s.status}">${s.status}</span></div>
            <div class="key">Started</div><div class="val">${new Date(s.started_at + 'Z').toLocaleString()}</div>
            <div class="key">Ended</div><div class="val">${s.ended_at ? new Date(s.ended_at + 'Z').toLocaleString() : '-'}</div>
            <div class="key">Prompt Tokens</div><div class="val">${s.prompt_tokens}</div>
            <div class="key">Completion Tokens</div><div class="val">${s.completion_tokens}</div>
            <div class="key">Total Tokens</div><div class="val">${s.prompt_tokens + s.completion_tokens}</div>
            <div class="key">Duration</div><div class="val">${s.duration_ms >= 1000 ? (s.duration_ms/1000).toFixed(1)+'s' : s.duration_ms+'ms'}</div>
            ${s.error_message ? '<div class="key">Error</div><div class="val" style="color:#c53030;">'+s.error_message+'</div>' : ''}
        `;
    });
</script>
{{end}}
```

**Step 5: Create models.html**

```html
{{define "content"}}
<h1 style="margin-bottom:24px;font-size:24px;">Models</h1>

<div class="card">
    <table>
        <thead>
            <tr>
                <th>Model</th>
                <th>Requests</th>
                <th>Total Tokens</th>
                <th>Avg Duration</th>
            </tr>
        </thead>
        <tbody id="modelsBody">
            <tr><td colspan="4">Loading...</td></tr>
        </tbody>
    </table>
</div>

<script>
fetch('/api/models')
    .then(r => r.json())
    .then(data => {
        document.getElementById('modelsBody').innerHTML = data.map(m => `
            <tr>
                <td>${m.model}</td>
                <td>${m.request_count}</td>
                <td>${m.total_tokens}</td>
                <td>${(m.avg_duration_ms/1000).toFixed(1)}s</td>
            </tr>
        `).join('');
    });
</script>
{{end}}
```

**Step 6: Implement dashboard/handler.go**

```go
package dashboard

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/jachy-h/umbra-gate/db"
)

//go:embed templates/*
var templateFS embed.FS

type Handler struct {
	db   *db.DB
	tmpl *template.Template
}

func New(database *db.DB) *Handler {
	funcMap := template.FuncMap{
		"formatNum": func(n int64) string {
			if n >= 1000000 {
				return fmt.Sprintf("%.1fM", float64(n)/1000000)
			}
			if n >= 1000 {
				return fmt.Sprintf("%.1fK", float64(n)/1000)
			}
			return fmt.Sprintf("%d", n)
		},
	}

	tmpl := template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.html"))
	return &Handler{db: database, tmpl: tmpl}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/dashboard")
	path = strings.TrimPrefix(path, "/")

	if path == "" || path == "/" {
		h.home(w, r)
		return
	}

	if path == "sessions" {
		h.sessions(w, r)
		return
	}

	if strings.HasPrefix(path, "sessions/") {
		h.sessionDetail(w, r)
		return
	}

	if path == "models" {
		h.models(w, r)
		return
	}

	http.NotFound(w, r)
}

func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	stats, _ := h.db.GetStats()
	data := map[string]interface{}{
		"Active": "home",
		"Stats":  stats,
	}
	h.tmpl.ExecuteTemplate(w, "layout.html", data)
}

func (h *Handler) sessions(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Active": "sessions",
	}
	h.tmpl.ExecuteTemplate(w, "layout.html", data)
}

func (h *Handler) sessionDetail(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Active": "sessions",
	}
	h.tmpl.ExecuteTemplate(w, "layout.html", data)
}

func (h *Handler) models(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Active": "models",
	}
	h.tmpl.ExecuteTemplate(w, "layout.html", data)
}
```

**Step 7: Verify it compiles**

Run: `go build ./...`
Expected: No errors

**Step 8: Commit**

```bash
git add dashboard/ && git commit -m "feat: add dashboard with home, sessions, models pages"
```

---

## Phase 2: Anthropic + Streaming

### Task 8: SSE streaming for OpenAI

**Files:**
- Modify: `proxy/openai.go`

**Step 1: Add streaming proxy method**

Add to `proxy/openai.go`:

```go
func (p *Proxy) proxyOpenAIStream(w http.ResponseWriter, r *http.Request, providerCfg *config.ProviderConfig, target *url.URL, path string, bodyBytes []byte, sessionID int64, startTime time.Time) {
	upstreamURL := target.String() + path
	req, err := http.NewRequestWithContext(r.Context(), "POST", upstreamURL, bytes.NewReader(bodyBytes))
	if err != nil {
		errMsg := err.Error()
		p.db.CompleteSession(sessionID, 0, 0, 0, &errMsg)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+providerCfg.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		errMsg := err.Error()
		p.db.CompleteSession(sessionID, 0, 0, time.Since(startTime).Milliseconds(), &errMsg)
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		errMsg := string(body)
		p.db.CompleteSession(sessionID, 0, 0, time.Since(startTime).Milliseconds(), &errMsg)
		w.WriteHeader(resp.StatusCode)
		w.Write(body)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		errMsg := "streaming not supported"
		p.db.CompleteSession(sessionID, 0, 0, time.Since(startTime).Milliseconds(), &errMsg)
		return
	}

	var promptTokens, completionTokens int64
	buf := make([]byte, 4096)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
			flusher.Flush()

			lines := strings.Split(string(buf[:n]), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if !strings.HasPrefix(line, "data: ") {
					continue
				}
				data := strings.TrimPrefix(line, "data: ")
				if data == "[DONE]" {
					continue
				}
				var chunk openAIStreamChunk
				if err := json.Unmarshal([]byte(data), &chunk); err == nil && chunk.Usage != nil {
					promptTokens = int64(chunk.Usage.PromptTokens)
					completionTokens = int64(chunk.Usage.CompletionTokens)
				}
			}
		}
		if readErr != nil {
			break
		}
	}

	durationMs := time.Since(startTime).Milliseconds()
	p.db.CompleteSession(sessionID, promptTokens, completionTokens, durationMs, nil)
	p.db.CreateRequest(sessionID, promptTokens, completionTokens, durationMs, nil)
}
```

Add `"strings"` to imports in `proxy/openai.go`.

**Step 2: Verify it compiles**

Run: `go build ./...`
Expected: No errors

**Step 3: Commit**

```bash
git add proxy/openai.go && git commit -m "feat: add SSE streaming support for OpenAI"
```

---

### Task 9: Anthropic protocol proxy

**Files:**
- Create: `proxy/anthropic.go`

**Step 1: Implement Anthropic handler**

```go
package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jachy-h/umbra-gate/config"
)

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicResponse struct {
	Model string         `json:"model"`
	Usage anthropicUsage `json:"usage"`
}

type anthropicStreamEvent struct {
	Type  string          `json:"type"`
	Usage *anthropicUsage `json:"usage"`
}

type anthropicMessage struct {
	Model    string `json:"model"`
	Stream   bool   `json:"stream,omitempty"`
}

func (p *Proxy) handleAnthropic(w http.ResponseWriter, r *http.Request, providerName string, providerCfg *config.ProviderConfig, target *url.URL, path string) {
	startTime := time.Now()

	bodyBytes, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		slog.Error("failed to read request body", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var msg anthropicMessage
	json.Unmarshal(bodyBytes, &msg)
	model := msg.Model
	if model == "" {
		model = "unknown"
	}

	slog.Info("request", "provider", providerName, "model", model, "path", path)

	providerID, err := p.db.EnsureProvider(providerName)
	if err != nil {
		slog.Error("failed to ensure provider", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	sessionID, err := p.db.CreateSession(providerID, model)
	if err != nil {
		slog.Error("failed to create session", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if msg.Stream {
		p.proxyAnthropicStream(w, r, providerCfg, target, path, bodyBytes, sessionID, startTime)
	} else {
		p.proxyAnthropicNonStream(w, r, providerCfg, target, path, bodyBytes, sessionID, startTime)
	}
}

func (p *Proxy) proxyAnthropicNonStream(w http.ResponseWriter, r *http.Request, providerCfg *config.ProviderConfig, target *url.URL, path string, bodyBytes []byte, sessionID int64, startTime time.Time) {
	upstreamURL := target.String() + path
	req, err := http.NewRequestWithContext(r.Context(), "POST", upstreamURL, bytes.NewReader(bodyBytes))
	if err != nil {
		errMsg := err.Error()
		p.db.CompleteSession(sessionID, 0, 0, 0, &errMsg)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", providerCfg.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		errMsg := err.Error()
		p.db.CompleteSession(sessionID, 0, 0, time.Since(startTime).Milliseconds(), &errMsg)
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		errMsg := err.Error()
		p.db.CompleteSession(sessionID, 0, 0, time.Since(startTime).Milliseconds(), &errMsg)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var antResp anthropicResponse
	json.Unmarshal(respBody, &antResp)

	durationMs := time.Since(startTime).Milliseconds()
	promptTokens := int64(antResp.Usage.InputTokens)
	completionTokens := int64(antResp.Usage.OutputTokens)

	p.db.CompleteSession(sessionID, promptTokens, completionTokens, durationMs, nil)
	p.db.CreateRequest(sessionID, promptTokens, completionTokens, durationMs, nil)

	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
}

func (p *Proxy) proxyAnthropicStream(w http.ResponseWriter, r *http.Request, providerCfg *config.ProviderConfig, target *url.URL, path string, bodyBytes []byte, sessionID int64, startTime time.Time) {
	upstreamURL := target.String() + path
	req, err := http.NewRequestWithContext(r.Context(), "POST", upstreamURL, bytes.NewReader(bodyBytes))
	if err != nil {
		errMsg := err.Error()
		p.db.CompleteSession(sessionID, 0, 0, 0, &errMsg)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", providerCfg.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		errMsg := err.Error()
		p.db.CompleteSession(sessionID, 0, 0, time.Since(startTime).Milliseconds(), &errMsg)
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		errMsg := string(body)
		p.db.CompleteSession(sessionID, 0, 0, time.Since(startTime).Milliseconds(), &errMsg)
		w.WriteHeader(resp.StatusCode)
		w.Write(body)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		errMsg := "streaming not supported"
		p.db.CompleteSession(sessionID, 0, 0, time.Since(startTime).Milliseconds(), &errMsg)
		return
	}

	var inputTokens, outputTokens int64
	buf := make([]byte, 4096)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
			flusher.Flush()

			lines := strings.Split(string(buf[:n]), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if !strings.HasPrefix(line, "data: ") {
					continue
				}
				data := strings.TrimPrefix(line, "data: ")
				var event anthropicStreamEvent
				if err := json.Unmarshal([]byte(data), &event); err == nil && event.Usage != nil {
					inputTokens = int64(event.Usage.InputTokens)
					outputTokens = int64(event.Usage.OutputTokens)
				}
			}
		}
		if readErr != nil {
			break
		}
	}

	durationMs := time.Since(startTime).Milliseconds()
	p.db.CompleteSession(sessionID, inputTokens, outputTokens, durationMs, nil)
	p.db.CreateRequest(sessionID, inputTokens, outputTokens, durationMs, nil)
}
```

**Step 2: Verify it compiles**

Run: `go build ./...`
Expected: No errors

**Step 3: Commit**

```bash
git add proxy/anthropic.go && git commit -m "feat: add Anthropic protocol proxy with streaming support"
```

---

## Phase 3: Integration test & polish

### Task 10: End-to-end smoke test

**Files:**
- Create: `proxy/proxy_test.go`

**Step 1: Write basic test**

```go
package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProxyUnknownProvider(t *testing.T) {
	cfg := &config.Config{
		Listen:    "127.0.0.1:4141",
		Providers: map[string]config.ProviderConfig{},
	}
	p := New(cfg, nil)

	req := httptest.NewRequest("POST", "/unknown/v1/chat/completions", nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
```

Add `"github.com/jachy-h/umbra-gate/config"` to imports.

**Step 2: Run test**

Run: `go test ./proxy/ -v`
Expected: PASS

**Step 3: Commit**

```bash
git add proxy/proxy_test.go && git commit -m "test: add basic proxy routing test"
```

---

### Task 11: Final build verification

**Step 1: Build the binary**

Run: `go build -o umbragate .`
Expected: Creates `umbragate` binary

**Step 2: Verify binary runs**

Run: `./umbragate 2>&1 &; sleep 1; curl -s http://127.0.0.1:4141/dashboard | head -5; kill %1 2>/dev/null`
Expected: HTML output from dashboard

**Step 3: Commit**

```bash
git add -A && git commit -m "chore: final build verification"
```

---

## Summary

| Phase | Tasks | Description |
|-------|-------|-------------|
| 1 | 1-7 | Project setup, DB, OpenAI non-streaming, API, Dashboard |
| 2 | 8-9 | SSE streaming, Anthropic protocol |
| 3 | 10-11 | Tests, build verification |

**Total: 11 tasks**
