# Agent, Provider, and Analytics Product Redesign

## Background

Umbragate is moving from a provider-only local proxy into a local Agent gateway
control plane. The product should manage how different coding agents connect to
upstream model providers, while preserving enough request metadata to analyze
usage across agent, provider, model, time, session, and project dimensions.

The existing architecture already provides a solid base:

- `config.yaml` owns gateway provider definitions.
- The proxy routes `/<provider>/...` to the configured upstream provider.
- SQLite stores providers, sessions, requests, and recent request logs.
- Dashboard and JSON APIs already expose provider/model/session analytics.
- `codexconfig` and `opencodeconfig` already show the desired live-config
  management pattern: discover, plan, diff, checksum, backup, atomic write.

The `cc-switch` exploration confirms the product direction:

1. Keep Umbragate's own provider/control-plane state.
2. Write selected gateway settings back into each agent's live config file.
3. Preserve unrelated agent settings.
4. Use checksum-confirmed plans before writing.
5. Back up live configs before replacement.

## Confirmed Product Decisions

1. Add an agent-aware route format:

   ```text
   /a/{agent}/{provider}/...
   ```

2. Do not manage internal providers for closed agents such as Codex or Claude
   Code. For those agents, only manage whether their externally configurable
   base URL points at Umbragate.

3. Make OpenCode the first-class fine-grained provider management target.
   OpenCode can expose per-provider gateway enable/disable controls.

4. Project attribution is optional and explicit. First version should support
   empty project values plus user-supplied project labels or headers; it should
   not promise automatic project discovery.

5. Reshape the dashboard around four primary areas:

   ```text
   Agents / Providers / Analytics / Sessions
   ```

## Product Model

### Agent

An agent is a client application that can send model traffic through Umbragate.

Initial agents:

- `codex`
- `claude-code`
- `opencode`
- `unknown` for legacy traffic or unmanaged clients

Agent responsibilities:

- Discover live config files.
- Report whether the agent currently points at Umbragate.
- Generate a preview plan for enabling/disabling gateway routing.
- Apply a confirmed config update.
- Preserve unrelated config fields.

### Provider

A provider is an upstream routing target in Umbragate's `config.yaml`.

Provider responsibilities:

- Store provider id, protocol type, base URL, and optional API key reference.
- Decide whether Umbragate injects credentials or forwards client credentials.
- Feed proxy routing and analytics attribution.

### Agent Provider Binding

A binding describes how an agent uses a provider.

Examples:

```text
codex -> openai -> gateway enabled
claude-code -> anthropic -> gateway enabled
opencode -> openrouter -> gateway disabled
opencode -> deepseek -> gateway enabled
```

For Codex and Claude Code, the binding is effectively the active gateway target.
For OpenCode, bindings can be per provider.

### Usage Event

Every proxied request should be attributed to:

- Agent
- Provider
- Model
- Session
- Time
- Optional project
- Protocol and endpoint
- Stream/non-stream mode
- Token counts
- Latency
- Status and failure category

## Routing Design

### Backward Compatibility

Keep the current provider-only route:

```text
/{provider}/...
```

Traffic through this route is recorded with:

```text
agent_id = "unknown"
provider_id = first path segment
```

### New Agent-Aware Route

Add:

```text
/a/{agent}/{provider}/...
```

Examples:

```text
/a/codex/openai/v1/responses
/a/claude-code/anthropic/v1/messages
/a/opencode/openrouter/v1/chat/completions
```

Routing result:

```text
agent_id = "codex"
provider_id = "openai"
remaining_path = "v1/responses"
```

The upstream URL joining rule remains unchanged: Umbragate appends
`remaining_path` to the provider's configured `base_url`. It should not perform
protocol conversion in this redesign.

### Recommended Agent Gateway URLs

Codex:

```text
http://127.0.0.1:4141/a/codex/openai/v1
```

Claude Code:

```text
http://127.0.0.1:4141/a/claude-code/anthropic
```

