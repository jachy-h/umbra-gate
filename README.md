# Umbragate

Local-first LLM gateway. It sits between AI clients and model providers, records usage locally, and exposes a built-in dashboard.

## Highlights

- Single Go binary
- Local SQLite storage only, no telemetry
- OpenAI-compatible and Anthropic proxying
- Dashboard for sessions, models, providers, failures, and trend analytics
- OpenCode provider integration and gateway toggling

## Quick Start

```bash
git clone git@github.com:jachy-h/umbra-gate.git
cd umbragate
cp config.example.yaml config.yaml

export OPENAI_API_KEY=sk-xxxxx
go build -o umbragate .
./umbragate
```

Then open `http://127.0.0.1:4141/dashboard`.

## Install With Homebrew

For macOS users:

```bash
brew tap jachy-h/umbragate
brew trust --tap jachy-h/umbragate
brew install umbragate
umbragate
cp ~/.umbragate/config.example.yaml ~/.umbragate/config.yaml
```

The supported Homebrew path is the tagged release build.

## Requirements

- Go 1.24+
- macOS, Linux, or another platform supported by Go

## Configuration

Start from `config.example.yaml` and write your local settings to `config.yaml`.

The runtime looks for `config.yaml` in this order:

1. `UMBRAGATE_HOME/config.yaml` when `UMBRAGATE_HOME` is set
2. `./config.yaml` in the current working directory
3. default: `~/.umbragate/config.yaml`

On startup, Umbragate automatically creates `~/.umbragate/` when needed and writes `~/.umbragate/config.example.yaml` if it does not already exist.

Example:

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

`config.yaml` is intentionally ignored by git so local secrets are not committed.

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

- **Dashboard** - overview metrics and time-series usage trends
- **Sessions / Models / Providers** - usage statistics
- **Failures** - recent failure breakdowns and error visibility
- **Providers** - manage your `opencode.json` and toggle gateway forwarding
- **Gateway config sync** - provider changes are written to `config.yaml` immediately, with backup files like `config.yaml.<timestamp>.bak`

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

The runtime stores both `config.yaml` and `router.db` in the same app directory:

1. `UMBRAGATE_HOME/`
2. current working directory when `./config.yaml` exists
3. default: `~/.umbragate/`

Generated local files such as `router.db`, `tmp/`, and backup configs are ignored by git.

If installed with Homebrew and started normally, the default stats database path is:
`~/.umbragate/router.db`

Default related paths:

- config: `~/.umbragate/config.yaml`
- config example: `~/.umbragate/config.example.yaml`
- database: `~/.umbragate/router.db`
- log: `~/.umbragate/umbragate.log`

Background startup is available with either:

```bash
umbragate -d
umbragate daemon
brew services start umbragate
```

## Build And Test

```bash
go test ./...
go build -o umbragate .
```

## Install Notes

For local development, use `go build`.

Release automation publishes macOS tarballs on tags like `v0.1.0`. If the repository secret `HOMEBREW_TAP_TOKEN` is configured, the release workflow also updates the Homebrew formula SHA256 values automatically.

## Supported Protocols

- OpenAI Chat Completions (`/v1/chat/completions`)
- Anthropic Messages (`/v1/messages`)
- SSE streaming
