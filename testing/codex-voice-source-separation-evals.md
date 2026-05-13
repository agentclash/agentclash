# codex/voice-source-separation-evals — Test Contract

## Functional Behavior

- Add voice eval metric support for float/ratio `voice.metric.recorded` events.
- Add stable metric keys for:
  - `dialogue_retention_ratio`
  - `background_preservation_ratio`
  - `speech_drop_risk`
- Allow voice scorecards to opt into a `media_policy` dimension when expectations require media preservation.
- `media_policy` should:
  - pass when dialogue retention and background preservation meet minimums and speech drop risk stays below the maximum
  - fail as a hard gate when any required metric violates its threshold
  - degrade when required source-separation evidence is missing
- Add release-gate deltas for media-policy score, background preservation, and speech drop risk.

## Unit Tests

- `backend/internal/voiceeval` tests ratio metric parsing, invalid ratio payloads, and missing metric evidence.
- `backend/internal/voicescorecard` tests media-policy pass, hard failure, and degraded evidence.
- `backend/internal/releasegate` tests media-policy release-gate regressions.

## Integration / Functional Tests

- Run focused backend Go tests:

```bash
cd backend
go test ./internal/voiceeval ./internal/voicescorecard ./internal/releasegate
```

## Smoke Tests

- Existing non-voice release-gate fixtures must still pass.
- Existing default voice scorecard tests must keep their current behavior unless media-policy expectations are explicitly enabled.

## E2E Tests

- N/A — this PR adds scoring primitives only. Voicey emits the source-separation report separately.

## Manual / cURL Tests

- N/A — no API routes are changed in this slice.

