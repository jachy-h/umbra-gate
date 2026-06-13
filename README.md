# Personal AI Router

Local-first LLM Gateway. Transparent proxy between AI clients and LLM providers, with usage statistics and dashboard.

## Quick Start

```bash
# 1. Download or build
go build -o personal-ai-router .

# 2. Edit config.yaml — set your API keys
providers:
  openai:
    base_url: https://api.openai.com
    api_key: sk-xxxxx
    protocol: openai-compatible
  anthropic:
    base_url: https://api.anthropic.com
    api_key: sk-ant-xxxxx
    protocol: anthropic

# 3. Run
./personal-ai-router
```

## OpenCode Configuration

Add providers pointing to the router:

```jsonc
{
  "provider": {
    "openai": {
      "npm": "@ai-sdk/openai-compatible",
      "models": { "gpt-4o": {} },
      "options": { "baseURL": "http://127.0.0.1:4141/openai" }
    },
    "anthropic": {
      "npm": "@ai-sdk/anthropic",
      "models": { "claude-sonnet-4-20250514": {} },
      "options": { "baseURL": "http://127.0.0.1:4141/anthropic" }
    }
  }
}
```

Client passes `apiKey: local` — router replaces it with the real key.

## Dashboard

Open http://127.0.0.1:4141/dashboard to view usage statistics.

## Configuration

| Field | Default | Description |
|-------|---------|-------------|
| `listen` | `127.0.0.1:4141` | Listen address |
| `providers.<name>.base_url` | — | Upstream API endpoint |
| `providers.<name>.api_key` | — | Provider API key |
| `providers.<name>.protocol` | — | `openai-compatible` or `anthropic` |
| `storage.save_prompt` | `false` | Save prompt/response content |

## Data

All data stored locally in `router.db` (SQLite). No cloud dependencies. No telemetry.

## Supported Protocols

- OpenAI Chat Completions (`/v1/chat/completions`)
- Anthropic Messages (`/v1/messages`)
- SSE streaming
