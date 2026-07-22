[中文](./README_zh.md)

# UmbraGate

> One gate. Every LLM. Zero friction.

**UmbraGate** is a production-ready LLM gateway in a single binary. One command and you have a unified OpenAI-compatible endpoint backed by any provider — with smart failover, built-in analytics, and a full web console. No Docker. No database setup. No nonsense.

```bash
brew tap jachy-h/umbragate
brew trust --tap jachy-h/umbragate
brew install umbragate
umbragate
# Open http://localhost:8787 — you're done.
```

## Why UmbraGate

- **Single binary, zero deps.** API gateway, admin UI, and SQLite — all embedded. Just run it.
- **Chain & failover.** Stack providers by priority. Each gets retry counts, status-code rules, error matching, timeout policies, and fallback model overrides. One fails, the next fires.
- **Attribute-driven analytics.** Tag links and requests with `key:value` attributes. Stats auto-aggregate hourly by link × provider × attribute — cost allocation and usage tracking built in.
- **Provider freedom.** Native support for OpenAI, Anthropic, Gemini, DeepSeek, Qwen. Any OpenAI-compatible API works as a custom provider. Hot-reload — no restarts.
- **Web console included.** A React SPA ships inside the binary. Manage links, configure chains, browse stats — all from the browser.

## 30-Second Demo

```bash
# 1. Add providers
curl -X POST localhost:8787/admin/providers -H 'Content-Type: application/json' -d '{
  "name":"openai","type":"openai","base_url":"https://api.openai.com",
  "api_key":"sk-...","models":["gpt-4o-mini"],"enabled":true}'

curl -X POST localhost:8787/admin/providers -H 'Content-Type: application/json' -d '{
  "name":"deepseek","type":"deepseek","base_url":"https://api.deepseek.com",
  "api_key":"sk-...","models":["deepseek-chat"],"enabled":true}'

# 2. Create a link with fallback chain
curl -X POST localhost:8787/admin/links -H 'Content-Type: application/json' -d '{
  "name":"my-gateway","attributes":{"team":"core"},
  "chain":[
    {"provider_id":"<openai-id>","retry_count":1,"rules":{"on_status_codes":[429,500,503]}},
    {"provider_id":"<deepseek-id>","fallback_model":"deepseek-chat"}
  ]}'

# 3. Use it — standard OpenAI SDK / curl compatible
curl -X POST localhost:8787/llm-gateway-lite/<path>/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}'
```

## Install

```bash
brew tap jachy-h/umbragate && brew trust --tap jachy-h/umbragate && brew install umbragate   # macOS / Linux
# or: make && ./umbragate                             # build from source (Go + Node.js required)
```

All data lives under `~/.umbragate/` — config, DB, everything. Migrate or reset by moving that directory.

---

[Admin API reference](https://github.com/jachy-h/umbragate) &nbsp;|&nbsp; [中文](./README_zh.md)
