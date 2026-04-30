# codex/issue-436-agent-skills — Test Contract

## Functional Behavior
- AgentClash exposes a canonical catalog of copyable Agent Skills for coding agents, with one source of truth in the repo.
- The MVP catalog includes these six Agent Skills:
  - `agentclash-cli-setup`
  - `agentclash-challenge-pack-author`
  - `agentclash-eval-runner`
  - `agentclash-scorecard-reader`
  - `agentclash-regression-flywheel`
  - `agentclash-ci-release-gate`
- Each skill is Agent Skills-compatible, uses YAML frontmatter with `name`, `description`, and `metadata`, and includes practical procedure sections for agents.
- CLI examples in skill content prefer staging with `AGENTCLASH_API_URL="https://staging-api.agentclash.dev"` unless the workflow is explicitly local or CI-oriented.
- The public docs navigation includes an Agent Skills section with a catalog/index page and individual skill pages.
- Each skill page exposes the full `SKILL.md` content in a copyable block and links to its markdown export.
- `/docs-md/agent-skills/...` serves markdown for the index and every skill page.
- `/llms.txt` links to the Agent Skills docs, and `/llms-full.txt` includes the Agent Skills corpus.
- The existing docs search index includes the skill pages through the same docs pipeline.
- The implementation keeps generated CLI/config reference behavior unchanged.

## Unit Tests
- Add focused tests for the docs library behavior:
  - `getDocBySlug(["agent-skills"])` returns the Agent Skills index.
  - `getDocBySlug(["agent-skills", "<skill>"])` returns individual skill pages with copyable `SKILL.md` content.
  - `getAllDocMarkdownPaths()` includes `/docs-md/agent-skills` and all six skill markdown paths.
  - `buildLlmsIndex()` includes Agent Skills links.
  - `buildLlmsFull()` includes the skill catalog and at least one individual skill body.

## Integration / Functional Tests
- Run the web test suite path that covers docs library changes.
- Run type/lint validation for the web app if available and reasonably scoped.
- Verify the docs route generation still uses `getAllDocSlugs()` and therefore includes the Agent Skills pages.

## Smoke Tests
- Build or typecheck the web app enough to catch broken imports, bad MDX, and route generation errors.
- Manually inspect generated markdown output for at least one skill page to confirm the full skill body is present.

## E2E Tests
- N/A — this change is static docs/content and library routing. Browser E2E is not required for the MVP.

## Manual / cURL Tests
```bash
cd web
npm test -- --run src/lib/docs.test.ts
npm run lint
```

Expected: both commands exit 0.

```bash
cd web
npm run build
```

Expected: build exits 0 and includes docs routes for `/docs/agent-skills` and `/docs/agent-skills/agentclash-cli-setup`.
