# LLM Gateway Analytics Roadmap Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Turn the current LLM gateway into an analytics-first product by shipping phased usage, reliability, latency, cost, and attribution capabilities without disrupting existing proxy behavior.

**Architecture:** Reuse the current Go + SQLite + embedded template stack. Phase 1 maximizes the current `sessions`, `requests`, and `request_logs` data model. Later phases add lightweight schema extensions for cost, endpoint, stream, and source attribution. Keep existing APIs stable while adding new analytics endpoints and dashboard pages.

**Tech Stack:** Go, SQLite, `net/http`, Go templates, native JavaScript, current dashboard structure, current test layout.

---

### Task 1: Build shared analytics range infrastructure

**Files:**
- Modify: `db/queries.go`
- Modify: `api/handler.go`
- Add/Modify tests: `db/queries_test.go`
- Add/Modify tests: `api/handler_test.go`

**Step 1: Write failing tests**

Add tests for a shared analytics range parser and range-aware queries:

- `24h`, `7d`, `30d`, `90d` are accepted.
- Empty or invalid range falls back to `7d`.
- Over-large ranges are clamped.
- Existing time-series and future analytics endpoints return data scoped to the selected range.

For DB tests, prefer inserting rows with explicit timestamps to validate filtering boundaries.

**Step 2: Run tests to verify failure**

Run:

`go test ./db ./api`

Expected: FAIL because shared range support does not exist yet.

**Step 3: Implement minimal production code**

Add a small shared range model, either in `db/queries.go` or a nearby package-level helper, with:

- canonical range parsing
- start-time calculation
- safe defaults

Update time-aware queries to optionally accept a range input rather than always using hard-coded today/month windows.

Keep existing endpoints backward compatible.

**Step 4: Run tests to verify pass**

Run:

`go test ./db ./api`

Expected: PASS.

---

### Task 2: Add provider and model reliability analytics

**Files:**
- Modify: `db/queries.go`
- Modify: `api/handler.go`
- Add/Modify tests: `db/queries_test.go`
- Add/Modify tests: `api/handler_test.go`

**Step 1: Write failing tests**

Add coverage for grouped reliability metrics by provider and model:

- request/session totals
- success count
- error count
- success rate
- error rate

Seed both successful and failed sessions so the tests validate mixed outcomes, not just the happy path.

**Step 2: Run tests to verify failure**

Run:

`go test ./db ./api`

Expected: FAIL because analytics query types and routes do not exist.

**Step 3: Implement DB queries**

Add new result types and queries such as:

- `ProviderAnalytics`
- `ModelAnalytics`
- `GetProviderAnalytics(range)`
- `GetModelAnalytics(range)`

Recommended fields:

- dimension key (`provider_name` or `model`)
- request/session count
- total tokens
- avg latency
- success count
- error count
- success rate

Use the current `sessions` table as the primary source for reliability because status and error state already live there.

**Step 4: Implement API routes**

Add endpoints such as:

- `GET /api/providers/analytics?range=7d`
- `GET /api/models/analytics?range=7d`

Do not remove or change the existing `/api/providers` and `/api/models` endpoints.

**Step 5: Run tests**

Run:

`go test ./db ./api`

Expected: PASS.

---

### Task 3: Add latency percentile analytics

**Files:**
- Modify: `db/queries.go`
- Modify: `api/handler.go`
- Add/Modify tests: `db/queries_test.go`
- Add/Modify tests: `api/handler_test.go`

**Step 1: Write failing tests**

Add tests for percentile metrics using fixed duration fixtures:

- median
- P95
- P99

Validate both per-provider and per-model aggregation with deterministic data.

**Step 2: Run tests to verify failure**

Run:

`go test ./db ./api`

Expected: FAIL because percentile support does not exist.

**Step 3: Implement minimal percentile logic**

Start with a bounded in-memory percentile calculation based on durations selected for the requested range.

Guidelines:

- keep implementation small and deterministic
- use successful sessions only for latency summaries unless a different rule is explicitly needed
- return zero values for empty result sets

Add API support, for example:

- `GET /api/latency?range=7d&by=provider`
- `GET /api/latency?range=7d&by=model`

**Step 4: Run tests**

Run:

`go test ./db ./api`

Expected: PASS.

---

### Task 4: Add failure analytics and error categorization

