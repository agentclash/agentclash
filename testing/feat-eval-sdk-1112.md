# feat/eval-sdk-1112 — Test Contract

## Functional Behavior

- `agentclash evaltest promote-failures --from ... --to ...`
- Writes draft challenge-pack YAML from failed cases
- Supports `--dry-run` and `--append`

## Unit Tests

- `TestEvaltestPromoteFailuresWritesDraftPack`
- `TestEvaltestPromoteFailuresDryRunNoFailures`

## Smoke Tests

- `go test ./cmd -run TestEvaltestPromote`

## Manual Tests

- Promote a metric-failure fixture report
