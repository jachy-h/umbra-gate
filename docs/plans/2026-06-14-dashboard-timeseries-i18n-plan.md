# Dashboard Time Series and I18n Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a last-7-days usage chart and Chinese/English switching to the dashboard.

**Architecture:** Add a DB aggregation for daily request counts and token totals, expose it via `/api/timeseries`, and render it on the dashboard with Chart.js from a CDN. Implement i18n on the client with a small dictionary and persist language selection in `localStorage`.

**Tech Stack:** Go, SQLite, `net/http`, Go templates, native JavaScript, Chart.js CDN.

---

### Task 1: Add time-series database aggregation

**Files:**
- Modify: `db/queries.go`
- Test: `db/queries_test.go`

**Step 1: Write the failing test**

Add `TestGetTimeSeriesStatsFillsLastSevenDays` to `db/queries_test.go`. It should:
- Open a temp DB.
- Insert a provider and two successful sessions.
- Insert request rows with explicit `created_at` dates using `database.conn.Exec` because the test is in package `db`.
- Call `GetTimeSeriesStats(7)`.
- Assert length is 7, dates are ascending, missing days are zero, and populated days include request counts and token totals.

**Step 2: Run test to verify it fails**

Run: `go test ./db -run TestGetTimeSeriesStatsFillsLastSevenDays -v`
Expected: FAIL because `GetTimeSeriesStats` does not exist.

**Step 3: Implement minimal production code**

Add:

```go
type TimeSeriesStats struct {
	Date         string `json:"date"`
	RequestCount int64  `json:"request_count"`
	TotalTokens  int64  `json:"total_tokens"`
}
```

Add `GetTimeSeriesStats(days int) ([]TimeSeriesStats, error)`:
- Clamp `days` to 7 if `days <= 0`; cap at 90.
- Build dates from oldest to newest using `time.Now()`.
- Query `requests` grouped by `date(created_at)` where date is within the start date.
- Fill missing dates with zero values.

**Step 4: Run test to verify it passes**

Run: `go test ./db -run TestGetTimeSeriesStatsFillsLastSevenDays -v`
Expected: PASS.

---

### Task 2: Add `/api/timeseries`

**Files:**
- Modify: `api/handler.go`
- Test: `api/handler_test.go`

**Step 1: Write the failing test**

Add `TestTimeSeriesEndpointReturnsDailyStats` to `api/handler_test.go`. It should seed one request, call `GET /timeseries?days=7`, decode `[]db.TimeSeriesStats`, and assert status 200 and length 7.

**Step 2: Run test to verify it fails**

Run: `go test ./api -run TestTimeSeriesEndpointReturnsDailyStats -v`
Expected: FAIL because route or type is missing.

**Step 3: Implement route and handler**

In `ServeHTTP`, add:

```go
case "/timeseries":
	h.handleTimeSeries(w, r)
```

Add handler:

```go
func (h *Handler) handleTimeSeries(w http.ResponseWriter, r *http.Request) {
	days := 7
	if raw := r.URL.Query().Get("days"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			days = parsed
		}
	}
	stats, err := h.db.GetTimeSeriesStats(days)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	if stats == nil {
		stats = []db.TimeSeriesStats{}
	}
	writeJSON(w, stats)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./api -run TestTimeSeriesEndpointReturnsDailyStats -v`
Expected: PASS.

---

### Task 3: Add dashboard chart and i18n UI

**Files:**
- Modify: `dashboard/templates/layout.html`
- Modify: `dashboard/templates/home.html`
- Test: `dashboard/handler_test.go`

**Step 1: Write the failing test**

Update `TestHomeRendersIconStatsAndUsageBreakdowns` to also assert the rendered HTML contains:
- `cdn.jsdelivr.net/npm/chart.js`
- `id="usageTrendChart"`
- `id="languageToggle"`
- `data-i18n="dashboard"`

**Step 2: Run test to verify it fails**

Run: `go test ./dashboard -run TestHomeRendersIconStatsAndUsageBreakdowns -v`
Expected: FAIL because chart/i18n markup does not exist.

**Step 3: Update layout**

Add a language toggle in nav:

```html
<button id="languageToggle" class="lang-toggle" type="button">中文</button>
```

Add CSS for `.nav-spacer`, `.lang-toggle`, and `.trend-card` if needed. Use `data-i18n` attributes on nav labels.

**Step 4: Update home template**

Add Chart.js script:

```html
<script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
```

Add trend card above provider/model grid:

```html
<div class="card trend-card">
    <h2 data-i18n="last7Days">Last 7 Days</h2>
    <div class="chart-container"><canvas id="usageTrendChart"></canvas></div>
    <div id="trendState" class="empty-state"></div>
</div>
```

Add front-end dictionary and language functions:
- `const translations = { en: {...}, zh: {...} }`
- `getLang()` reads `localStorage.dashboard_lang` and defaults to `en`.
- `setLang(lang)` writes localStorage and calls `applyTranslations()`.
- `applyTranslations()` updates all `[data-i18n]` nodes and toggle label.

Update existing labels/headings/loading/error/empty text with `data-i18n` where static, and use translation lookups in JS for dynamic strings.

Add chart loading:
- Fetch `/api/timeseries?days=7`.
- If `window.Chart` is missing, show translated error text.
- Render tokens as line dataset and requests as bar dataset.

**Step 5: Run test to verify it passes**

Run: `go test ./dashboard -run TestHomeRendersIconStatsAndUsageBreakdowns -v`
Expected: PASS.

---

### Task 4: Final verification

**Files:**
- No new files expected beyond existing tests and docs.

**Step 1: Format Go files**

Run: `gofmt -w db/queries.go db/queries_test.go api/handler.go api/handler_test.go dashboard/handler_test.go`
Expected: no output.

**Step 2: Run all tests**

Run: `go test ./...`
Expected: PASS.

**Step 3: Build all packages**

Run: `go build ./...`
Expected: PASS.

**Step 4: Inspect diff**

Run: `git diff --stat`
Expected: only DB/API/dashboard/doc/test changes plus existing gitignore/config tracking changes if still uncommitted.
