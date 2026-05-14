# Voice Manifest Schema — Test Contract

## Functional Behavior

- Add a producer-neutral JSON Schema for AgentClash voice artifact manifests.
- The schema accepts the canonical `2026-05-13` manifest version, UUID `run_id` and `run_agent_id`, non-empty `voice_session_id`, and a non-empty `artifacts` array.
- Each artifact requires a `key`, supported generic `kind`, supported `location`, and lowercase SHA-256 checksum. The schema rejects exact duplicate artifact entries; the Go validator remains responsible for enforcing unique keys across different artifact objects.
- `local_path` artifacts require a relative `path` and must not set `bucket` or `object_key`.
- `object_storage` artifacts require `bucket` and `object_key` and must not set `path`.
- The schema requires the same required artifact kinds as the Go validator: `caller_audio`, `agent_audio`, `transcript_json`, `waveform_timeline_json`, and `structured_output_json`.
- The schema remains generic to AgentClash voice evals and does not introduce Voicey-specific canonical fields.

## Unit Tests

- `TestVoiceArtifactManifestSchemaAcceptsExamples` validates a minimal local-path manifest and a richer object-storage manifest against the schema.
- `TestVoiceArtifactManifestSchemaRejectsInvalidExamples` rejects invalid version, invalid UUIDs, missing required artifact kinds, exact duplicate artifacts, invalid checksums, unsafe local paths, and invalid location/reference combinations.
- Existing `voiceartifacts.Manifest.Validate` tests continue to pass.

## Integration / Functional Tests

- The schema is linked from the voice artifact contract docs alongside report schemas.
- Existing docs schema tests continue validating the report schemas.

## Smoke Tests

- `go test ./internal/voiceartifacts` passes from the backend module.

## E2E Tests

- N/A — this PR adds a schema/docs contract and backend schema validation tests only.

## Manual / cURL Tests

```bash
cd backend
go test ./internal/voiceartifacts
```

Expected: tests pass.
