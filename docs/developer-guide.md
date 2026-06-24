# Umbragate Developer Guide

## Architecture

```
Client ──► 127.0.0.1:4141 ──► ServeMux
                                  │
                    ┌─────────────┼─────────────┐
                    │             │             │
                /api/*        /dashboard    /a/<agent>/<provider>/*
              (api.Handler) (dashboard.    (proxy.Proxy)
                              Handler)         │
                                         │
                                  passthrough proxy
                                         │
                                    upstream API
```

## Project Structure

```
├── main.go              Entry point, graceful shutdown
├── config.yaml          Default configuration
├── config/config.go     YAML config loading
├── agents/              Agent-specific config managers
├── db/
│   ├── db.go            SQLite connection, migrations, WAL mode
│   └── queries.go       CRUD operations, stats aggregation
├── proxy/
│   ├── proxy.go         HTTP routing by path prefix to provider
│   └── passthrough.go   Transparent forwarding and request accounting
├── api/
│   └── handler.go       JSON API: stats, sessions, models
└── dashboard/
    ├── handler.go        Embedded HTML template handler
    └── templates/        Go templates (layout, home, sessions, models)
```

## Key Design Decisions

- **Agent-aware routing**: New clients use `/a/{agent}/{provider}/...`; legacy `/{provider}/...` remains supported and records `agent_id=unknown`
- **Transparent forwarding**: All providers use the same passthrough path; client headers and bodies are forwarded without provider-specific mutation
- **SSE streaming**: Chunks forwarded immediately via `http.Flusher`; leftover buffer handles line-boundary splits
- **SQLite WAL mode**: Enables concurrent reads (dashboard) while writes (proxy) are in progress
- **Zero cloud dependencies**: Only `gopkg.in/yaml.v3` and `modernc.org/sqlite` (pure Go)

## Adding a New Provider

Add the provider to `config.yaml` with a `base_url`. The client request path is
appended to that base URL and forwarded through the shared passthrough proxy.

## Sessions & Requests

- One session = one API call (MVP simplification)
- Session created on request start (`status: pending`) with agent, provider, model, project, endpoint, and stream attribution
- Session completed on response finish (`status: success/error`)
- Request record created per session (for future multi-request sessions)

## Building

```bash
go build -o umbragate .
```

Single binary, no runtime dependencies.

## Testing

```bash
go test ./...
```

Tests use `httptest` for proxy routing validation, no real upstream needed.

## Configuration

Config is loaded from `UMBRAGATE_HOME/config.yaml` when `UMBRAGATE_HOME` is set, otherwise from `./config.yaml` if present in the working directory, otherwise from `~/.umbragate/config.yaml`. On startup the app creates a default `config.yaml` when missing. All providers are optional — add only what you need.

## Client Proxy Notes

- [Claude Code and Codex proxy notes](claude-code-codex-proxy-notes.md) records how to point those clients at Umbragate and what config-management pieces are still needed.

## Database

- Auto-created SQLite file (`router.db`) in the same app directory as `config.yaml`
- Migrations applied on startup via `schema_migrations` table
- Foreign keys enforced (PRAGMA)
- WAL mode enabled for concurrent access
