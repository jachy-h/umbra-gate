# LLM Gateway Analytics Roadmap Design

## Background

The current project is a local-first LLM gateway written in Go. It already has four usable layers:

1. Proxy layer: forwards requests to OpenAI, Anthropic, and passthrough providers.
2. Persistence layer: stores `providers`, `sessions`, `requests`, and `request_logs` in local SQLite.
3. API layer: exposes `/api/stats`, `/api/sessions`, `/api/models`, `/api/providers`, `/api/timeseries`, and log endpoints.
4. Dashboard layer: shows home metrics, trend chart, model usage, provider usage, sessions, and session-level request logs.

From the current code, the gateway already captures these fields reliably:

- Provider name
- Model name
- Request start/end time
- Request duration
- Prompt tokens
- Completion tokens
- Success/error status
- Error message
- Upstream URL, HTTP method, response status
- Redacted request/response headers and truncated request/response bodies

This means the product already has a solid base for usage, reliability, and performance analytics. The main opportunity is not "whether analytics can be done", but "how to organize analytics into a usable product path".

## Current Data Reality

Before defining roadmap scope, two current constraints matter:

1. The existing schema is request-centric, not user-centric.
The database can explain what provider/model traffic happened, but cannot yet answer who triggered it, from which app, workspace, team, or machine.

2. The existing metrics are strong for aggregates, weak for attribution.
We can already compute totals, trends, error rates, and latency distribution from `requests` and `sessions`, but we cannot yet segment by client type, project, environment, or estimated business owner.

So the roadmap should be phased:

- Phase 1: maximize insight from existing data.
- Phase 2: add metadata for better attribution.
- Phase 3: turn analytics into decision and governance features.

## Analytics Dimensions

### 1. Traffic And Usage

Questions answered:

- How many requests are sent through the gateway?
- How many tokens are consumed?
- Which time ranges are busiest?
- Is usage growing or shrinking?

Core metrics:

- Request count
- Session count
- Prompt tokens
- Completion tokens
- Total tokens
- Active days
- Requests per day/hour
- Tokens per day/hour

Available today:

- Fully available from `requests` and `sessions`

### 2. Provider Dimension

Questions answered:

- Which provider carries most traffic?
- Which provider is fastest?
- Which provider fails more often?
- Are some upstream providers underused or overloaded?

Core metrics:

- Requests by provider
- Tokens by provider
- Success rate by provider
- Error rate by provider
- Average and P95 latency by provider
- Estimated cost by provider

Available today:

- Traffic, tokens, average latency: available
- Error rate: available after adding grouped queries on failed sessions/requests
- P95 latency: can be computed from stored raw rows
- Estimated cost: requires model/provider price table

### 3. Model Dimension

Questions answered:

- Which models are used most?
- Which models are expensive but low value?
- Which models are slow or unstable?
- Is a fallback model taking abnormal traffic?

Core metrics:

- Requests by model
- Tokens by model
- Input/output token split by model
- Success rate by model
- Average and P95 latency by model
- Estimated cost by model
- Share of total usage by model

Available today:

- Requests, tokens, average latency: available
- Error metrics and percentile latency: need new queries
- Cost: needs pricing catalog

### 4. Reliability Dimension

Questions answered:

- How often do requests fail?
- What are the main failure categories?
- Are failures concentrated in a provider, model, or endpoint?

Core metrics:

- Success rate
- Error rate
- Upstream non-200 rate
- Network failure rate
- Error message classification
- Error trend over time

Available today:

- Raw error data is already stored in `sessions.error_message` and `request_logs.response_status`
- Product value comes from aggregation and normalization, not new collection

### 5. Performance Dimension

Questions answered:

- Are responses getting slower?
- Which provider/model has the highest latency?
- How much variability exists between average and tail latency?

Core metrics:

- Average latency
- Median latency
- P95 latency
- P99 latency
- Slow request count above threshold
- Duration trend over time

Available today:

- Raw duration is stored on `sessions` and `requests`
- Only average latency is surfaced today
- Percentiles need new query or in-memory aggregation logic

### 6. Token Structure Dimension

Questions answered:

- Are prompts too large?
- Are completions unexpectedly verbose?
- Which models have poor prompt/output efficiency?

Core metrics:

- Prompt token share
- Completion token share
- Tokens per request
- Completion-to-prompt ratio
- Large prompt outliers

Available today:

- Fully available from token fields

### 7. Time Dimension

Questions answered:

- What happens by hour, day, week, month?
- When do spikes happen?
- Are errors concentrated in specific windows?

Core metrics:

- Daily trend
- Hourly heatmap
- Week-over-week growth
- Rolling 7-day and 30-day averages

Available today:

- Daily trend already exists
- Hourly/weekly analysis needs new group-by queries

### 8. Endpoint And Protocol Dimension

Questions answered:

- Which API paths are used most?
- Is traffic mostly chat, messages, or passthrough endpoints?
- Are stream requests slower or more failure-prone?

Core metrics:

- Requests by upstream path
- Requests by HTTP method
- Stream vs non-stream request count
- Error rate by path
- Latency by path

