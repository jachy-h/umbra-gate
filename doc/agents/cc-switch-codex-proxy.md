# cc-switch Codex Proxy Evidence

Date: 2026-06-26

## Conclusion

`cc-switch` proxies Codex by taking over Codex's live configuration, not by
hooking the Codex process. The takeover flow starts a local HTTP proxy, backs up
the current live files, rewrites Codex `config.toml` so the active provider's
`base_url` points at `http://127.0.0.1:<port>/v1`, and writes proxy placeholder
credentials such as `PROXY_MANAGED`.

This differs from Umbragate's provider-addressed route design. cc-switch's
actual Codex live base URL is the local proxy origin plus `/v1`, while
Umbragate uses `/a/codex/openai/v1` so the request path still carries both agent
and provider identity.

## GitHub Evidence

- `src-tauri/src/codex_config.rs`
  - `CC_SWITCH_CODEX_MODEL_PROVIDER_ID` is `"custom"`.
  - Codex live files are `~/.codex/config.toml` and `~/.codex/auth.json`.
  - Built-in provider IDs such as `openai` are treated as reserved.
  - Source: https://github.com/farion1231/cc-switch/blob/main/src-tauri/src/codex_config.rs

- `src-tauri/src/services/proxy.rs`
  - `start_with_takeover` backs up live config, syncs live tokens to the DB, and
    calls `takeover_live_configs`.
  - `build_proxy_urls` returns the Codex base URL as
    `http://<loopback-host>:<port>/v1`.
  - `takeover_live_configs` updates Codex `auth.OPENAI_API_KEY` to
    `PROXY_MANAGED`, updates Codex `config.toml` `base_url`, enforces
    `wire_api = "responses"`, then writes the Codex live files.
  - Source: https://github.com/farion1231/cc-switch/blob/main/src-tauri/src/services/proxy.rs

- `src-tauri/src/proxy/server.rs`
  - The local proxy exposes Codex/OpenAI-compatible routes including
    `/v1/responses`, `/v1/chat/completions`, `/v1/models`, and `/codex/v1/...`.
  - Source: https://github.com/farion1231/cc-switch/blob/main/src-tauri/src/proxy/server.rs

## Umbragate Implication

Umbragate Codex gateway mode should write:

```toml
model_provider = "custom"

[model_providers.custom]
name = "Umbragate"
base_url = "http://127.0.0.1:4141/a/codex/openai/v1"
wire_api = "responses"
requires_openai_auth = true
```

The proxy should recognize Umbragate Codex requests such as
`/a/codex/openai/v1/responses` as `agent_id=codex`, route them to the OpenAI
provider, and forward them to the configured OpenAI upstream. The cc-switch
local shape `/v1/responses` remains useful as a compatibility route, but it
does not encode provider identity. For managed Codex routes, Umbragate should
forward Codex's own `Authorization` header unchanged; it should not depend on
provider `api_key` values in Umbragate `config.yaml`.
