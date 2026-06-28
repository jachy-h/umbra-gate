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
URL configuration. Most Umbragate routes use the first path segment as the
provider ID and append the remaining path to that provider's configured upstream
`base_url`. Codex gateway mode follows the cc-switch shape instead:
`/v1/responses`, `/v1/chat/completions`, and related OpenAI-compatible endpoints
are recognized as Codex traffic and routed to the OpenAI provider.

The important distinction:

- Codex CLI can already be pointed at Umbragate by writing
  `~/.codex/config.toml`.
- Claude Code can use the same local-gateway idea, but needs a dedicated config
  writer for `~/.claude/settings.json` if we want first-class dashboard support.

## Codex CLI

Codex is configured through `~/.codex/config.toml`, or
`$CODEX_HOME/config.toml` when `CODEX_HOME` is set.

Following the cc-switch pattern, gateway takeover only rewrites `base_url`
inside the active `[model_providers.<model_provider>]` table. Codex rejects
overriding reserved built-in provider ids (like `openai`) in `model_providers`,
so when the active provider is reserved (or unset) takeover switches routing to
a non-reserved `custom` entry — mirroring cc-switch's
`CC_SWITCH_CODEX_MODEL_PROVIDER_ID = "custom"`. It never injects a
`model_provider`/table for a reserved id, and preserves all unrelated keys
(name, env_key, wire_api, requires_openai_auth, etc.).

Enable writes this diff against an existing config without an active provider:

```toml
# before (official OpenAI provider, no explicit table)
model = "gpt-5.5"

# after (routes through managed custom table)
model = "gpt-5.5"

model_provider = "custom"

[model_providers.custom]
name = "Umbragate"
base_url = "http://127.0.0.1:4141/a/codex/openai/v1"
wire_api = "responses"
requires_openai_auth = true
```

When `model_provider` already points at a non-reserved custom table, only that
table's `base_url` is updated in place:

```toml
model_provider = "custom"

[model_providers.custom]
name = "MyCustom"
base_url = "http://127.0.0.1:4141/a/codex/openai/v1"   # rewritten
env_key = "MY_KEY"
wire_api = "responses"
```

Disable removes the gateway `base_url`. When the managed `custom` table was
created solely by enable (name = "Umbragate", no user auth fields, and
`requires_openai_auth = true`), the whole table and `model_provider` are
removed. User-configured tables keep everything except the gateway-matching
`base_url`.

How the request flows:

1. Codex reads `model_provider` (unchanged by takeover).
2. Codex sends OpenAI Responses API traffic to
   `http://127.0.0.1:4141/a/codex/openai/v1/...`.
3. Umbragate records `agent_id=codex` and routes `openai` to the matching
   `config.yaml` provider.
4. The remaining OpenAI path is appended to the upstream provider `base_url`;
   when the upstream already ends in `/v1`, the local `/v1` proxy prefix is not
   duplicated.
5. Umbragate records the request/session in SQLite and forwards stream or
   non-stream responses back to Codex.

For the managed Codex route, Umbragate does not authenticate upstream requests
from `config.yaml`. Codex uses its own `auth.json` login state to send
`Authorization`, and Umbragate forwards that header unchanged.

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

Codex is already implemented through `codexconfig/manager.go`. The current
approach follows the cc-switch Codex takeover pattern:

- discover `$CODEX_HOME/config.toml` or `~/.codex/config.toml`
- generate a diff before writing
- require the caller's base checksum before applying
- write through a temp file and rename
- create timestamped backups before replacement
- **enable** only rewrites `base_url` in the active provider table; when the
  active provider is a reserved built-in (like `openai`) or unset, it switches
  routing to a managed `custom` entry (cc-switch pattern) — it never overrides a
  reserved id in `model_providers`
- **disable** only removes a gateway-matching `base_url`; the managed `custom`
  table is removed wholesale only if it was created solely by enable, otherwise
  user config is preserved

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

Umbragate does not rewrite protocols. The client-facing base URL should include
only the local gateway prefix required by that client:

| Client | Gateway URL | Typical upstream result |
| --- | --- | --- |
| Codex | `http://127.0.0.1:4141/a/codex/openai/v1` | `https://api.openai.com/v1/...` |
| Claude Code | `http://127.0.0.1:4141/a/claude-code/anthropic` | `https://api.anthropic.com/v1/messages` |

The upstream provider's `base_url` in `config.yaml` should be set so appending
the client's remaining path creates the exact provider endpoint. For Codex,
`https://api.openai.com` is the preferred OpenAI upstream.

## Remaining Work

- Add restore-from-backup behavior for disabling proxy takeover if users want
  exact previous-value restoration instead of safe removal of managed values.
- Consider protocol conversion later, but do not mix that into the initial
  config-management feature. Today Umbragate forwards OpenAI-compatible traffic
  as OpenAI-compatible traffic and Anthropic traffic as Anthropic traffic.
