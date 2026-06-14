# opencode Active Providers Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Show providers opencode can use on the provider management page.

**Architecture:** Extend the dashboard provider config response with a merged provider status list built from the built-in catalog and the selected opencode config's provider map. Render that list in the existing provider page with status columns for built-in, configured, and API key presence.

**Tech Stack:** Go standard library, existing dashboard templates and tests.

---

### Task 1: Provider status API

**Files:**
- Modify: `dashboard/handler.go`
- Modify: `dashboard/handler_test.go`

**Step 1: Write failing test**

Update `TestProviderConfigEndpointReturnsCatalogAndMaskedConfig` to assert the JSON response includes `providers`, configured provider ids, built-in provider ids, and status booleans.

**Step 2: Run test**

Run: `go test ./dashboard`

Expected: FAIL until API includes provider statuses.

**Step 3: Implement provider status response**

Add a provider status struct and a helper that merges `providerCatalog()` with `config.provider`. Include fields: `id`, `name`, `built_in`, `configured`, `has_api_key`.

**Step 4: Run test**

Run: `go test ./dashboard`

Expected: PASS.

### Task 2: Provider status UI

**Files:**
- Modify: `dashboard/templates/providers.html`
- Modify: `dashboard/handler_test.go`

**Step 1: Write failing render test**

Update `TestProvidersPageRendersManagementUI` to assert the page contains `opencode Providers` and renders status labels.

**Step 2: Run test**

Run: `go test ./dashboard`

Expected: FAIL until template changes.

**Step 3: Implement UI**

Render `data.providers` as the main provider list, showing provider id, name, built-in/configured/API key status.

**Step 4: Verify**

Run:

```bash
go test ./...
go build ./...
```

Expected: PASS.
