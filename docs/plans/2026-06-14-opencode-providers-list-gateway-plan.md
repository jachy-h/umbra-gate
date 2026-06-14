# opencode Providers List Gateway Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Use `opencode providers list` as the provider list source and add per-provider gateway toggles.

**Architecture:** Replace hardcoded provider catalog with a command-backed provider source. Extend provider config planning to support gateway enable/disable by setting or removing `provider.<id>.options.baseURL`.

**Tech Stack:** Go standard library, existing dashboard templates and tests.

---

### Task 1: Command-backed provider list

- Add provider list command execution in dashboard.
- Parse JSON output first; if unavailable, parse plain line/table output.
- Remove built-in catalog fallback.
- Test response contains providers from fake command output only.

### Task 2: Gateway toggle config changes

- Extend provider input with gateway fields.
- Enable writes `options.baseURL = <gateway-base>/<provider>`.
- Disable removes baseURL only when it points to this gateway.
- Test diff/apply behavior.

### Task 3: UI toggle

- Add Use Gateway column with switch buttons.
- Switch previews diff and requires Apply Confirmed Change.
- Remove hardcoded provider select options; populate from command providers.

### Task 4: Verification

Run `go test ./...` and `go build ./...`.
