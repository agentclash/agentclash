Cursor Composer 2 review for `codex/generic-provider-log-metrics`.

Verdict: comment, no blockers.

Notes:

- `VideoSyncReport` now exposes provider-neutral `provider_log_metrics`.
- Legacy `voicey_log_metrics` decodes into `ProviderLogMetrics`.
- Generic metrics win when both generic and legacy keys are present.
- Default marshaling emits `provider_log_metrics` and no legacy `voicey_log_metrics` field.

Follow-up applied:

- Added a legacy-ingest marshal assertion so old Voicey payloads are re-emitted with the generic key only.
