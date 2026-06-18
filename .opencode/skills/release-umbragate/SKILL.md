---
name: release-umbragate
description: Use when publishing Umbragate releases, updating the Homebrew tap, tagging versions, or verifying the release workflow for this repository and its paired tap repo.
---

# Release Umbragate

Use this skill when the task is to publish a new Umbragate release from this repository and keep the Homebrew tap aligned.

## Repositories

- Main repo: `git@github.com:jachy-h/umbra-gate.git`
- Homebrew tap: `git@github.com:jachy-h/homebrew-umbra-gate.git`

## Current naming rules

- GitHub repositories still use `umbra-gate` and `homebrew-umbra-gate`
- Product name, binary name, archive name, and formula name use `umbragate`
- Formula file: `Formula/umbragate.rb`
- Release archives:
  - `umbragate_Darwin_arm64.tar.gz`
  - `umbragate_Darwin_x86_64.tar.gz`

## Pre-release checklist

In the main repo:

1. Inspect `git status --short`
2. Inspect `git diff --stat` and `git diff`
3. Inspect `git log --oneline -10`
4. Run tests:

```bash
env GOROOT="/opt/homebrew/opt/go/libexec" PATH="/opt/homebrew/opt/go/libexec/bin:$PATH" go test ./...
```

5. Build the binary:

```bash
env GOROOT="/opt/homebrew/opt/go/libexec" PATH="/opt/homebrew/opt/go/libexec/bin:$PATH" go build -o /tmp/umbragate .
```

In the tap repo:

1. Inspect `git status --short`
2. Confirm formula path is `Formula/umbragate.rb`
3. Confirm URLs still point at `https://github.com/jachy-h/umbra-gate/...`

## Versioning

- Tags use `vX.Y.Z`
- The release workflow triggers on pushing a tag or via manual dispatch
- The workflow publishes release assets from the main repo and updates SHA256 values in the tap repo when `HOMEBREW_TAP_TOKEN` is configured

## Release steps

### 1. Commit the main repo

Use a concise commit message matching repo style, for example:

- `feat: rename binary to umbragate and add daemon mode`
- `feat: default runtime data to ~/.umbragate`

### 2. Push the main branch

```bash
git push origin main
```

### 3. Create and push the tag

```bash
git tag vX.Y.Z
git push origin vX.Y.Z
```

### 4. Monitor the release workflow

```bash
gh run list --workflow release.yml --limit 5
gh run watch <run-id>
```

### 5. Verify release assets

```bash
gh release view vX.Y.Z --repo jachy-h/umbra-gate
gh release download vX.Y.Z --repo jachy-h/umbra-gate --dir /tmp/umbragate-release
```

Check that the release contains:

- `umbragate_Darwin_arm64.tar.gz`
- `umbragate_Darwin_x86_64.tar.gz`
- `sha256sums.txt`

### 6. Verify Homebrew tap update

In `homebrew-umbra-gate`:

1. Pull latest changes
2. Check `Formula/umbragate.rb`
3. Confirm:
   - version bumped
   - URLs point to the new `vX.Y.Z` release in `jachy-h/umbra-gate`
   - SHA256 values changed

## Manual fallback for tap update

If automation fails, update `Formula/umbragate.rb` manually:

1. Bump `version`
2. Update both archive URLs
3. Update both SHA256 values
4. Commit in the tap repo with:

```bash
git add Formula/umbragate.rb
git commit -m "chore: update umbragate vX.Y.Z"
git push origin main
```

## Post-release verification

Verify install flow:

```bash
brew tap jachy-h/umbra-gate
brew install umbragate
umbragate --help
```

Verify default filesystem layout:

- `~/.umbragate/config.example.yaml` should exist
- `~/.umbragate/config.yaml` is the active config file when created
- `~/.umbragate/router.db` is the local stats database
- `~/.umbragate/umbragate.log` is used by `umbragate -d`

## Important constraints

- Do not rename the GitHub repositories inside the release process unless the user explicitly asks for repository migration too
- Keep the binary/formula name `umbragate`
- Keep GitHub release and tap URLs pointed at the existing `umbra-gate` repositories until an explicit repo rename happens
