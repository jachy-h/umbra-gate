# opencode Provider Management Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Refine provider management into a safer four-step flow that supports all opencode global config filenames and does not manage models.

**Architecture:** Extend `opencodeconfig.Manager` with config discovery, selected-path requests, and JSONC parsing. Simplify provider writes to credentials and optional provider metadata only. Update dashboard endpoints and template to drive config-file selection before diff/apply.

**Tech Stack:** Go standard library HTTP/templates/filesystem/JSON, existing embedded dashboard templates, existing Go tests. No new dependencies.

---

### Task 1: Update opencode config manager behavior

**Files:**
- Modify: `opencodeconfig/manager.go`
- Modify: `opencodeconfig/manager_test.go`

**Step 1: Write failing tests**

Add tests for:

- `Discover` returns `opencode.json`, `opencode.jsonc`, `.opencode/opencode.json`, `.opencode/opencode.jsonc` when present.
- `Discover` returns default `opencode.json` as creatable when none exist.
- `Load` parses JSONC comments and trailing commas.
- `Plan` does not write `models`, `model`, or `small_model` even if request includes model fields.

**Step 2: Run tests to verify failure**

Run: `go test ./opencodeconfig`

Expected: FAIL for missing discovery/JSONC/no-model behavior.

**Step 3: Implement manager changes**

Add:

- `type ConfigFile struct { Path string; Label string; Exists bool; Selected bool }`
- `func Discover(baseDir string) []ConfigFile`
- `func DefaultDir() string`

Change parsing to strip JSONC comments and trailing commas before `json.Unmarshal`.

Change apply logic to ignore `Models`, `DefaultModel`, and `SmallModel`. Keep struct fields for request compatibility, but do not write them.

**Step 4: Run tests**

Run: `go test ./opencodeconfig`

Expected: PASS.

---

### Task 2: Update dashboard endpoints for selected config files

**Files:**
- Modify: `dashboard/handler.go`
- Modify: `dashboard/handler_test.go`

**Step 1: Write failing endpoint tests**

Add/update tests for:

- `GET /dashboard/providers/config` returns `files` and selected config.
- `POST /dashboard/providers/diff` accepts `path` and writes no models.
- `POST /dashboard/providers/apply` accepts `path` and checksum.

**Step 2: Run tests to verify failure**

Run: `go test ./dashboard`

Expected: FAIL until endpoints support `path` and files.

**Step 3: Implement endpoint changes**

Add request wrappers:

- `providerPlanRequest { Path string; opencodeconfig.ProviderInput }`
- `providerApplyRequest { Path string; BaseChecksum string; opencodeconfig.ProviderInput }`

Use `opencodeconfig.Manager{Path: requestedPath}` for diff/apply. For config GET, return discovered files and selected masked config.

Reject empty or unsupported paths except the discovered default creatable path.

**Step 4: Run tests**

Run: `go test ./dashboard`

Expected: PASS.

---

### Task 3: Redesign provider page interaction

**Files:**
- Modify: `dashboard/templates/providers.html`
- Modify: `dashboard/handler_test.go`

**Step 1: Write failing render test**

Update page test to assert:

- `Step 1`
- `configFileSelect`
- `Step 2`
- `Provider ID`
- `Step 3`
- `API Key`
- no `Models`, `Default Model`, or `Small Model`

**Step 2: Run test to verify failure**

Run: `go test ./dashboard`

Expected: FAIL until template changes.

**Step 3: Implement template changes**

Replace single form with four cards/steps:

1. Config file select populated from `/dashboard/providers/config`.
2. Provider select/input.
3. API key plus optional advanced metadata: name, api, baseURL.
4. Diff preview and apply.

Remove model fields from HTML and JavaScript payload.

**Step 4: Run tests**

Run: `go test ./dashboard`

Expected: PASS.

---

### Task 4: Final verification

**Files:**
- All changed files

**Step 1: Run full tests**

Run: `go test ./...`

Expected: PASS.

**Step 2: Build**

Run: `go build ./...`

Expected: PASS.

**Step 3: Inspect status**

Run: `git status --short`

Expected: only intended files changed.
