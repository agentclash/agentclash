# Cursor Review - Voice Schema CLI

Model: `composer-2`

## First pass

Cursor found two blockers:

- `github.com/google/jsonschema-go` was imported by CLI code but listed as an indirect dependency.
- Auto-detection looked for `docs/schemas` by walking up from the process working directory, which would fail for shipped CLI installs outside a repository checkout.

## Fixes made

- Promoted `github.com/google/jsonschema-go` to a direct CLI dependency and ran `go mod tidy`.
- Embedded the three voice schemas under `cli/cmd/voice_schemas`.
- Kept `--schema` for explicit external schemas.

## Second pass

Cursor reported no blockers for embedded schema packaging, module hygiene, CLI UX, hidden network/API calls, or tests.
