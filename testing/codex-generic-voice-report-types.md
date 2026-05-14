# Generic Voice Report Types — Test Contract

## Functional Behavior
- AgentClash voice artifact ingestion should prefer provider-neutral report `type` strings under the `agentclash.voice.*.v1` namespace.
- Existing Voicey-produced report `type` strings must remain accepted for backwards compatibility.
- Source-separation and live-continuity reports should reject unrelated type strings.
- Video-sync reports should support optional `schema_version` and `type` fields: omitted fields remain valid for older reports, generic AgentClash type is valid, Voicey legacy type is valid, unrelated type is rejected.
- Public evidence projections and artifact kinds should remain unchanged.

## Unit Tests
- `TestSourceSeparationReportAcceptsGenericType` — generic type ingests successfully.
- `TestSourceSeparationReportAcceptsLegacyVoiceyType` — legacy Voicey type ingests successfully.
- `TestLiveContinuityReportAcceptsGenericType` — generic type ingests successfully.
- `TestLiveContinuityReportAcceptsLegacyVoiceyType` — legacy Voicey type ingests successfully.
- `TestVideoSyncReportAcceptsGenericType` — generic type ingests successfully.
- `TestVideoSyncReportAcceptsLegacyVoiceyType` — legacy Voicey type ingests successfully.
- `TestVideoSyncReportRejectsWrongType` — unrelated type fails validation.

## Integration / Functional Tests
- `cd backend && go test ./internal/voiceartifacts ./internal/voicelive` passes.

## Smoke Tests
- N/A — backend artifact validator-only change.

## E2E Tests
- N/A — no user-facing workflow changes.

## Manual / cURL Tests
- N/A — no HTTP endpoint changes.
