# Cursor Review - Generic Voice Artifact Contracts

Model: `composer-2`

## First pass

Cursor found documentation gaps around validator-specific behavior:

- Manifest checksums must be 64 lowercase hex characters.
- `passed` must mirror `status` for live continuity and source separation.
- Live continuity cannot pass when evidence is degraded.
- Report status vocabularies differ by report.
- Video-sync `schema_version` is recommended but not enforced.
- Legacy alias strings should be listed explicitly.
- Count fields and video-sync summary/pair coupling should be called out.

## Follow-up

After fixes, Cursor rechecked the docs against:

- `backend/internal/voiceartifacts/manifest.go`
- `backend/internal/voiceartifacts/live_continuity_report.go`
- `backend/internal/voiceartifacts/video_sync_report.go`
- `backend/internal/voiceartifacts/source_separation_report.go`

Result: no blockers. Cursor confirmed validator alignment, docs navigation/link targets, and producer-neutral language.
