# codex/issue-442-cli-setup-workflow - Test Contract

## Functional Behavior
- `web/content/agent-skills/agentclash-cli-setup/SKILL.md` remains an Agent Skills-compatible file with `name`, trigger-oriented `description`, and the three `metadata.agentclash.*` fields.
- The CLI setup skill explains production-hosted defaults, device login, token/env behavior, workspace selection, `link`, config precedence, local/self-hosted overrides, troubleshooting, safety notes, and report-back format.
- The workflow defaults to `AGENTCLASH_API_URL="https://api.agentclash.dev"` unless local or self-hosted behavior is explicit.
- The skill is useful to a coding agent without root-repo access by including concrete commands, expected outputs, failure modes, and related `/docs-md/...` links.
- The generated docs page, `/docs-md/...` export, `/llms.txt`, and `/llms-full.txt` continue to include the updated skill through the existing docs pipeline.

## Unit Tests
- `web/src/lib/docs.test.ts` continues to verify the `agentclash-cli-setup` page is generated from canonical `SKILL.md` content.
- The docs test should assert at least one new CLI setup detail that distinguishes the expanded workflow from the prior scaffold.

## Integration / Functional Tests
- From `web/`, run `npm test -- docs.test.ts` and confirm all docs-generation tests pass.
- From `web/`, run `npm run lint` and confirm lint passes.

## Smoke Tests
- `getDocBySlug(["agent-skills", "agentclash-cli-setup"])` contains production API default, config precedence, troubleshooting, and report-back guidance.
- `buildLlmsFull("https://example.test")` includes the expanded CLI setup skill body.

## E2E Tests
N/A - this change updates static skill content and docs-generation coverage, not a browser workflow.

## Manual / cURL Tests
Manual reviewer checks:

```bash
sed -n '1,260p' web/content/agent-skills/agentclash-cli-setup/SKILL.md
cd web
npm test -- docs.test.ts
npm run lint
```

Expected:
- The skill documents the source-backed CLI setup workflow without requiring source-code access.
- Tests and lint pass.
