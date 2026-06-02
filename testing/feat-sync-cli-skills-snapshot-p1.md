# feat/sync-cli-skills-snapshot-p1 — Test Contract

## Functional Behavior

Regenerate CLI embedded skills snapshot after P1 skills merge (23 skills total).

## Unit Tests

```bash
node scripts/sync-cli-skills-snapshot.mjs
cd cli && go test -short -count=1 ./internal/skills/...
```

## Integration Tests

```bash
cd cli && go test -short -count=1 ./cmd/... -run Integration
```

## Smoke Tests

Snapshot manifest lists 23 skills including harness, multi-turn, dataset, prompt-eval.

## E2E Tests

N/A.

## Manual Tests

```bash
agentclash integration claude doctor
```