**Files:**
- Modify: `db/queries.go`
- Modify: `api/handler.go`
- Add/Modify tests: `db/queries_test.go`
- Add/Modify tests: `api/handler_test.go`

**Step 1: Write failing tests**

Add tests for:

- failure counts over time
- grouped failures by provider/model
- error category aggregation
- recent failure list ordering

Use representative messages and statuses for categories like:

- network error
- timeout
- upstream 4xx
- upstream 5xx
- unknown

**Step 2: Run tests to verify failure**

Run:

`go test ./db ./api`

Expected: FAIL because failure analytics queries are missing.

**Step 3: Implement categorization and queries**

Add lightweight derived categorization in Go rather than changing schema first.

Recommended new outputs:

- failure summary rows
- error category rows
- recent failure session rows

Add endpoint:

- `GET /api/failures?range=7d`

Keep response shapes simple and dashboard-friendly.

**Step 4: Run tests**

Run:

`go test ./db ./api`

Expected: PASS.

---

### Task 5: Add Phase 1 overview analytics endpoint

**Files:**
- Modify: `db/queries.go`
- Modify: `api/handler.go`
- Add/Modify tests: `db/queries_test.go`
- Add/Modify tests: `api/handler_test.go`

**Step 1: Write failing tests**

Add tests for an overview endpoint returning the key Phase 1 summary metrics:

- total requests
- total sessions
- total tokens
- success rate
- avg latency
- P95 latency
- error count

**Step 2: Run tests to verify failure**

Run:

`go test ./db ./api`

Expected: FAIL because overview aggregation does not exist.

**Step 3: Implement overview aggregation and route**

Add:

- `GetOverviewStats(range)`
- `GET /api/overview?range=7d`

Keep the existing `/api/stats` endpoint for current UI compatibility. The new overview endpoint is the richer analytics-oriented contract.

**Step 4: Run tests**

Run:

`go test ./db ./api`

Expected: PASS.

---

### Task 6: Add dashboard range selector and overview cards

**Files:**
- Modify: `dashboard/templates/layout.html`
- Modify: `dashboard/templates/home.html`
- Modify: `dashboard/handler.go` if needed for rendering hooks
- Add/Modify tests: `dashboard/handler_test.go`

**Step 1: Write failing tests**

Update dashboard render tests to assert the home page contains:

- a range selector
- placeholders for reliability metrics
- placeholders for latency metrics
- overview data containers fed by the new API

**Step 2: Run tests to verify failure**

Run:

`go test ./dashboard`

Expected: FAIL because the overview analytics UI does not exist.

**Step 3: Implement home page updates**

Add a compact analytics control row with supported ranges:

- `24h`
- `7d`
- `30d`
- `90d`

Update the home page to render or fetch:

- total requests
- total tokens
- success rate
- avg latency
- P95 latency
- top provider summary
- top model summary

Keep the implementation in native JavaScript and aligned with the current dashboard style.

**Step 4: Run tests**

Run:

`go test ./dashboard`

Expected: PASS.

---

### Task 7: Add provider analytics dashboard page

**Files:**
- Modify: `dashboard/handler.go`
- Modify: `dashboard/templates/providers.html`
- Add/Modify tests: `dashboard/handler_test.go`

**Step 1: Write failing tests**

Update dashboard tests to assert the provider page now contains analytics-oriented sections such as:

- provider leaderboard
- provider success rate
- provider latency comparison
- range-aware usage container

If the page currently mixes config and analytics too tightly, first assert a clear split in rendered markup.

**Step 2: Run tests to verify failure**

Run:

`go test ./dashboard`

Expected: FAIL because provider analytics UI is not present.

**Step 3: Implement provider analytics UI**

Refactor the provider page so it can support both:

- provider analytics
- provider or gateway configuration management

Prefer one of these minimal patterns:

- analytics first, config second on the same page
- separate sections/tabs with clear headings

Do not remove existing management capability.

**Step 4: Run tests**

Run:

`go test ./dashboard`

Expected: PASS.

---

### Task 8: Add failure analytics dashboard page

**Files:**
- Modify: `dashboard/handler.go`
- Add: `dashboard/templates/failures.html`
- Modify related template parsing setup
- Add/Modify tests: `dashboard/handler_test.go`

**Step 1: Write failing tests**

Add dashboard tests covering:

