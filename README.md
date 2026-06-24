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
```

The supported Homebrew path is the tagged release build.

## Requirements

- Go 1.24+
- macOS, Linux, or another platform supported by Go

## Configuration

`config.yaml` is created automatically on first startup.

The runtime looks for `config.yaml` in this order:

1. `UMBRAGATE_HOME/config.yaml` when `UMBRAGATE_HOME` is set
2. `./config.yaml` in the current working directory
3. default: `~/.umbragate/config.yaml`

On startup, Umbragate automatically creates `~/.umbragate/` when needed and writes a default `config.yaml` if it does not already exist.

Example:

```yaml
listen: "127.0.0.1:4141"

providers:
  openai:
    base_url: https://api.openai.com/v1

  github-copilot:
    base_url: https://api.githubcopilot.com
```

| Field | Required | Description |
|-------|----------|-------------|
| `listen` | no (default `127.0.0.1:4141`) | Listen address |
| `providers.<id>.base_url` | yes | Upstream base URL. The client request path is appended verbatim (e.g. a client requesting `/openai/v1/chat/completions` against `base_url: https://api.openai.com` proxies to `https://api.openai.com/v1/chat/completions`). |
| `providers.<id>.api_key` | no | Optional metadata for provider management. The proxy does not inject it into upstream requests. Missing env vars still cause startup to fail when referenced. |
`config.yaml` is now safe to commit when you keep `api_key` empty or use `${ENV_VAR}` references instead of raw secrets.

### How `base_url` works

The router does **not** rewrite paths. Whatever the client sends after `/<provider-id>/` is appended to the configured `base_url`. So:

| Client SDK | What it sends | Set `base_url` to |
|------------|---------------|-------------------|
| `@ai-sdk/openai` | `/v1/chat/completions` | `https://api.openai.com` |
| `@ai-sdk/openai-compatible` | `/chat/completions` | `https://upstream.example.com/v1` (if upstream needs `/v1`) |
| `@ai-sdk/anthropic` | `/v1/messages` | `https://api.anthropic.com` |

### Authentication

The gateway forwards the client's auth headers unchanged. It does not strip,
replace, or inject provider credentials, and it does not add protocol-specific
headers. Keep credentials in the client environment.

## Dashboard

Open http://127.0.0.1:4141/dashboard.

- **Dashboard** - overview metrics and time-series usage trends
- **Agents / Providers / Analytics / Sessions** - gateway status and usage statistics
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
        "baseURL": "http://127.0.0.1:4141/a/opencode/openai",
        "apiKey": "local"
      }
    },
    "anthropic": {
      "npm": "@ai-sdk/anthropic",
      "models": { "claude-sonnet-4-20250514": {} },
      "options": {
        "baseURL": "http://127.0.0.1:4141/a/opencode/anthropic",
        "apiKey": "local"
      }
    }
  }
}
```

The gateway uses `config.yaml` as its provider routing map. Client credentials continue to flow through unchanged by default.

## Codex CLI Configuration

The Providers dashboard can also toggle Codex CLI traffic through Umbragate. It discovers `CODEX_HOME/config.toml` when `CODEX_HOME` is set, otherwise `~/.codex/config.toml`, and writes a Codex model provider such as:

```toml
model_provider = "openai"

[model_providers.openai]
name = "Umbragate OpenAI"
base_url = "http://127.0.0.1:4141/a/codex/openai/v1"
env_key = "OPENAI_API_KEY"
wire_api = "responses"
```

For Codex, the gateway keeps the upstream provider in passthrough mode so the client-provided `Authorization` header from `env_key` reaches the upstream API unchanged. The default OpenAI upstream is `https://api.openai.com`, so Codex requests to `/a/codex/openai/v1/...` are proxied to `https://api.openai.com/v1/...` and recorded in the local dashboard with `agent_id=codex`.

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

## Supported Upstream APIs

- OpenAI Chat Completions (`/v1/chat/completions`)
- Anthropic Messages (`/v1/messages`)
- SSE streaming