Available today:

- Path and method are already stored in `request_logs`
- Stream vs non-stream is not persisted explicitly and should be added

### 9. Cost Dimension

Questions answered:

- What is the estimated spend by model/provider/time range?
- Which traffic segment contributes most cost?
- Which cheaper alternatives could replace expensive usage?

Core metrics:

- Estimated input cost
- Estimated output cost
- Total estimated cost
- Cost by provider/model/time
- Cost per successful request

Available today:

- Requires a pricing registry and cost calculation logic
- Token data already exists, so this is a high-value additive feature

### 10. Source Attribution Dimension

Questions answered:

- Which app or SDK produced the traffic?
- Which local workspace/project consumes the most tokens?
- Which environment or operator is responsible for failures?

Core metrics:

- Requests by client app
- Tokens by workspace/project
- Latency and error rate by source

Not available today:

- Needs new metadata capture, such as custom headers or config-level source tagging

## Recommended Product Roadmap

### Phase 1: Strengthen Existing Analytics

Goal:
Turn current raw usage into a dependable operational dashboard without changing the proxy contract.

Deliverables:

- Overview page with usage, reliability, and latency summary cards
- Provider analytics page
- Enhanced model analytics page
- Failure analysis page
- More flexible time filters: `24h`, `7d`, `30d`, `90d`
- Top slow requests and top failed requests views

Why first:

- Highest value for lowest engineering cost
- Uses data already stored in SQLite
- Produces immediate product differentiation for a local gateway

### Phase 2: Add Cost And Protocol Insights

Goal:
Help users decide whether current routing choices are efficient.

Deliverables:

- Pricing catalog for major models/providers
- Cost estimation in API and dashboard
- Input/output token composition views
- Endpoint and path analytics
- Stream vs non-stream comparison

Why second:

- Cost analytics becomes meaningful once traffic and reliability views are stable
- Requires some schema extension, but not a full architecture change

### Phase 3: Add Source Attribution

Goal:
Answer "who or what caused this usage" instead of only "what happened".

Deliverables:

- Client/source metadata ingestion
- Requests by app, workspace, project, or environment
- Team-level and project-level breakdowns
- Source-aware anomaly views

Why third:

- Requires protocol or client integration changes
- Adds the biggest product value for multi-project users, but has higher rollout complexity

### Phase 4: Add Governance And Optimization

Goal:
Move from descriptive analytics to decision support.

Deliverables:

- Budget thresholds and alerts
- Provider/model recommendation rules
- Automatic fallback reporting
- Error anomaly detection
- Cost spike detection

Why fourth:

- Depends on stable analytics primitives from earlier phases
- Better built after the product proves its core metric quality

## Feature Prioritization Matrix

### P0: Should Build Next

- Success/error rate by provider and model
- Latency percentiles by provider and model
- Time range filters across all analytics APIs
- Failure leaderboard with error category aggregation
- Provider analytics page in dashboard

### P1: High Value

- Cost estimation by provider/model
- Hourly heatmap and peak usage windows
- Endpoint/path analytics
- Stream vs non-stream segmentation

### P2: Strategic

- Source attribution
- Budget controls
- Alerts and anomaly detection
- Recommendation engine

## Detailed Implementation Plan

## 1. Database Changes

### Keep Existing Tables

Current tables are sufficient for Phase 1:

- `providers`
- `sessions`
- `requests`
- `request_logs`

### Additive Schema For Phase 2+

Recommended new columns in `requests`:

- `is_stream INTEGER NOT NULL DEFAULT 0`
- `endpoint TEXT NOT NULL DEFAULT ''`
- `client_name TEXT NOT NULL DEFAULT ''`
- `source_project TEXT NOT NULL DEFAULT ''`

Recommended new table:

```sql
CREATE TABLE IF NOT EXISTS model_pricing (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    provider_name TEXT NOT NULL,
    model TEXT NOT NULL,
    input_price_per_million REAL NOT NULL,
    output_price_per_million REAL NOT NULL,
    currency TEXT NOT NULL DEFAULT 'USD',
    effective_from TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(provider_name, model)
);
```

Recommended indexes:

- `idx_sessions_started_at`
- `idx_sessions_provider_model`
- `idx_requests_created_at`
- `idx_requests_session_id_created_at`
- `idx_requests_stream_endpoint`
- `idx_request_logs_status_created_at`

Notes:

- Phase 1 should avoid disruptive schema changes.
- Phase 2 should add only append-only fields and indexes.

## 2. Proxy Instrumentation Changes

### Phase 1

No protocol changes required.

### Phase 2

Persist these derived fields per request:

- `is_stream`: extracted from request body for OpenAI/Anthropic and inferred in passthrough if possible
- `endpoint`: derived from upstream path, such as `/v1/chat/completions` or `/v1/messages`

### Phase 3

Support optional source attribution via headers:

- `X-Gateway-Client`
- `X-Gateway-Project`
- `X-Gateway-Source`

Rules:

- Treat them as optional metadata only
- Redact nothing unless they are explicitly secret-bearing
- Keep backward compatibility for clients that do not send them

