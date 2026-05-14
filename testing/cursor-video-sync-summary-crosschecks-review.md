Cursor Composer 2 review for `codex/video-sync-summary-crosschecks`.

Verdict: comment, no blockers.

Notes:

- Summary/pair crosschecks match the golden fixture arithmetic: five source segments, three paired rows, two missing rows, and 0.6 coverage.
- Numeric summary fields remain optional; crosschecks only enforce fields that are present.
- Review suggested adding a symmetric test for `extra_translation_segments` mismatches.

Follow-up applied:

- Added `TestVideoSyncReportRejectsExtraTranslationCountMismatch`.
