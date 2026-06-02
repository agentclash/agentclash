# feat/skills-p0-workflow — Test Contract

## Functional Behavior

P0 portable agent skills from issue #922 must ship in the canonical docs tree:

- `web/content/agent-skills/agentclash-hub/SKILL.md` — workflow map, dependency order, UI links, hosted defaults.
- `web/content/agent-skills/agentclash-quickstart/SKILL.md` — documents `agentclash quickstart` checks and next-command guidance.
- `web/content/agent-skills/agentclash-compare-and-triage/SKILL.md` — documents `compare latest`, `compare gate`, `baseline`, and `replay triage`.
- `web/content/agent-skills/agentclash-eval-runner/SKILL.md` — extended with `eval session list|get|follow` and `run series create|report`.
- Catalog `SKILL.md` dependency order includes hub, quickstart, and compare-and-triage.
- `web/src/lib/docs.ts` DOCS_NAV lists the three new top-level skills.
- `web/src/lib/docs.test.ts` includes new skill markdown paths and resolves nav items.

Each skill follows the catalog contract: frontmatter, Purpose, Use When, Do Not Use When, Inputs, Environment, Procedure, Commands, Expected Output, Failure Modes, Safety Notes, Report Back Format, Related Skills, Related Docs.

## Unit Tests

- `web/src/lib/docs.test.ts` — `generates an agent skills index page` still passes; index mentions new skills when present on disk.
- `web/src/lib/docs.test.ts` — `includes the index and every skill in markdown paths` includes:
  - `/docs-md/agent-skills/agentclash-hub`
  - `/docs-md/agent-skills/agentclash-quickstart`
  - `/docs-md/agent-skills/agentclash-compare-and-triage`
- `web/src/lib/docs.test.ts` — `resolves every docs navigation item` passes for new nav entries.
- `web/src/lib/docs.test.ts` — `includes platform pages, blog posts, and agent skills in llms.txt` includes new skill URLs.

## Integration / Functional Tests

- Docs generator discovers 20 skill files (17 existing + 3 new top-level).
- Category pages unchanged for nested skills.

## Smoke Tests

```bash
cd web && npm test -- docs.test.ts
cd web && npm run lint
```

## E2E Tests

N/A — documentation-only change; no browser E2E required.

## Manual / cURL Tests

```bash
# After build or dev server, verify markdown exports exist (local):
curl -sS http://localhost:3000/docs-md/agent-skills/agentclash-hub | head
curl -sS http://localhost:3000/docs-md/agent-skills/agentclash-quickstart | head
curl -sS http://localhost:3000/docs-md/agent-skills/agentclash-compare-and-triage | head
curl -sS http://localhost:3000/llms.txt | rg 'agentclash-hub|agentclash-quickstart|agentclash-compare-and-triage'
```

Manual CLI spot-check (hosted, optional):

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
agentclash quickstart --json
agentclash compare latest --help
agentclash replay triage --help
agentclash eval session list --help
agentclash run series report --help
```
