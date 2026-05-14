# Generic Voice Artifact Contracts - Test Contract

## Functional Behavior
- Add platform-level documentation for generic voice-agent eval artifacts, independent of any single producer.
- Document accepted generic report `type` values for live continuity, video sync, and source separation.
- Explain legacy producer aliases as backward compatibility only, not the preferred contract.
- Explain required versus optional voice artifact manifest kinds.
- Include minimal JSON examples that match the current Go validators.
- Call out validator edge cases that producers often get wrong: checksum format, report-specific status values, `passed`/`status` coupling, count fields, and video-sync summary/pair coupling.
- Link the new page from the docs sidebar so users can discover it.

## Unit Tests
- N/A - docs-only change.

## Integration / Functional Tests
- Documentation navigation data should include the new page.
- Existing voice artifact Go tests continue passing if code-adjacent examples reveal a validator mismatch.

## Smoke Tests
- Run the docs/web lint or typecheck command available in the repo, if dependencies are installed.
- If dependency installation is unavailable, verify file paths and sidebar wiring with repository search.

## E2E Tests
- N/A - docs-only change.

## Manual / cURL Tests
- N/A - no HTTP endpoint change.
