# Dashboard Redesign Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Redesign the dashboard home page with icon summary cards and token usage breakdowns by provider and model.

**Architecture:** Keep the current Go server-rendered dashboard and native JavaScript. Add provider aggregation in the DB layer, expose it through `/api/providers`, and update the home template/CSS to render provider/model token bar lists.

**Tech Stack:** Go, `net/http`, SQLite via `modernc.org/sqlite`, Go `html/template`, embedded templates, native browser JavaScript.

---

### Task 1: Add provider usage aggregation

**Files:**
- Modify: `db/queries.go`

**Step 1: Add `ProviderStats` type**

Add near `ModelStats`:

```go
type ProviderStats struct {
	ProviderName  string  `json:"provider_name"`
	RequestCount  int64   `json:"request_count"`
	TotalTokens   int64   `json:"total_tokens"`
	AvgDurationMs float64 `json:"avg_duration_ms"`
}
```

**Step 2: Add `GetProviderStats`**

Add after `GetModelStats`:

```go
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
```

**Step 3: Run tests**

Run: `go test ./...`
Expected: PASS.

---

### Task 2: Expose provider stats API

**Files:**
- Modify: `api/handler.go`

**Step 1: Add route**

In `ServeHTTP`, add:

```go
case "/providers":
	h.handleProviders(w, r)
```

**Step 2: Add handler**

Add near `handleModels`:

```go
func (h *Handler) handleProviders(w http.ResponseWriter, r *http.Request) {
	stats, err := h.db.GetProviderStats()
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	if stats == nil {
		stats = []db.ProviderStats{}
	}
	writeJSON(w, stats)
}
```

**Step 3: Run tests**

Run: `go test ./...`
Expected: PASS.

---

### Task 3: Update dashboard layout styles

**Files:**
- Modify: `dashboard/templates/layout.html`

**Step 1: Update CSS only**

Keep existing classes working. Add styles for icon stat cards, section grids, bar lists, empty state, and responsive behavior.

**Step 2: Run build**

Run: `go test ./...`
Expected: PASS.

---

### Task 4: Redesign dashboard home template

**Files:**
- Modify: `dashboard/templates/home.html`

**Step 1: Replace current content**

Use icon stats and two usage cards. Fetch `/api/providers` and `/api/models`. Render horizontal bars by `total_tokens`.

Implementation requirements:
- Use `{{formatNum .Stats.TodayTokens}}` and `{{formatNum .Stats.MonthTokens}}` for server-rendered summary values.
- Render provider rows from `provider_name`.
- Render model rows from `model`.
- Use max token value to calculate bar percentage.
- Show `No usage yet` for empty arrays.
- Show `Failed to load usage` on fetch errors.
- Keep local `esc` helper.

**Step 2: Run tests**

Run: `go test ./...`
Expected: PASS.

---

### Task 5: Final verification

**Files:**
- No file changes expected.

**Step 1: Format Go code**

Run: `gofmt -w db/queries.go api/handler.go dashboard/handler.go db/queries_test.go api/handler_test.go dashboard/handler_test.go`
Expected: no output.

**Step 2: Run all tests**

Run: `go test ./...`
Expected: PASS.

**Step 3: Build binary**

Run: `go build ./...`
Expected: PASS.

**Step 4: Inspect diff**

Run: `git diff`
Expected: Only intended dashboard/API/query/doc changes.