OpenCode provider `{id}`:

```text
http://127.0.0.1:4141/a/opencode/{id}
```

## Agent Management Architecture

Create a dedicated agent management layer instead of continuing to embed
agent-specific behavior in dashboard handlers.

Proposed package layout:

```text
agents/
  types.go
  registry.go
  codex/
    manager.go
    manager_test.go
  claude/
    manager.go
    manager_test.go
  opencode/
    manager.go
    manager_test.go
```

Existing `codexconfig` and `opencodeconfig` can either be moved into this shape
or wrapped by adapters first. Wrapping is safer for the initial migration.

### Interface

```go
type Manager interface {
    ID() string
    DisplayName() string
    Discover() ([]ConfigFile, error)
    Status(ctx Context) (*Status, error)
    Plan(ctx Context, input BindingInput) (*Plan, error)
    Apply(ctx Context, input BindingInput, baseChecksum string) error
}
```

Shared types:

```go
type ConfigFile struct {
    Path     string `json:"path"`
    Label    string `json:"label"`
    Exists   bool   `json:"exists"`
    Selected bool   `json:"selected"`
}

type Status struct {
    AgentID        string          `json:"agent_id"`
    DisplayName    string          `json:"display_name"`
    ConfigFiles    []ConfigFile    `json:"config_files"`
    Bindings       []BindingStatus `json:"bindings"`
    GatewayCapable bool            `json:"gateway_capable"`
    FineGrained    bool            `json:"fine_grained"`
}

type BindingStatus struct {
    ProviderID      string `json:"provider_id"`
    Configured      bool   `json:"configured"`
    Active          bool   `json:"active"`
    GatewayEnabled  bool   `json:"gateway_enabled"`
    GatewayBaseURL  string `json:"gateway_base_url"`
    LiveBaseURL     string `json:"live_base_url"`
    ConfigPath      string `json:"config_path"`
}

type BindingInput struct {
    ProviderID      string `json:"provider_id"`
    Enabled         bool   `json:"enabled"`
    GatewayBaseURL  string `json:"gateway_base_url"`
    ConfigPath      string `json:"config_path"`
    ProjectID       string `json:"project_id,omitempty"`
}

type Plan struct {
    BaseChecksum string `json:"base_checksum"`
    Diff         string `json:"diff"`
    Current      string `json:"current"`
    ProposedText string `json:"proposed_text"`
}
```

### Codex Manager

Scope:

- Discover `$CODEX_HOME/config.toml`, otherwise `~/.codex/config.toml`.
- Manage only the selected model provider entry that points to Umbragate.
- Do not discover or manage Codex's internal provider ecosystem.
- Preserve unrelated TOML content as much as the current manager supports.
- Continue checksum, diff, backup, and atomic-write behavior.

Gateway enable should write a provider entry like:

```toml
model_provider = "openai"

[model_providers.openai]
name = "Umbragate OpenAI"
base_url = "http://127.0.0.1:4141/a/codex/openai/v1"
env_key = "OPENAI_API_KEY"
wire_api = "responses"
```

### Claude Code Manager

Scope:

- Discover `~/.claude/settings.json`.
- Manage only `env.ANTHROPIC_BASE_URL` and a compatible auth placeholder/key.
- Preserve unrelated Claude settings such as permissions, hooks, MCP, and model
  preferences.
- Use diff, checksum, backup, and atomic writes.

