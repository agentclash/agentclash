Cursor Composer 2 review for `codex/generic-voice-report-types`.

Verdict: comment, no blockers.

Notes:

- Provider-neutral `agentclash.voice.*.v1` report types are wired while Voicey legacy `voicey.*` types remain accepted.
- Existing Voicey fixtures still ingest.
- Video-sync type/schema fields are optional for older reports; when present, type is validated.

Follow-up applied:

- Added `TestLiveContinuityReportRejectsWrongType` for symmetry with source separation and video sync.
- Normalized report type comparison with `strings.TrimSpace` before allowlist matching.
