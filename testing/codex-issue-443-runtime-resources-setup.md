# codex/issue-443-runtime-resources-setup - Test Contract

## Functional Behavior
- `web/content/agent-skills/agent-build-skills/agentclash-runtime-resources-setup/SKILL.md` remains an Agent Skills-compatible file with `name`, trigger-oriented `description`, and the three `metadata.agentclash.*` fields.
- The runtime resources skill explains the source-backed setup order for workspace secrets, model catalog entries, provider accounts, runtime profiles, model aliases, workspace tools, and readiness checks before builds/deployments/runs.
- Examples default to `AGENTCLASH_API_URL="https://api.agentclash.dev"` unless local or self-hosted behavior is explicit.
- The skill distinguishes workspace infrastructure resources from pack-defined tools and uses `workspace-secret://KEY` references for provider credentials.
- The skill includes concrete commands, JSON request examples, expected outputs, failure modes, safety notes, report-back format, and related `/docs-md/...` links.
- The generated docs page, `/docs-md/...` export, `/llms.txt`, and `/llms-full.txt` continue to include the updated skill through the existing docs pipeline.

## Unit Tests
- `web/src/lib/docs.test.ts` continues to verify nested agent-build skill pages are generated from canonical `SKILL.md` content.
- The docs test should assert at least one new runtime-resource detail that distinguishes the expanded workflow from the prior scaffold.

## Integration / Functional Tests
- From `web/`, run `npm test -- docs.test.ts` and confirm all docs-generation tests pass.
- From `web/`, run `npm run lint` and confirm lint passes.

## Smoke Tests
- `getDocBySlug(["agent-skills", "agent-build-skills", "agentclash-runtime-resources-setup"])` contains production API default, `workspace-secret://`, model catalog, provider account, runtime profile, model alias, workspace tool, and report-back guidance.
- `buildLlmsFull("https://example.test")` includes the expanded runtime resources skill body.

## E2E Tests
N/A - this change updates static skill content and docs-generation coverage, not a browser workflow.

## Manual / cURL Tests
Manual reviewer checks:

```bash
sed -n '1,320p' web/content/agent-skills/agent-build-skills/agentclash-runtime-resources-setup/SKILL.md
cd web
npm test -- docs.test.ts
npm run lint
```

Expected:
- The skill documents the source-backed runtime resource workflow without requiring source-code access.
- Tests and lint pass.
