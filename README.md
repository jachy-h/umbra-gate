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
- **Protocol-aware routing.** One OpenAI Link accepts both `/v1/chat/completions` and `/v1/responses`. Saving or testing a Link actively probes every node, records its supported formats, and exposes only the formats shared by the full chain. Anthropic Messages remains native.
- **Web console included.** A React SPA ships inside the binary. Manage links, configure chains, browse stats — all from the browser. The console automatically detects each node's protocol and API-format capabilities; no protocol-style selector is required.

## Getting Started

```bash
brew tap jachy-h/umbragate && brew trust --tap jachy-h/umbragate && brew install umbragate
umbragate start
```

Or build from source: `make && ./umbragate` (requires Go + Node.js).

1. Open **http://localhost:8787** — the built-in web console.
2. DeepSeek, OpenCode, and OpenCode Go are preconfigured. Add API keys or create any additional providers you need.
3. Create a proxy link, stack providers in priority order, set fallback rules, then save. UmbraGate probes Chat Completions and Responses for every OpenAI node and shows the formats the entire chain supports.
4. Copy the link's URL and paste it into your favorite AI client — OpenCode, Cursor, ChatGPT client, or any OpenAI-compatible tool.

For an OpenAI Link, call `/v1/chat/completions` or `/v1/responses` only when that format appears in its automatic capability-check result. Anthropic-native nodes expose `/v1/messages`.

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
umbragate version # or: umbragate -v
```

`start` runs in the background; `run` runs in the foreground. `version` (or `-v`) prints the installed version. After a background start, `start` and `status` display the Web UI URL so it can be opened directly from the terminal. Running `start` again while UmbraGate is already running displays the same status instead of failing. Both modes use `~/.umbragate/config.yaml` by default. Pass a custom configuration with `umbragate start -config /path/to/config.yaml`, `umbragate restart -config /path/to/config.yaml`, or `umbragate run -config /path/to/config.yaml`. Runtime files are stored in `~/.umbragate/`: `umbragate.pid` records the background process, `umbragate.url` records its Web UI URL, and `umbragate.log` contains its output. Running `umbragate` without a command is equivalent to `umbragate run`.

## Release verification

Every release is validated before publication: CI builds the React frontend, verifies that it is embedded, compiles the Go binary, and runs the Go test suite. Release archives contain this fully self-contained binary plus `config.yaml` for Apple Silicon and Intel Macs.

---

[Admin API reference](https://github.com/jachy-h/umbragate) &nbsp;|&nbsp; [中文](./README_zh.md)