- route handling for `/dashboard/failures`
- page title and failure-specific containers
- recent failures list placeholder
- error category chart or list placeholder

**Step 2: Run tests to verify failure**

Run:

`go test ./dashboard`

Expected: FAIL because the failures page and route do not exist.

**Step 3: Implement dashboard page**

Add a new failures page with:

- failure summary cards
- category breakdown
- provider/model failure breakdown
- recent failed sessions with links to detail or log views

Keep it lightweight and reuse existing styling patterns where possible.

**Step 4: Run tests**

Run:

`go test ./dashboard`

Expected: PASS.

---

### Task 9: Prepare Phase 2 cost analytics foundation

**Files:**
- Modify: `db/db.go`
- Modify: `db/queries.go`
- Add/Modify tests: `db/queries_test.go`
- Optionally add a small pricing helper file if needed

**Step 1: Write failing tests**

Add tests for:

- model pricing lookup
- input/output cost calculation
- unknown pricing behavior returns missing cost rather than fake zero

**Step 2: Run tests to verify failure**

Run:

`go test ./db`

Expected: FAIL because pricing support does not exist.

**Step 3: Implement minimal schema and logic**

Add `model_pricing` migration and a minimal query surface for cost computation.

Keep scope intentionally small:

- exact provider+model match first
- straightforward formula based on stored tokens
- no automatic external pricing sync

**Step 4: Run tests**

Run:

`go test ./db`

Expected: PASS.

---

### Task 10: Prepare Phase 2 protocol attribution fields

**Files:**
- Modify: `db/db.go`
- Modify: `db/queries.go`
- Modify: `proxy/openai.go`
- Modify: `proxy/anthropic.go`
- Modify: `proxy/passthrough.go`
- Add/Modify tests: `proxy/proxy_test.go`
- Add/Modify tests: `db/queries_test.go`

**Step 1: Write failing tests**

Add tests that verify request rows can persist and expose:

- `is_stream`
- `endpoint`

If practical, add tests for optional custom headers to support future source attribution.

**Step 2: Run tests to verify failure**

Run:

`go test ./proxy ./db`

Expected: FAIL because request metadata fields do not exist.

**Step 3: Implement minimal schema and proxy wiring**

Add append-only columns and persist:

- stream flag
- normalized endpoint path

Do not force clients to provide source metadata yet.

**Step 4: Run tests**

Run:

`go test ./proxy ./db`

Expected: PASS.

---

### Task 11: Final Phase 1 verification

**Files:**
- All changed files

**Step 1: Format Go files**

Run:

`gofmt -w db/*.go api/*.go dashboard/*.go proxy/*.go`

Expected: no output.

**Step 2: Run full test suite**

Run:

`go test ./...`

Expected: PASS.

**Step 3: Build all packages**

Run:

`go build ./...`

Expected: PASS.

**Step 4: Inspect diff**

Run:

`git diff --stat`

Expected: only intended analytics-related code, tests, templates, and docs changed.

---

## Suggested Delivery Sequence

If implementation starts immediately, execute in this order:

1. Task 1: shared range infrastructure
2. Task 2: provider/model reliability analytics
3. Task 3: latency percentile analytics
4. Task 4: failure analytics
5. Task 5: overview endpoint
6. Task 6: home dashboard updates
7. Task 7: provider analytics page
8. Task 8: failures page

Tasks 9 and 10 should follow after Phase 1 ships, unless cost or endpoint analytics becomes urgent.

## Acceptance Criteria

Phase 1 is complete when:

- analytics APIs support shared time ranges
- provider/model analytics include success/error and latency summaries
- failure analytics is available via API and dashboard
- home dashboard shows more than raw request/token totals
- all existing endpoints continue to work
- `go test ./...` and `go build ./...` both pass

## Risks And Mitigations

### Risk 1: SQLite percentile queries become complex

Mitigation:

- compute percentiles in Go first using bounded result sets

### Risk 2: Existing provider page becomes overloaded

Mitigation:

- separate analytics and configuration sections clearly

### Risk 3: Error messages are too inconsistent for useful grouping

Mitigation:

- start with coarse categories and refine only after real data review

### Risk 4: Cost analytics creates false confidence with incomplete pricing

Mitigation:

- return missing cost where pricing is unknown instead of zero or guessed values
