# feat/skills-p1-workflow — Test Contract

## Functional Behavior

P1 portable agent skills from issue #922:

- `agentclash-agent-harness-setup` — harness create/run/suite/execution/failure-review CLI.
- `agentclash-multi-turn-operator` — `run turn status|submit` for human takeover phases.
- `agentclash-dataset-workflows` — dataset CRUD, import/export, eval, test gate, generate, traces, sync-regression-suite.
- `agentclash-prompt-eval-playground` — `prompt-eval` and `playground` command workflows.
- Catalog, hub, docs nav, and docs tests updated.

## Unit Tests

- `web/src/lib/docs.test.ts` — new markdown paths and page content assertions.

## Integration Tests

```bash
cd web && npm test -- docs.test.ts
cd web && npm run lint
node scripts/sync-cli-skills-snapshot.mjs
cd cli && go test -short -count=1 ./internal/skills/...
```

## Smoke Tests

All four skills appear in `/docs-md/agent-skills/<name>` and `llms.txt`.

## E2E Tests

N/A.

## Manual Tests

```bash
agentclash agent-harness list --help
agentclash run turn status --help
agentclash dataset test --help
agentclash prompt-eval validate --help
agentclash playground list --help
```