Gateway enable should write a shape like:

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "http://127.0.0.1:4141/a/claude-code/anthropic",
    "ANTHROPIC_AUTH_TOKEN": "PROXY_MANAGED"
  }
}
```

### OpenCode Manager

Scope:

- Discover existing OpenCode config candidates.
- Preserve unrelated JSON/JSONC fields.
- Manage `provider.{id}.options.baseURL` per provider.
- Support per-provider gateway enable/disable.
- Keep existing provider listing and config planning behavior.

Gateway enable for provider `{id}` should set:

```text
provider.{id}.options.baseURL = "http://127.0.0.1:4141/a/opencode/{id}"
```

Gateway disable should remove the base URL only when it exactly matches the
Umbragate-managed URL. User-authored non-Umbragate URLs should be preserved.

## Provider Management Design

Provider management remains centered on `config.yaml`.

Provider fields:

- `id`
- `type`: `openai`, `anthropic`, or empty passthrough
- `base_url`
- `api_key` or env reference

Important behavior:

- `type=openai` and `type=anthropic` mean Umbragate may parse protocol-specific
  payloads and inject configured credentials.
- Empty type should remain passthrough and preserve client credentials.
- Codex should typically use passthrough when Codex owns `env_key`.
- Provider edits should continue to write `config.yaml` with backups.

## Analytics Design

### Dimensions

First-class dimensions:

- Agent
- Provider
- Model
- Time
- Session
- Project
- Endpoint
- Protocol
- Status/failure category
- Stream mode

Project should be explicit and optional. Accepted sources:

- `X-Umbra-Project` request header
- Agent binding configuration
- Future project-path mapping

When no project is supplied, store an empty string or `unknown`.

### Metrics

Core metrics:

- Requests
- Sessions
- Prompt tokens
- Completion tokens
- Total tokens
- Average latency
- Median latency
- P95 latency
- P99 latency
- Success rate
- Error rate
- Failure category counts
- Tokens per request
- Completion-to-prompt ratio

Future metrics:

- Estimated cost by provider/model
- Slow request threshold counts
- Week-over-week growth
- Provider recommendation score

## Database Design

Keep existing tables and add attribution fields through migrations.

### New Tables

```sql
CREATE TABLE IF NOT EXISTS agents (
    id TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    kind TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS agent_provider_bindings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id TEXT NOT NULL,
    provider_id INTEGER NOT NULL REFERENCES providers(id),
    enabled INTEGER NOT NULL DEFAULT 1,
    gateway_enabled INTEGER NOT NULL DEFAULT 0,
    config_path TEXT NOT NULL DEFAULT '',
    project_id TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(agent_id, provider_id, config_path)
);
```

### Session Additions

```sql
ALTER TABLE sessions ADD COLUMN agent_id TEXT NOT NULL DEFAULT 'unknown';
ALTER TABLE sessions ADD COLUMN project_id TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN endpoint TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN protocol TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN stream INTEGER NOT NULL DEFAULT 0;
```

### Request Additions

```sql
ALTER TABLE requests ADD COLUMN response_status INTEGER NOT NULL DEFAULT 0;
ALTER TABLE requests ADD COLUMN endpoint TEXT NOT NULL DEFAULT '';
```

### Indexes

```sql
CREATE INDEX IF NOT EXISTS idx_sessions_agent_started
    ON sessions(agent_id, started_at);

CREATE INDEX IF NOT EXISTS idx_sessions_provider_model_started
    ON sessions(provider_id, model, started_at);

CREATE INDEX IF NOT EXISTS idx_sessions_project_started
    ON sessions(project_id, started_at);

CREATE INDEX IF NOT EXISTS idx_sessions_endpoint_started
    ON sessions(endpoint, started_at);
