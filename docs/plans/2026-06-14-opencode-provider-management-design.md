# opencode Provider Management Design

## Scope

Add and refine a web-based provider management page for opencode global configuration. This iteration only manages opencode provider credentials and writes to user-selected global opencode config files.

The feature must preserve existing opencode configuration. It only changes provider fields selected by the user and never replaces unrelated config fields.

## User Experience

`/dashboard/providers` uses a four-step flow:

1. Choose the opencode config file.
2. Choose a provider from common providers or enter another provider id.
3. Enter API key and, only when needed, advanced metadata such as display name, API type, and base URL.
4. Preview a masked diff and explicitly confirm before applying.

Common provider choices include OpenAI, Anthropic, DeepSeek, GLM, and Kimi. Other provider ids are allowed with the same simple API-key-only flow because opencode may already know those providers. The advanced section is optional and is only for providers that need custom metadata.

Provider setup does not ask for model ids, default `model`, or `small_model`.

## Config File Discovery

The page scans supported opencode global config filenames under `~/.config/opencode`:

- `opencode.json`
- `opencode.jsonc`
- `.opencode/opencode.json`
- `.opencode/opencode.jsonc`

If multiple files exist, the page asks the user to choose which one to edit. If none exist, the page offers to create `~/.config/opencode/opencode.json`.

The selected config path is sent with diff and apply requests. Apply rejects stale confirmations if the selected file changed after diff generation.

## Data and File Handling

Read and write the selected config file.

If the selected file does not exist, create a minimal config with `$schema` and `provider`. If it exists, parse it into a generic object so unknown fields and user settings are preserved.

JSONC input supports comments and trailing commas. Output is formatted as JSON, which is valid JSONC. Before writing, create a timestamped backup beside the target file.

Write behavior:

- Preserve all unrelated top-level fields.
- Preserve existing provider entries unless the user edits or deletes them.
- Update `provider.<id>.options.apiKey` from the API key field.
- Add `provider.<id>.options.baseURL` only when supplied.
- Add `provider.<id>.api` only when supplied.
- Add `provider.<id>.name` only when supplied.
- Do not write provider `models`.
- Do not write top-level `model` or `small_model`.
- Ensure `$schema` remains `https://opencode.ai/config.json` when creating a new file.

Use atomic write: write to a temporary file in the same directory, then rename it over the target file.

## API

Dashboard-scoped endpoints:

- `GET /dashboard/providers/config`: returns discovered config files, selected masked config, and provider catalog.
- `POST /dashboard/providers/diff`: accepts `path` and provider change, returns a masked unified diff without writing.
- `POST /dashboard/providers/apply`: accepts `path`, provider change, and checksum; writes only if the file still matches the diff base.

## Security

API keys are accepted from the page and stored in opencode config. Existing API keys are masked in normal page responses. Diff output masks old and new API key values.

Do not log API keys or request bodies.

## Error Handling

If the selected config file is invalid JSON/JSONC, show a clear error and do not write.

If multiple config files exist, require explicit file selection before diff/apply.

If the config directory does not exist, create it on apply for the default new file path.

If backup or atomic write fails, return an error and leave the original file untouched.

If the selected file changes between diff and apply, reject the apply request and ask the user to regenerate the diff.

## Testing

Add or update tests for:

- Discovering all supported global config file names.
- Choosing among multiple existing files.
- Creating the default config when none exists.
- Parsing JSONC comments and trailing commas.
- Preserving unrelated config fields.
- Updating providers without writing models or default models.
- Masking API keys in read and diff responses.
- Rejecting stale apply requests.
- Rendering the four-step provider UI.

Run Go tests and build verification after implementation.
