# Dashboard Redesign Design

## Scope

Redesign the dashboard home page to make usage easier to scan visually. The dashboard will show icon-based summary cards and token usage breakdowns by provider and model. CWD-based usage is out of scope for this iteration because the gateway does not currently capture cwd metadata.

## Data

Usage will be measured primarily by total tokens. Existing model usage data is available through `GetModelStats` and `/api/models`. Add provider usage aggregation with the same shape of metrics: provider name, request/session count, total tokens, and average duration.

Provider totals should join `sessions` with `providers`, include successful sessions only, group by provider name, and sort by total tokens descending.

## API

Add `GET /api/providers`, returning an array of provider usage rows. Keep existing endpoints unchanged.

## UI

The dashboard home page will keep server-rendered Go templates and native JavaScript. No charting dependency will be added.

Top summary cards will use simple inline icons for today requests, month requests, today tokens, and month tokens. Below the cards, show two usage cards:

- Tokens by Provider
- Tokens by Model

Each usage card will render a lightweight horizontal bar list using API data. Empty states will be shown when no usage exists.

## Error Handling

API handlers return the existing JSON error style. UI fetch failures or empty responses should show a simple empty/error row instead of leaving loading text indefinitely.

## Testing

Verify with Go tests and a full build. Add targeted database/query or API tests if existing test structure makes that practical.
