# Personal AI Router — Design Document

**Date:** 2026-06-13
**Status:** Approved

---

## Overview

Personal AI Router is a local-first, single-user LLM Gateway. It sits between OpenCode (or any OpenAI/Anthropic-compatible client) and upstream LLM providers, transparently proxying requests while collecting usage statistics. All data stays local in SQLite. The entire application ships as a single Go binary.

## Architecture

```
OpenCode                        Personal AI Router              Upstream Providers
────────                        ──────────────────              ───────────────────
                                 127.0.0.1:4141

POST /volcengine/v1/chat/... ──► /volcengine/* ──────────────► volcengine API
POST /anthropic/v1/messages ──► /anthropic/*  ──────────────► anthropic API
POST /openai/v1/chat/...    ──► /openai/*     ──────────────► openai API

GET  /dashboard              ──► Dashboard (HTML templates)
GET  /api/stats              ──► JSON API
GET  /api/sessions           ──► JSON API
GET  /api/models             ──► JSON API
```

- Gateway listens on `127.0.0.1:4141` (configurable)
- URL path prefix identifies the provider (e.g., `/volcengine`)
- Prefix is stripped before forwarding to upstream
- Dashboard and API endpoints serve on the same port

## Config File

Located alongside the binary as `config.yaml` (will move to `~/.personal-ai-router/` later).

```yaml
listen: "127.0.0.1:4141"

providers:
  volcengine:
    base_url: https://ark.cn-beijing.volces.com/api/coding/v3
    api_key: sk-xxx
    protocol: openai-compatible

  anthropic:
    base_url: https://api.anthropic.com
    api_key: sk-ant-xxx
    protocol: anthropic

  openai:
    base_url: https://api.openai.com
    api_key: sk-xxx
    protocol: openai-compatible

storage:
  save_prompt: false
```

- `protocol`: `openai-compatible` or `anthropic` — determines which proxy handler to use
- No model list in config (not needed for routing; pricing deferred)
- `save_prompt`: defaults to `false`, controls whether prompt/response content is stored

## Provider Routing

Provider is determined by the first path segment after the host:

```
/volcengine/v1/chat/completions  →  provider "volcengine"
/anthropic/v1/messages           →  provider "anthropic"
/openai/v1/chat/completions      →  provider "openai"
```

Gateway strips the provider prefix and forwards the remaining path to the provider's `base_url`.

OpenCode configuration for each provider points to a different path:
```
volcengine  → http://127.0.0.1:4141/volcengine
anthropic   → http://127.0.0.1:4141/anthropic
openai      → http://127.0.0.1:4141/openai
```

## SQLite Schema

```sql
CREATE TABLE schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE providers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE sessions (
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
);

CREATE TABLE requests (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id INTEGER NOT NULL REFERENCES sessions(id),
    prompt_tokens INTEGER DEFAULT 0,
    completion_tokens INTEGER DEFAULT 0,
    duration_ms INTEGER DEFAULT 0,
    error_message TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```

- MVP: one session = one request (can be extended later)
- Token counts extracted from response body (OpenAI: `usage.prompt_tokens`/`completion_tokens`; Anthropic: `usage.input_tokens`/`output_tokens`)
- No cost fields (deferred)
- No cwd/git_branch fields (deferred)

## Request Flow

### Non-streaming

1. Client sends `POST /volcengine/v1/chat/completions`
2. Gateway extracts provider name from path (`volcengine`)
3. Gateway looks up provider config (base_url, api_key, protocol)
4. Gateway strips prefix → `/v1/chat/completions`
5. Gateway creates session record (status=pending)
6. Gateway replaces `apiKey` in request headers with real key
7. Gateway proxies request to upstream
8. Gateway reads full response, extracts token usage
9. Gateway updates session (status=success, tokens, duration)
10. Gateway creates request record
11. Gateway returns response to client

### SSE Streaming

1-6. Same as above
7. Gateway sets up streaming proxy:
   - Reads SSE chunks from upstream response body
   - Writes each chunk immediately to client via `http.Flusher`
   - Accumulates token usage from the final `[DONE]` chunk or `usage` field