```

## API Design

### Agent APIs

```text
GET  /api/agents
GET  /api/agents/{agent}/status
POST /api/agents/{agent}/plan
POST /api/agents/{agent}/apply
```

`/plan` returns a masked diff and checksum. `/apply` requires the checksum.

### Provider APIs

Keep the existing provider management surface:

```text
GET    /api/gateway/providers
POST   /api/gateway/providers
PUT    /api/gateway/providers/{id}
DELETE /api/gateway/providers/{id}
```

### Analytics APIs

Add a more flexible analytics API without immediately removing existing
endpoints:

```text
GET /api/analytics/overview?range=7d
GET /api/analytics/timeseries?range=30d&by=agent
GET /api/analytics/breakdown?range=7d&dimension=provider
GET /api/analytics/latency?range=7d&by=model
GET /api/analytics/failures?range=7d
GET /api/analytics/sessions?agent=codex&provider=openai&model=gpt-4.1
GET /api/analytics/sessions/{id}
```

Existing APIs such as `/api/providers`, `/api/models`, `/api/timeseries`, and
`/api/sessions` should remain during migration.

## Dashboard Design

### Agents

Purpose: show and manage agent connection state.

Rows:

- Agent
- Config path
- Gateway enabled
- Active provider
- Last request
- Requests in selected range
- Error rate
- Action buttons

Agent detail should show config status and binding rows.

### Providers

Purpose: manage upstream provider definitions and see which agents use them.

Rows:

- Provider id
- Type
- Base URL
- API key source/availability
- Used by agents
- Requests
- Tokens
- Error rate
- P95 latency
- Actions

OpenCode per-provider gateway toggles can appear as an agent-specific panel or
binding matrix.

### Analytics

Purpose: answer operational questions across dimensions.

Views:

- Overview
- By agent
- By provider
- By model
- By project
- By endpoint
- Failures
- Latency

### Sessions

Purpose: inspect request-level detail.

Filters:

- Range
- Agent
- Provider
- Model
- Project
- Status

Session detail should include agent, provider, model, project, endpoint,
protocol, stream mode, request log, token counts, latency, and error.

## Implementation Phases

### Phase 1: Attribution Foundation

Goal: collect agent-aware data without breaking existing behavior.

Tasks:

1. Add `/a/{agent}/{provider}/...` route parsing.
2. Preserve `/{provider}/...` as legacy route with `agent_id="unknown"`.
3. Add session/request attribution columns and indexes.
4. Thread agent, endpoint, protocol, stream, and project through proxy logging.
5. Add tests for legacy and agent-aware route parsing.

### Phase 2: Agent Manager Layer

Goal: isolate agent-specific config logic.

Tasks:

1. Add `agents` package with shared types and registry.
2. Wrap existing Codex config manager.
3. Wrap existing OpenCode config manager.
4. Add Claude Code manager.
5. Add agent status, plan, and apply APIs.
6. Keep checksum, diff, backup, and atomic write behavior for all managers.

### Phase 3: Dashboard Reshape

Goal: expose the new product model.

Tasks:

1. Add `Agents` dashboard page.
2. Split provider configuration from provider analytics.
3. Add analytics pages or tabs by dimension.
4. Update sessions table and detail view with attribution fields.
5. Keep existing pages available until the new navigation is complete.

### Phase 4: Analytics Expansion

Goal: make the collected dimensions useful.

Tasks:

1. Add generic breakdown query by dimension.
2. Add grouped time series by agent/provider/model/project.
3. Add latency percentile calculations by dimension.
4. Add failure categorization across agent/provider/model/endpoint.
5. Add optional pricing catalog for estimated cost.

## Non-Goals

- No protocol conversion between OpenAI and Anthropic in this redesign.
- No automatic project discovery guarantee.
- No internal provider management for closed agents.
- No destructive overwrites of unrelated agent config fields.
- No cloud sync or telemetry.

## Open Questions

1. Should disabled gateway writes restore the previous live config value from a
   backup, or only remove Umbragate-managed values when safe?
2. Should the agent-aware route use `a` permanently, or should it be aliased to
   a more explicit path such as `/agent/{agent}/{provider}/...`?
3. Should project labels live only in request/session records, or also in
   `agent_provider_bindings` as a default?
4. Should Analytics be one page with dimension controls, or several focused
   pages under the same navigation item?

## Recommended Defaults

1. Start with safe removal of Umbragate-managed values; add backup restore only
   after the first version is stable.
2. Use `/a/{agent}/{provider}/...` as the canonical route and keep it documented.
3. Store project defaults on bindings and allow `X-Umbra-Project` to override
   per request.
4. Build Analytics as one page with tabs or segmented controls first.
