# feat/sync-cli-skills-snapshot-p0 — Test Contract

## Functional Behavior

Embedded CLI skills snapshot (`cli/internal/skills/snapshot/`) must match canonical docs after PR #923:

- 19 installable skills (excludes catalog `SKILL.md` only).
- New: `agentclash-hub`, `agentclash-quickstart`, `agentclash-compare-and-triage`.
- Updated: eval-runner, cli-setup, scorecard-reader, regression-flywheel, ci-release-gate.
- `manifest.json` snapshot_version changes; doctor/install use new hashes.

## Unit Tests

- `cli/internal/skills/skills_test.go` — Install and manifest load pass.
- `cli/cmd/integration_test.go` — integration install/doctor pass.

## Integration Tests

```bash
node scripts/sync-cli-skills-snapshot.mjs
cd cli && go test -short -count=1 ./internal/skills/... ./cmd/... -run 'Integration|Skills'
```

## Smoke Tests

```bash
cd cli && go run . integration claude doctor
cd cli && go run . integration codex doctor
```

## E2E Tests

N/A.

## Manual Tests

Verify snapshot skill count:

```bash
python3 -c "import json; print(len(json.load(open('cli/internal/skills/snapshot/manifest.json'))['skills']))"
# expect 19
```