## 3. Query Layer Additions

Recommended new query families in `db/queries.go`:

- `GetOverviewStats(range)`
- `GetProviderAnalytics(range)`
- `GetModelAnalytics(range)`
- `GetFailureStats(range)`
- `GetLatencyDistribution(range, dimension)`
- `GetHourlyUsage(range)`
- `GetEndpointStats(range)`
- `GetEstimatedCost(range, dimension)`

Recommended output shapes:

- overview summary object
- list-by-provider rows
- list-by-model rows
- time-series rows
- histogram/percentile rows
- failure category rows

Implementation guidance:

- Keep queries SQLite-friendly
- For percentile metrics, start with in-memory calculation after fetching bounded rows for the selected range
- If row volume grows later, introduce pre-aggregated rollups rather than premature complexity now

## 4. API Design

### Keep Existing Endpoints Stable

Do not break:

- `/api/stats`
- `/api/sessions`
- `/api/models`
- `/api/providers`
- `/api/timeseries`
- `/api/logs`

### Add New Endpoints

Recommended additions:

- `GET /api/overview?range=7d`
- `GET /api/providers/analytics?range=7d`
- `GET /api/models/analytics?range=7d`
- `GET /api/failures?range=7d`
- `GET /api/latency?range=7d&by=provider`
- `GET /api/usage/hourly?range=7d`
- `GET /api/endpoints?range=7d`
- `GET /api/costs?range=7d&by=model`

API rules:

- Use a shared `range` parser: `24h`, `7d`, `30d`, `90d`
- Return empty arrays instead of `null`
- Preserve the current JSON error style

## 5. Dashboard Information Architecture

Recommended navigation evolution:

1. Home
2. Sessions
3. Models
4. Providers
5. Failures
6. Costs

### Home

Add:

- Total requests
- Total tokens
- Success rate
- Average latency
- P95 latency
- Cost estimate
- 24h/7d/30d trend switcher
- Peak usage periods

### Providers

Turn the current provider view into analytics plus configuration split:

- Provider usage leaderboard
- Provider latency comparison
- Provider reliability comparison
- Provider cost comparison
- Separate tab or section for configuration management

### Models

Extend the existing page with:

- Success rate
- P95 latency
- Input/output token ratio
- Cost estimate
- Range filter

### Failures

New page focused on debugging patterns:

- Error trend line
- Top error categories
- Failures by provider/model/endpoint
- Recent failure table with drilldown to request log

### Costs

New page focused on optimization:

- Spend by provider/model
- High-cost low-volume outliers
- Cost trend over time
- Input/output cost composition

## 6. Error Categorization Strategy

Raw error strings are useful for debugging but weak for analysis. Add lightweight categorization:

- `network_error`
- `timeout`
- `upstream_4xx`
- `upstream_5xx`
- `client_disconnect`
- `stream_parse_error`
- `unknown`

Implementation approach:

- Start with derived categorization in query/service code
- Add persisted category column only if repeated computation becomes cumbersome

## 7. Cost Calculation Strategy

Formula:

- `input_cost = prompt_tokens / 1_000_000 * input_price_per_million`
- `output_cost = completion_tokens / 1_000_000 * output_price_per_million`
- `total_cost = input_cost + output_cost`

Fallback rules:

- Exact provider+model match first
- Prefix or alias match second
- Unknown pricing returns `null` cost instead of fake zero

This avoids misleading financial summaries.

## 8. Rollout Plan

### Milestone A

Scope:

- Time range filters
- Success/error rate metrics
- Latency percentile metrics
- Provider analytics page
- Failure analytics page

Expected impact:

- Makes the gateway operationally observable

### Milestone B

Scope:

- Cost estimation
- Endpoint analytics
- Stream segmentation

Expected impact:

- Makes routing and model selection decisions more data-driven

### Milestone C

Scope:

- Source attribution metadata
- Project/app breakdowns

Expected impact:

- Makes the gateway useful across multiple local tools and teams

## 9. Testing Plan

### Unit Tests

- Query tests for provider/model/failure aggregations
- Cost calculation tests
- Error categorization tests
- Range parser tests
- Percentile calculation tests

### API Tests

- Endpoint success responses
- Empty-state responses
- Invalid range handling
- JSON shape stability

### Integration Verification

- `go test ./...`
- `go build ./...`
- Manual dashboard verification against a populated `router.db`

## 10. Recommended Next Build Order

If only one roadmap is executed now, the best order is:

1. Add shared time-range filtering to current analytics APIs.
2. Add provider/model reliability and percentile latency queries.
3. Create a dedicated failures dashboard page and API.
4. Split provider analytics from provider configuration UI.
5. Add pricing table and estimated cost analytics.
6. Add request metadata fields for stream, endpoint, and source attribution.

## Summary

The current project is already strong enough to become an analytics-first local LLM gateway. The best near-term strategy is to avoid over-expanding collection and instead fully exploit the existing `sessions`, `requests`, and `request_logs` data.

The most valuable next product step is Phase 1: reliability, latency, and time-range analytics. After that, cost and source attribution become the two biggest multipliers for product value.
