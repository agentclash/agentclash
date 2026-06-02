# feat/sync-cli-skills-snapshot-p3 — Test Contract

## Functional Behavior

Regenerate CLI embedded skills snapshot after P3 skills merge (25 skills total).

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

Snapshot manifest lists 25 skills including security-evaluation and workspace-admin.

## Manual Tests

```bash
agentclash integration claude doctor
```
