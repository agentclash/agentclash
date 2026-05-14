# Voice Manifest CLI — Test Contract

## Functional Behavior

- Add `agentclash artifact validate-voice-manifest <file>` for local producer preflight.
- The command validates a JSON manifest against the generic AgentClash `voice-artifact-manifest.schema.json`.
- The schema is embedded in the CLI so validation works outside an AgentClash repository checkout.
- Human output prints the manifest path, schema, and validity for successful validation.
- `--json` output emits `{ "path": "...", "schema": "...", "valid": true }` on success.
- Validation failures return a nonzero exit and, with `--json`, emit `{ "valid": false, "errors": [...] }`.
- The command remains producer-neutral and does not introduce Voicey-specific flags, aliases, or fields.

## Unit Tests

- `TestArtifactValidateVoiceManifestAcceptsValidManifest` validates a complete local-path manifest through the CLI.
- `TestArtifactValidateVoiceManifestRejectsInvalidManifest` rejects an invalid manifest and returns structured JSON failure output.
- `TestEmbeddedVoiceSchemasMatchDocsSchemas` continues to catch drift for every embedded voice schema, including the manifest schema.

## Integration / Functional Tests

- The manifest schema is copied into `cli/cmd/voice_schemas` and is covered by the embedded-vs-docs drift test.
- The voice artifact contract docs mention the CLI preflight command.

## Smoke Tests

- `cd cli && go test ./cmd` passes.

## E2E Tests

- N/A — this is an offline CLI validation command.

## Manual / cURL Tests

```bash
cd cli
go test ./cmd
```

Expected: tests pass.
