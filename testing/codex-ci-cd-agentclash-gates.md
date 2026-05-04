# Codex CI CD AgentClash Gates — Test Contract

## Functional Behavior

- The CLI exposes an `agentclash ci` command group for main-product CI/CD integration.
- `agentclash ci init <file>` writes a sample AgentClash CI manifest without overwriting existing files unless `--force` is passed.
- `agentclash ci init <file>` creates missing parent directories for the target manifest path so the documented `.agentclash/ci.yaml` quickstart works in a fresh repository.
- `agentclash ci validate <file>` validates a YAML manifest that describes:
  - watched source paths that should trigger AgentClash CI
  - the candidate agent build spec and deployment resource IDs
  - the evaluation workload using a challenge pack version, optional input set, and optional regression suites/cases
  - the baseline run or deployment reference used for comparison
  - release gate and regression promotion policy
- Validation succeeds for a well-formed manifest and prints structured JSON when `--json` is used.
- Validation fails with a nonzero exit when required sections or IDs are missing, path globs are blank, regression suite/case IDs are blank, enum values are invalid, unknown manifest fields are present, or the manifest is not valid YAML.
- The feature remains scoped to main-product CI. It must not depend on experimental agent harnesses.

## Unit Tests

- CLI tests cover `ci init` creating a manifest with expected top-level sections.
- CLI tests cover `ci init` creating missing parent directories for nested manifest paths.
- CLI tests cover `ci init` refusing to overwrite unless `--force` is present.
- CLI tests cover `ci validate` accepting a valid manifest.
- CLI tests cover `ci validate` rejecting missing `trigger.paths`.
- CLI tests cover `ci validate` rejecting unknown manifest fields so typoed contract keys fail fast.
- CLI tests cover `ci validate` rejecting missing candidate build/deployment resource fields.
- CLI tests cover `ci validate` rejecting a missing evaluation challenge pack version.
- CLI tests cover `ci validate` rejecting blank regression suite and regression case entries.
- CLI tests cover `ci validate` rejecting a missing baseline reference.
- CLI tests cover invalid enum values for release gate failure mode and regression promotion mode.

## Integration / Functional Tests

- N/A — this PR is limited to local CLI manifest parsing/validation and docs. It does not create backend records or call the AgentClash API for CI orchestration.

## Smoke Tests

- From `cli/`, `go test ./cmd -run 'TestCI' -count=1` passes.
- From `cli/`, `go test -short ./cmd -count=1` passes.
- From `cli/`, `go build ./...` passes.

## E2E Tests

- N/A — full GitHub Actions orchestration will be a follow-up once the manifest contract is merged.

## Manual / cURL Tests

```bash
cd cli
go run . ci init /tmp/agentclash-ci.yaml --force
go run . ci validate /tmp/agentclash-ci.yaml
go run . ci validate /tmp/agentclash-ci.yaml --json
```

Expected:
- `ci init` creates the file and reports success.
- `ci validate` prints a success message for the generated manifest.
- `ci validate --json` prints a JSON object with `valid: true`.
