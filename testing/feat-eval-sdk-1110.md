# feat/eval-sdk-1110 — Test Contract

## Functional Behavior

- `agentclash evaltest init` scaffolds config and example test without auth.
- `agentclash evaltest run` executes Python SDK smoke runner and emits JSON/JUnit.
- Exit codes 0–4 implemented.
- Supports `--out`, `--format json|junit|both`.

## Unit Tests

- `TestEvaltestInitCreatesFiles`
- `TestEvaltestRunWritesJSONAndJUnit`
- `TestEvaltestRunMissingConfigUsesExitCode2`

## Smoke Tests

- `cd cli && go test -short -count=1 ./cmd -run TestEvaltest`

## Manual Tests

- `go run . evaltest init && go run . evaltest run --format both --out /tmp/ac-results`
