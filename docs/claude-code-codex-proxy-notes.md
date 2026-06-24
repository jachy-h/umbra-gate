# Claude Code and Codex Proxy Notes

This document records the proxy approach for Claude Code and Codex CLI, based
on the current Umbragate architecture and the implementation pattern observed in
`farion1231/cc-switch`.

## Summary

Umbragate is already a local HTTP gateway:

```text
AI client -> http://127.0.0.1:4141/<provider-id>/... -> Umbragate -> upstream API
```

The client is pointed at Umbragate by changing the client's own provider/base
URL configuration. Umbragate then uses the first path segment as the provider ID
and appends the remaining path to that provider's configured upstream
`base_url`.

The important distinction:

- Codex CLI can already be pointed at Umbragate by writing
  `~/.codex/config.toml`.
- Claude Code can use the same local-gateway idea, but needs a dedicated config
  writer for `~/.claude/settings.json` if we want first-class dashboard support.

## Codex CLI

Codex is configured through `~/.codex/config.toml`, or
`$CODEX_HOME/config.toml` when `CODEX_HOME` is set.

Umbragate's `codexconfig.Manager` writes a model provider entry like this:

```toml
model_provider = "openai"

[model_providers.openai]
name = "Umbragate OpenAI"
base_url = "http://127.0.0.1:4141/a/codex/openai/v1"
env_key = "OPENAI_API_KEY"
wire_api = "responses"
```

How the request flows:

1. Codex reads `model_provider = "openai"`.
2. Codex sends OpenAI Responses API traffic to
   `http://127.0.0.1:4141/a/codex/openai/v1/...`.
3. Umbragate records `agent_id=codex` and routes `openai` to the matching
   `config.yaml` provider.
4. The remaining path, such as `v1/responses`, is appended to the upstream
   provider `base_url`.
5. Umbragate records the request/session in SQLite and forwards stream or
   non-stream responses back to Codex.

Umbragate forwards the client-provided `Authorization` header from Codex's
`env_key` to the upstream API unchanged. Provider `api_key` values in
`config.yaml` are not injected into requests.

## Claude Code

Claude Code reads Anthropic-style settings from `~/.claude/settings.json`.

A direct Umbragate provider entry should write a shape like this:

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "http://127.0.0.1:4141/a/claude-code/anthropic",
    "ANTHROPIC_AUTH_TOKEN": "local"
  }
}
```

How the request flows:

1. Claude Code sends Anthropic Messages API traffic to
   `http://127.0.0.1:4141/a/claude-code/anthropic/v1/messages`.
2. Umbragate records `agent_id=claude-code` and routes `anthropic` to the matching `config.yaml` provider.
3. The remaining path, `v1/messages`, is appended to the upstream provider
   `base_url`.
4. Umbragate forwards Claude Code's headers to the upstream API unchanged.
5. Umbragate records usage and returns the upstream response to Claude Code.

Claude Code usually expects some credential-like value to be present even when
the local gateway owns or forwards auth. A harmless placeholder such as `local`
or `PROXY_MANAGED` is enough for local routing, as long as Umbragate either
passes through the real client auth or injects the configured upstream key.

## cc-switch Pattern

`cc-switch` uses two layers:

1. It keeps provider definitions in its own database.
2. It writes the active provider back into each tool's live config file.

For proxy takeover, it starts a local HTTP proxy and rewrites live configs to
point clients at that proxy:

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "http://127.0.0.1:<port>",
    "ANTHROPIC_AUTH_TOKEN": "PROXY_MANAGED"
  }
}
```

```toml
base_url = "http://127.0.0.1:<port>/v1"
```

It also backs up the original live config before takeover and restores it when
proxy takeover is disabled. That backup/restore pattern is worth copying for
Claude Code support in Umbragate.

## Recommended Umbragate Implementation

Codex is already implemented through `codexconfig/manager.go`. Keep the current
approach:

- discover `$CODEX_HOME/config.toml` or `~/.codex/config.toml`
- generate a diff before writing
- require the caller's base checksum before applying
- write through a temp file and rename
- create timestamped backups before replacement

Claude Code is implemented through `agents/claude.Manager`:

- discover `~/.claude/settings.json`
- parse and preserve unrelated JSON keys
- update only `env.ANTHROPIC_BASE_URL` and the chosen auth placeholder/key
- generate a preview plan and checksum before applying
- write atomically with timestamped backups
- expose status through `/api/agents` and the Agents dashboard

Avoid rewriting unrelated Claude settings such as permissions, hooks, MCP, or
model preferences unless the user explicitly asks for full takeover behavior.

## Provider URL Rules

Umbragate does not rewrite protocol paths. The client-facing base URL should
include only the gateway provider prefix and any client-required version prefix:

| Client | Gateway URL | Typical upstream result |
| --- | --- | --- |
| Codex | `http://127.0.0.1:4141/a/codex/openai/v1` | `https://api.openai.com/v1/...` |
| Claude Code | `http://127.0.0.1:4141/a/claude-code/anthropic` | `https://api.anthropic.com/v1/messages` |

The upstream provider's `base_url` in `config.yaml` should be set so appending
the client's remaining path creates the exact provider endpoint.

## Remaining Work

- Add restore-from-backup behavior for disabling proxy takeover if users want
  exact previous-value restoration instead of safe removal of managed values.
- Consider protocol conversion later, but do not mix that into the initial
  config-management feature. Today Umbragate forwards OpenAI-compatible traffic
  as OpenAI-compatible traffic and Anthropic traffic as Anthropic traffic.
