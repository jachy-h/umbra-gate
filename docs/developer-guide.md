# Personal AI Router — Developer Guide

## Architecture

```
Client ──► 127.0.0.1:4141 ──► ServeMux
                                  │
                    ┌─────────────┼─────────────┐
                    │             │             │
                /api/*        /dashboard    /<provider>/*
              (api.Handler) (dashboard.    (proxy.Proxy)
                              Handler)         │
                                         ┌─────┴─────┐
                                     OpenAI       Anthropic
                                   (openai.go)  (anthropic.go)
                                         │
                                    upstream API
```

## Project Structure

```
├── main.go              Entry point, graceful shutdown
├── config.yaml          Default configuration
├── config/config.go     YAML config loading
├── db/
│   ├── db.go            SQLite connection, migrations, WAL mode
│   └── queries.go       CRUD operations, stats aggregation
├── proxy/
│   ├── proxy.go         HTTP routing by path prefix to provider
│   ├── openai.go        OpenAI-compatible handler (stream + non-stream)
│   └── anthropic.go     Anthropic handler (stream + non-stream)
├── api/
│   └── handler.go       JSON API: stats, sessions, models
└── dashboard/
    ├── handler.go        Embedded HTML template handler
    └── templates/        Go templates (layout, home, sessions, models)
```

## Key Design Decisions

- **Path-prefix routing**: Provider identified by first URL segment (`/openai/...`, `/anthropic/...`)
- **Protocol switch**: Each provider has a `protocol` field; proxy dispatches to the correct handler
- **SSE streaming**: Chunks forwarded immediately via `http.Flusher`; leftover buffer handles line-boundary splits
- **SQLite WAL mode**: Enables concurrent reads (dashboard) while writes (proxy) are in progress
- **Zero cloud dependencies**: Only `gopkg.in/yaml.v3` and `modernc.org/sqlite` (pure Go)

## Adding a New Provider Protocol

1. Add the provider to `config.yaml` with appropriate `protocol` value
2. Create a new handler file (e.g., `proxy/google.go`)
3. Implement `handle<Name>` + `proxy<Name>NonStream` + `proxy<Name>Stream`
4. Add the protocol case in `proxy/proxy.go`'s `ServeHTTP` switch

## Sessions & Requests

- One session = one API call (MVP simplification)
- Session created on request start (`status: pending`)
- Session completed on response finish (`status: success/error`)
- Request record created per session (for future multi-request sessions)

## Building

```bash
go build -o personal-ai-router .
```

Single binary, no runtime dependencies.

## Testing

```bash
go test ./...
```

Tests use `httptest` for proxy routing validation, no real upstream needed.

## Configuration

Config is loaded from `config.yaml` in the working directory. All providers are optional — add only what you need.

## Database

- Auto-created SQLite file (`router.db`)
- Migrations applied on startup via `schema_migrations` table
- Foreign keys enforced (PRAGMA)
- WAL mode enabled for concurrent access
