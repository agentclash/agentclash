# feat/skills-p3-workflow — Test Contract

## Functional Behavior

P3 portable agent skills from issue #922:

- `agentclash-security-evaluation` — security stress-run, agent-vault-stress, runtime-stress, avmock-upstream.
- `agentclash-workspace-admin` — org/workspace CRUD and membership administration.
- Catalog, hub, docs nav, and docs tests updated (25 skills total).

## Unit Tests

- `web/src/lib/docs.test.ts`

## Integration Tests

```bash
cd web && npm test -- docs.test.ts && npm run lint
node scripts/sync-cli-skills-snapshot.mjs
cd cli && go test -short -count=1 ./internal/skills/...
```

## Smoke Tests

Both skills appear in `/docs-md/agent-skills/<name>` and `llms.txt`.

## Manual Tests

```bash
agentclash security stress-run --help
agentclash workspace members list --help
agentclash org list
```
