[中文](./README_zh.md)

# UmbraGate

> One gate. Every LLM. Zero friction.

**UmbraGate** is a production-ready LLM gateway in a single binary. One command and you have a unified OpenAI-compatible endpoint backed by any provider — with smart failover, built-in analytics, and a full web console. No Docker. No database setup. No nonsense.

```bash
brew tap jachy-h/umbragate
brew trust --tap jachy-h/umbragate
brew install umbragate
umbragate start
# Open http://localhost:8787 — you're done.
```

## Why UmbraGate

- **Single binary, zero deps.** API gateway, admin UI, and SQLite — all embedded. Just run it.
- **Chain & failover.** Stack providers by priority. Each gets retry counts, status-code rules, error matching, timeout policies, and fallback model overrides. One fails, the next fires.
- **Attribute-driven analytics.** Tag links with `key:value` attributes. Stats auto-aggregate hourly by link × provider × attribute — cost allocation and usage tracking built in.
- **Two native protocol styles.** Links are explicitly OpenAI Style or Anthropic Style and cannot mix the two. OpenAI Chat Completions and Responses, plus Anthropic Messages, are sent through the vendors' official Go SDKs. Compatible providers can declare multiple endpoint formats and base URLs.
- **Web console included.** A React SPA ships inside the binary. Manage links, configure chains, browse stats — all from the browser.

## Getting Started

```bash
brew tap jachy-h/umbragate && brew trust --tap jachy-h/umbragate && brew install umbragate
umbragate start
```

Or build from source: `make && ./umbragate` (requires Go + Node.js).

1. Open **http://localhost:8787** — the built-in web console.
2. Add your providers (OpenAI, Anthropic, DeepSeek, …) with their API keys.
3. Create a proxy link, stack providers in priority order, set fallback rules.
4. Copy the link's URL and paste it into your favorite AI client — OpenCode, Cursor, ChatGPT client, or any OpenAI-compatible tool.

OpenAI Style links expose `/v1/chat/completions` and `/v1/responses`. Anthropic Style links expose `/v1/messages`.

That's it. Your requests are now automatically routed with failover, logged, and analyzed.

All data lives under `~/.umbragate/` — config, DB, everything. Migrate or reset by moving that directory.

Startup prints the effective configuration path. The first-start configuration file documents every option: request logs are kept for 7 days by default, the database is capped at 1 GiB by pruning the oldest 1,000 request logs, and hourly aggregates are kept for 365 days. Background logs rotate daily or at 50 MiB and retain seven compressed backups.

## Process lifecycle

Run in the background and manage the local process with:

```bash
umbragate start
umbragate status
umbragate restart
umbragate stop
umbragate run
umbragate --help
```

`start` runs in the background; `run` runs in the foreground. Both modes use `~/.umbragate/config.yaml` by default. Pass a custom configuration with `umbragate start -config /path/to/config.yaml`, `umbragate restart -config /path/to/config.yaml`, or `umbragate run -config /path/to/config.yaml`. Runtime files are stored in `~/.umbragate/`: `umbragate.pid` records the background process and `umbragate.log` contains its output. Running `umbragate` without a command is equivalent to `umbragate run`.

---

[Admin API reference](https://github.com/jachy-h/umbragate) &nbsp;|&nbsp; [中文](./README_zh.md)
