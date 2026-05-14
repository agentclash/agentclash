# Cursor Review - Voice Report JSON Schemas

Model: `composer-2`

## First pass

Cursor found one direct schema contradiction and several limits to document:

- `pairs[].status` allowed `extra_translation`, but the Go video-sync validator only accepts `paired` and `missing_translation`.
- Paired video-sync rows could satisfy the schema with required fields set to `null`, while Go requires non-nil values.
- Segment end/start ordering, duration-range ordering, pair index bounds, and summary/pair count consistency are Go-ingestion checks that JSON Schema does not fully express here.

## Fixes made

- Removed `extra_translation` from the pair status enum.
- Added conditional non-null requirements for paired rows and missing-translation `source_index`.
- Added non-negative constraints for pair indexes and timing fields where the Go validator requires them.
- Added docs language that schemas are producer-side preflight checks and Go ingestion remains final for cross-field/cross-row invariants.
- Added tests for canonical generic report types, whitespace-normalized types, unsupported pair statuses, and paired rows with null required fields.
