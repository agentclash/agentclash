# Voice Report JSON Schemas - Test Contract

## Functional Behavior
- Add machine-readable JSON Schemas for the generic AgentClash voice report payloads:
  - `agentclash.voice.live_continuity_eval.v1`
  - `agentclash.voice.video_sync_eval.v1`
  - `agentclash.voice.source_separation_eval.v1`
- Keep schemas producer-neutral and store them with the existing schema docs under `docs/schemas`.
- Include current legacy aliases as compatibility values where the Go validators still ingest them.
- Encode important validator footguns: report-specific statuses, `passed`/`status` coupling, degraded evidence rule, integer counts, ratio bounds, and required interpretation/ratios.

## Unit Tests
- `TestVoiceReportSchemasAcceptFixtures` - current voice report fixtures validate against their matching schema.
- `TestVoiceReportSchemasRejectInvalidExamples` - schemas reject known invalid edge cases.

## Integration / Functional Tests
- `cd backend && go test ./internal/voiceartifacts` passes.

## Smoke Tests
- `jq empty docs/schemas/voice-*.schema.json` passes if `jq` is available.

## E2E Tests
- N/A - schema-only platform contract change.

## Manual / cURL Tests
- N/A - no HTTP endpoint change.
