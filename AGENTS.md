# Repository rules

## Release documentation

- Before creating or pushing any version tag or publishing a release, review and update both `README.md` and `README_zh.md`.
- The READMEs must reflect all user-visible changes in the release, including commands, flags, configuration defaults, installation steps, runtime behavior, and release artifacts.
- Verify documented CLI commands against the binary's current `--help` output and keep the English and Chinese instructions equivalent.
- Include the README updates in the release commit. Do not create the version tag while either README is outdated.

## Release verification

- A release must not be published unless the exact commit has first passed a full production build: build the frontend, embed it in the Go binary, and compile the binary.
- Keep the GitHub release workflow's verification job as a required dependency of the publishing job. It must run the frontend build, Go build, and test suite before any release asset or GitHub release is created.
- Before creating or pushing a release tag, run the same release verification locally when the required toolchain is available. Do not substitute unit tests alone for the production build.
