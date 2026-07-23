# Repository rules

## Release documentation

- Before creating or pushing any version tag or publishing a release, review and update both `README.md` and `README_zh.md`.
- The READMEs must reflect all user-visible changes in the release, including commands, flags, configuration defaults, installation steps, runtime behavior, and release artifacts.
- Verify documented CLI commands against the binary's current `--help` output and keep the English and Chinese instructions equivalent.
- Include the README updates in the release commit. Do not create the version tag while either README is outdated.
