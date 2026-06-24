---
name: release-umbragate
description: Publish Umbragate releases and keep the Homebrew tap aligned. Use when tagging versions, creating GitHub releases, verifying release workflow runs, checking release assets, updating Formula/umbragate.rb, or validating Homebrew install behavior for this repository and its paired tap repo.
---

# Release Umbragate

Use this skill when publishing a new Umbragate release from this repository or checking that the Homebrew tap is aligned with a release.

## Repositories

- Main repo: `git@github.com:jachy-h/umbra-gate.git`
- Homebrew tap: `git@github.com:jachy-h/homebrew-umbragate.git`
- Local tap path, when present: `../homebrew-umbragate`

## Naming Rules

- GitHub main repository still uses `umbra-gate`.
- GitHub Homebrew tap repository uses `homebrew-umbragate`.
- Product name, binary name, archive name, and formula name use `umbragate`.
- Formula file: `Formula/umbragate.rb`.
- Release archives:
  - `umbragate_Darwin_arm64.tar.gz`
  - `umbragate_Darwin_x86_64.tar.gz`

## Pre-Release Checklist

In the main repo:

1. Inspect `git status --short`.
2. Inspect `git diff --stat` and `git diff`.
3. Inspect `git log --oneline -10`.
4. Run tests:

```bash
env GOROOT="/opt/homebrew/opt/go/libexec" PATH="/opt/homebrew/opt/go/libexec/bin:$PATH" go test ./...
```

5. Build the binary:

```bash
env GOROOT="/opt/homebrew/opt/go/libexec" PATH="/opt/homebrew/opt/go/libexec/bin:$PATH" go build -o /tmp/umbragate .
```

In the tap repo:

1. Inspect `git status --short`.
2. Confirm formula path is `Formula/umbragate.rb`.
3. Confirm URLs still point at `https://github.com/jachy-h/umbra-gate/...`.

## Versioning

- Tags use `vX.Y.Z`.
- The release workflow triggers on pushing a tag or via manual dispatch.
- The workflow publishes release assets from the main repo and updates SHA256 values in the tap repo when `HOMEBREW_TAP_TOKEN` is configured.

## Release Steps

### 1. Commit The Main Repo

Use a concise commit message matching repo style, for example:

- `feat: rename binary to umbragate and add daemon mode`
- `feat: default runtime data to ~/.umbragate`

### 2. Push Main

```bash
git push origin main
```

### 3. Create And Push The Tag

```bash
git tag vX.Y.Z
git push origin vX.Y.Z
```

### 4. Monitor The Release Workflow

```bash
gh run list --workflow release.yml --limit 5
gh run watch <run-id>
```

### 5. Verify Release Assets

```bash
gh release view vX.Y.Z --repo jachy-h/umbra-gate
gh release download vX.Y.Z --repo jachy-h/umbra-gate --dir /tmp/umbragate-release
```

The release should contain:

- `umbragate_Darwin_arm64.tar.gz`
- `umbragate_Darwin_x86_64.tar.gz`
- `sha256sums.txt`

### 6. Verify Homebrew Tap Update

In `homebrew-umbragate`:

1. Pull latest changes.
2. Check `Formula/umbragate.rb`.
3. Confirm the version is bumped.
4. Confirm URLs point to the new `vX.Y.Z` release in `jachy-h/umbra-gate`.
5. Confirm SHA256 values changed.

## Manual Tap Fallback

If automation fails, update `Formula/umbragate.rb` manually:

1. Bump `version`.
2. Update both archive URLs.
3. Update both SHA256 values.
4. Commit in the tap repo:

```bash
git add Formula/umbragate.rb
git commit -m "chore: update umbragate vX.Y.Z"
git push origin main
```

## Post-Release Verification

Verify install flow:

```bash
brew tap jachy-h/umbragate
brew install umbragate
umbragate --help
```

Verify default filesystem layout:

- `~/.umbragate/config.yaml` should exist.
- `~/.umbragate/config.yaml` is the active config file when created.
- `~/.umbragate/router.db` is the local stats database.
- `~/.umbragate/umbragate.log` is used by `umbragate -d`.

## Important Constraints

- Do not rename the GitHub repositories inside the release process unless the user explicitly asks for repository migration too.
- Keep the binary/formula name `umbragate`.
- Keep GitHub release and tap URLs pointed at the existing `umbra-gate` repositories until an explicit repo rename happens.
