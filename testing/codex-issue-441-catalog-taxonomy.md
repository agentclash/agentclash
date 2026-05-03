# codex/issue-441-catalog-taxonomy - Test Contract

## Functional Behavior
- `web/content/agent-skills/SKILL.md` exists and defines the catalog taxonomy and generation contract for all AgentClash skill authors.
- The catalog skill uses trigger-oriented frontmatter with `name`, `description`, and `metadata` fields that match the existing `gray-matter` parser in `web/src/lib/docs.ts`.
- The skill explains exact inputs, required fields, commands, examples, failure modes, safety notes, dependency-order links, and report-back format.
- Examples default to `https://api.agentclash.dev` unless the workflow is explicitly local or self-hosted.
- The generated `/docs/agent-skills` and `/docs-md/agent-skills` catalog pages include the catalog contract content from the canonical `SKILL.md`.
- `/llms.txt` continues to include the Agent Skills entry and individual skill links.
- `/llms-full.txt` includes the Agent Skills catalog page and the catalog contract content.

## Unit Tests
- `web/src/lib/docs.test.ts` verifies the root catalog `SKILL.md` content is rendered into the Agent Skills index.
- `web/src/lib/docs.test.ts` verifies the root catalog contract appears in `llms-full.txt`.
- Existing agent skill docs tests continue to pass, including nested skill page generation, markdown path generation, and `llms.txt` inclusion.

## Integration / Functional Tests
- From `web/`, run `npm test -- docs.test.ts` and confirm all docs-generation tests pass.
- From `web/`, run `npm run lint` and confirm lint passes.

## Smoke Tests
- Generate docs surfaces through the tested helpers:
  - `getDocBySlug(["agent-skills"])` contains the catalog contract content.
  - `buildLlmsIndex("https://example.test")` contains `/docs-md/agent-skills`.
  - `buildLlmsFull("https://example.test")` contains the catalog contract content.

## E2E Tests
N/A - this change updates static skill content and docs-generation coverage, not a browser workflow.

## Manual / cURL Tests
Manual reviewer checks:

```bash
sed -n '1,220p' web/content/agent-skills/SKILL.md
cd web
npm test -- docs.test.ts
npm run lint
```

Expected:
- The skill documents the catalog taxonomy and generated-docs contract without requiring source-code access.
- Tests and lint pass.
