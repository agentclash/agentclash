# Generic Provider Log Metrics — Test Contract

## Functional Behavior
- Video-sync reports should expose provider log metrics through a provider-neutral `provider_log_metrics` field on `VideoSyncReport`.
- Legacy Voicey reports using `voicey_log_metrics` must continue to ingest and populate `ProviderLogMetrics`.
- If both generic and legacy metrics are present, the generic `provider_log_metrics` field wins.
- Marshal output should use `provider_log_metrics`; AgentClash should not emit new `voicey_log_metrics` keys.
- Existing summary, segment, pair, and timing evidence validation behavior must remain unchanged.

## Unit Tests
- `TestVideoSyncReportAcceptsProviderLogMetrics` — generic field ingests into `ProviderLogMetrics`.
- `TestVideoSyncReportAcceptsLegacyVoiceyLogMetrics` — legacy field ingests into `ProviderLogMetrics`.
- `TestVideoSyncReportProviderLogMetricsWinsOverLegacy` — generic value wins when both fields exist.
- Existing video-sync validation tests continue passing.

## Integration / Functional Tests
- `cd backend && go test ./internal/voiceartifacts ./internal/voicelive` passes.

## Smoke Tests
- N/A — backend artifact validator-only change.

## E2E Tests
- N/A — no user-facing workflow changes.

## Manual / cURL Tests
- N/A — no HTTP endpoint changes.
