# Personal AI Router

Local-first LLM Gateway. Transparent proxy between AI clients and LLM providers, with usage statistics and a built-in dashboard.

## Quick Start

```bash
# 1. Build
go build -o personal-ai-router .

# 2. Edit config.yaml — see below
# 3. Run
export OPENAI_API_KEY=sk-xxxxx
./personal-ai-router
```

## Configuration

`config.yaml`:

```yaml
listen: "127.0.0.1:4141"

providers:
  openai:
    type: openai                       # openai | anthropic
    base_url: https://api.openai.com   # client request path is appended as-is
    api_key: ${OPENAI_API_KEY}         # literal string or ${ENV_VAR}

  anthropic:
    type: anthropic
    base_url: https://api.anthropic.com
    api_key: ${ANTHROPIC_API_KEY}

storage:
  save_prompt: false
```

| Field | Required | Description |
|-------|----------|-------------|
| `listen` | no (default `127.0.0.1:4141`) | Listen address |
| `providers.<id>.type` | yes | `openai` or `anthropic` |
| `providers.<id>.base_url` | yes | Upstream base URL. The client request path is appended verbatim (e.g. a client requesting `/openai/v1/chat/completions` against `base_url: https://api.openai.com` proxies to `https://api.openai.com/v1/chat/completions`). |
| `providers.<id>.api_key` | yes | Literal key, or `${ENV_VAR}` to read from the environment. Missing env vars cause startup to fail. |
| `storage.save_prompt` | no | Persist prompt/response content to the local DB |

### How `base_url` works

The router does **not** rewrite paths. Whatever the client sends after `/<provider-id>/` is appended to the configured `base_url`. So:

| Client SDK | What it sends | Set `base_url` to |
|------------|---------------|-------------------|
| `@ai-sdk/openai` | `/v1/chat/completions` | `https://api.openai.com` |
| `@ai-sdk/openai-compatible` | `/chat/completions` | `https://upstream.example.com/v1` (if upstream needs `/v1`) |
| `@ai-sdk/anthropic` | `/v1/messages` | `https://api.anthropic.com` |

### Authentication

- `type: openai` → gateway injects `Authorization: Bearer <api_key>`
- `type: anthropic` → gateway injects `x-api-key: <api_key>` plus a default `anthropic-version` (only if the client did not provide one)

Client-supplied `Authorization` / `x-api-key` headers are stripped before forwarding. Other headers (e.g. `anthropic-beta`, `OpenAI-Organization`) pass through.

## Dashboard

Open http://127.0.0.1:4141/dashboard.

- **Sessions / Models / Providers** — usage statistics
- **Providers** — manage your `opencode.json` (toggle "Use Gateway")
- **Gateway** — manage `config.yaml` providers (add / edit / delete). Changes take effect immediately, no restart required. The previous file is backed up as `config.yaml.<timestamp>.bak`.

API keys submitted via the dashboard are stored as the literal string you provide. To keep secrets out of the file, type `${YOUR_ENV_VAR}` instead of the raw key.

## OpenCode Configuration

Point opencode at the gateway:

```jsonc
{
  "provider": {
    "openai": {
      "npm": "@ai-sdk/openai-compatible",
      "models": { "gpt-4o": {} },
      "options": {
        "baseURL": "http://127.0.0.1:4141/openai",
        "apiKey": "local"
      }
    },
    "anthropic": {
      "npm": "@ai-sdk/anthropic",
      "models": { "claude-sonnet-4-20250514": {} },
      "options": {
        "baseURL": "http://127.0.0.1:4141/anthropic",
        "apiKey": "local"
      }
    }
  }
}
```

The gateway substitutes the `apiKey` placeholder with the real provider key from `config.yaml`.

## Data

All data is stored locally in `router.db` (SQLite). No cloud dependencies. No telemetry.

## Supported Protocols

- OpenAI Chat Completions (`/v1/chat/completions`)
- Anthropic Messages (`/v1/messages`)
- SSE streaming
