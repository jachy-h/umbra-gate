# Dashboard Time Series and I18n Design

## Scope

Enhance the dashboard with a recent usage trend chart and client-side Chinese/English switching. The time chart shows the last 7 days by default. Language selection is saved in browser `localStorage`.

## Data

Add a database query that aggregates successful request usage by day for the last N days. The initial API will use `days=7`, with bounds validation to avoid expensive queries. Each row includes date, request count, and total tokens. Missing dates are filled with zero values so charts render continuous timelines.

## API

Add `GET /api/timeseries?days=7`. The response is an array ordered by date ascending:

```json
[
  { "date": "2026-06-08", "request_count": 3, "total_tokens": 12000 }
]
```

Invalid or missing `days` falls back to 7.

## UI

Add a "Last 7 Days" card to the dashboard home page. Load Chart.js from a CDN and render a combined chart: tokens as a line and requests as bars. If Chart.js or the API fails, show a simple error/empty state.

## I18n

Use a small front-end dictionary for English and Chinese. The language toggle appears in the nav. Labels, headings, loading/error/empty states, and unit text are updated by JavaScript. The selected language is stored as `dashboard_lang` in `localStorage`.

## Testing

Add tests for the time-series database aggregation and `/api/timeseries`. Update dashboard rendering tests to verify the new chart container, CDN script, and language toggle are present. Run `go test ./...` and `go build ./...`.
