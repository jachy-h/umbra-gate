# Project Rules

## Project Shape

- Product, binary, release archive, and Homebrew formula name: `umbragate`.
- GitHub main repository: `jachy-h/umbra-gate`.
- Homebrew tap repository: `jachy-h/homebrew-umbragate`.
- Homebrew tap path, when present next to this repo: `../homebrew-umbragate`.
- Formula path in the tap repository: `Formula/umbragate.rb`.

## Development

- This is a Go project. Prefer the existing package layout and keep changes focused.
- Run `go test ./...` after code changes.
- Build locally with `go build -o umbragate .` when startup or packaging behavior changes.
- The app serves the dashboard at `http://127.0.0.1:4141/dashboard` by default.

## Runtime Paths

- `UMBRAGATE_HOME/config.yaml` wins when `UMBRAGATE_HOME` is set.
- Otherwise `./config.yaml` is used when present in the current working directory.
- Otherwise runtime data defaults to `~/.umbragate/`.
- Local runtime files include `config.yaml`, `router.db`, and `umbragate.log`.

## Release And Homebrew

- Use the `release-umbragate` Codex skill for publishing, tagging, release workflow checks, or Homebrew tap updates.
- Keep repository URLs pointed at `jachy-h/umbra-gate` and `jachy-h/homebrew-umbragate` unless the user explicitly asks for repository migration.
- Tags use `vX.Y.Z`.
- Release assets are named `umbragate_Darwin_arm64.tar.gz`, `umbragate_Darwin_x86_64.tar.gz`, and `sha256sums.txt`.

## Browser Checks

- When checking the running dashboard, use the in-app browser/browser automation when available.
- Verify both `/dashboard` and JSON API endpoints relevant to the changed area.