8. After stream ends, updates session + creates request record

## Dashboard

### Pages

| Page | Route | Description |
|------|-------|-------------|
| Home | `/dashboard` | Today's requests, month's requests, month's tokens, top models, top providers |
| Sessions | `/dashboard/sessions` | List of sessions (time, provider, model, tokens, duration, status), detail view |
| Models | `/dashboard/models` | Aggregated by model: request count, tokens, avg duration |

### Technology

- Go `html/template` with `embed` for single-binary deployment
- Minimal vanilla JavaScript for charts (bar charts for model usage, etc.)
- No CSS framework — simple, functional styling

## Internal API

| Endpoint | Description |
|----------|-------------|
| `GET /api/stats` | Aggregate statistics (today, this month) |
| `GET /api/sessions` | List sessions (paginated) |
| `GET /api/sessions/:id` | Single session detail |
| `GET /api/models` | Model-level aggregation |

All APIs listen on `127.0.0.1` only.

## Project Structure

```
llm-gateway/
├── main.go
├── go.mod
├── go.sum
├── config.yaml              # Default config (user edits this)
├── config/
│   └── config.go            # YAML config parsing
├── db/
│   ├── db.go                # SQLite init, connection, migrations
│   └── queries.go           # CRUD operations
├── proxy/
│   ├── proxy.go             # Core reverse proxy logic
│   ├── openai.go            # OpenAI-compatible protocol handler
│   └── anthropic.go         # Anthropic protocol handler
├── dashboard/
│   ├── handler.go           # Dashboard HTTP handlers
│   └── templates/           # Go HTML templates (embedded)
│       ├── layout.html
│       ├── home.html
│       ├── sessions.html
│       ├── session_detail.html
│       └── models.html
├── api/
│   └── handler.go           # /api/* JSON handlers
└── static/
    └── app.js               # Minimal JS for charts
```

## Technology Choices

| Component | Choice | Rationale |
|-----------|--------|-----------|
| Language | Go (latest stable) | Single binary, fast, stdlib covers most needs |
| HTTP | `net/http` | Zero dependencies |
| Router | `net/http` ServeMux (Go 1.22+ patterns) | No framework needed |
| Config | `gopkg.in/yaml.v3` | One external dep, human-readable |
| Database | SQLite via `modernc.org/sqlite` | Pure Go, no CGO, single file |
| Templates | `html/template` + `embed` | Single binary, no external files |
| Logging | `log/slog` | Structured, stdlib |
| Charts | Vanilla JS (Canvas/SVG) | No framework |

## Out of Scope (MVP)

- User system / authentication
- Cloud sync / multi-user
- WebSocket
- Plugin system
- Prompt AI analysis / embedding / RAG
- Agent / auto task naming
- PR / GitHub integration
- Mobile support
- Docker
- PostgreSQL / Redis
- Cost calculation
- cwd / git_branch collection
- Tasks page (depends on cwd/git_branch)

## Development Phases

### Phase 1: Core Proxy + Stats
- OpenAI-compatible protocol proxy (non-streaming)
- SQLite schema + migrations
- Session/request recording
- Home dashboard with basic stats
- Config file loading

### Phase 2: Anthropic + Streaming
- Anthropic protocol proxy
- SSE streaming for both protocols
- Verify OpenCode works with both providers

### Phase 3: Dashboard Pages
- Sessions list + detail page
- Models aggregation page
- Charts with vanilla JS

### Phase 4: Polish
- Config file path migration to `~/.personal-ai-router/`
- Prompt storage toggle
- Migration mechanism
- Release build

## Acceptance Criteria

1. OpenCode works after changing baseURL to `http://127.0.0.1:4141/<provider>`
2. Supports OpenAI-compatible protocol (volcengine, openai, etc.)
3. Supports Anthropic Messages API
4. SSE streaming is transparent (no buffering)
5. SQLite database auto-created
6. Dashboard shows usage statistics
7. Prompts not saved by default
8. All data local
9. Single Go binary
10. No external dependencies to install
