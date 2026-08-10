---
name: umbra-gate-release
description: Prepare, verify, tag, publish, and audit an UmbraGate release. Use whenever a user asks to release, publish, cut a version, create or push a vX.Y.Z tag, update the Homebrew package, or check release readiness for this repository.
compatibility: Requires Git, Go, Node.js/npm, Make, and GitHub CLI or GitHub API access for post-release checks.
---

# UmbraGate Release

Treat a release as an immutable, verified commit. Do not create or push a tag until its exact commit passes the production verification below.

## 1. Prepare the release commit

1. Read `AGENTS.md`, `README.md`, and `README_zh.md`.
2. Pick a SemVer tag: `vMAJOR.MINOR.PATCH`, optionally with SemVer prerelease/build metadata.
3. Ensure both READMEs reflect all user-visible changes and remain equivalent in English and Chinese. Verify documented CLI commands against the current `umbragate --help` output.
4. Commit the implementation, generated embedded frontend assets when tracked, and both README updates together.
5. Confirm the worktree is clean with `git status --short`.

Do not use a mutable branch name as the release target. A release tag must identify one commit.

## 2. Run the required local gate

When Go and Node.js are available, run this from the repository root **before creating the tag**:

```bash
make release-verify TAG=vX.Y.Z
```

This command rejects a dirty tree, validates the SemVer value, checks that both READMEs changed since the previous release tag, builds the frontend into `internal/web/dist`, compiles the versioned Go binary, runs `go test ./...`, and verifies CLI help/version documentation.

If the command fails, fix the failure and rerun it. Never substitute unit tests alone for this gate.

## 3. Tag and publish

Only after the local gate succeeds:

```bash
git tag -a vX.Y.Z -m "release: vX.Y.Z"
git push origin HEAD
git push origin vX.Y.Z
```

The GitHub Actions release workflow validates that the tag is SemVer, checks out the tag’s exact commit, reruns the production release gate, then creates Darwin arm64/x86_64 archives, SHA-256 checksums, and the GitHub Release. It updates the Homebrew tap when `HOMEBREW_TAP_TOKEN` is configured.

Manual dispatch is for an **existing** tag only: enter the exact `vX.Y.Z` tag, never a branch name or a prospective version.

## 4. Verify publication

After the workflow succeeds, verify:

- The GitHub Release points to the intended tag and includes `umbragate_Darwin_arm64.tar.gz`, `umbragate_Darwin_x86_64.tar.gz`, and `sha256sums.txt`.
- Each archive’s SHA-256 matches `sha256sums.txt`.
- The Homebrew Formula version, URLs, and checksums match the release when the tap token is configured.
- The installed binary reports the released version:

```bash
umbragate version
```

Report the tag, commit SHA, workflow URL/status, assets, checksum result, and Homebrew update status. If GitHub authentication is unavailable, state that the remote checks could not be confirmed instead of claiming success.
