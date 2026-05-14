# Voice Schema CLI - Test Contract

## Functional Behavior
- Add a CLI preflight command for validating voice report JSON files against JSON Schemas.
- Support `agentclash artifact validate-voice-report <file>`.
- Auto-detect schemas from report `type` for live continuity, video sync, and source separation reports.
- Support `--schema <path>` override, especially for video-sync reports where `type` may be omitted.
- Return success and useful output for valid reports.
- Return non-zero with useful error output for invalid reports or unknown schema selection.

## Unit Tests
- Valid live-continuity fixture auto-detects schema and passes.
- Valid video-sync fixture passes with explicit `--schema`.
- Invalid report returns an error.
- Unknown type without `--schema` returns a schema selection error.

## Integration / Functional Tests
- `cd cli && go test ./cmd` passes.

## Smoke Tests
- N/A - CLI command tested through Cobra command execution.

## E2E Tests
- N/A - local preflight command only.

## Manual / cURL Tests
- N/A - no HTTP endpoint change.
