# feat/integration-all-hosts — Test Contract

## Functional Behavior

Extend `agentclash integration` install/doctor to cursor, openclaw, hermes, and opencode (matching agent-skills bundle paths).

## Unit Tests

```bash
cd cli && go test -short -count=1 ./internal/skills/... -run HostInstall
cd cli && go test -short -count=1 ./cmd/... -run Integration
```

## Integration Tests

N/A.

## Smoke Tests

Each host writes `agentclash-cli-setup/SKILL.md` under the expected relative path.

## E2E Tests

N/A.

## Manual Tests

```bash
agentclash integration cursor install --dir /tmp/ac-skills-test
agentclash integration cursor doctor --dir /tmp/ac-skills-test
```
