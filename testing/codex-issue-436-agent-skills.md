# codex/issue-436-agent-skills — Test Contract

## Functional Behavior
- AgentClash exposes a canonical catalog of copyable Agent Skills for coding agents, with one source of truth in the repo.
- The MVP catalog is a structured skill tree, not one large undifferentiated prompt.
- The catalog includes top-level operating skills:
  - `agentclash-cli-setup`
  - `agentclash-eval-runner`
  - `agentclash-scorecard-reader`
  - `agentclash-regression-flywheel`
  - `agentclash-ci-release-gate`
- The catalog includes granular Challenge Pack skills under `challenge-pack-skills/` so an agent can discover a focused workflow without reading source code:
  - `agentclash-challenge-pack-planner`
  - `agentclash-challenge-pack-yaml-author`
  - `agentclash-challenge-pack-input-sets`
  - `agentclash-challenge-pack-scoring-validators`
  - `agentclash-challenge-pack-llm-judges`
  - `agentclash-challenge-pack-tools-sandbox`
  - `agentclash-challenge-pack-artifacts`
  - `agentclash-challenge-pack-validation-publish`
- The catalog includes Agent Build and Deployment skills under `agent-build-skills/`:
  - `agentclash-agent-build-author`
  - `agentclash-agent-deployment-setup`
  - `agentclash-runtime-resources-setup`
- Each skill is Agent Skills-compatible, uses YAML frontmatter with `name`, `description`, and `metadata`, and includes practical procedure sections for agents.
- CLI examples in skill content use production with `AGENTCLASH_API_URL="https://api.agentclash.dev"` unless the workflow is explicitly local or CI-oriented.
- The public docs navigation includes an Agent Skills section with a catalog/index page and individual skill pages.
- Each skill page exposes the full `SKILL.md` content in a copyable block and links to its markdown export.
- `/docs-md/agent-skills/...` serves markdown for the index, category pages, and every skill page.
- `/llms.txt` links to the Agent Skills docs, and `/llms-full.txt` includes the Agent Skills corpus.
- The existing docs search index includes the skill pages through the same docs pipeline.
- The implementation keeps generated CLI/config reference behavior unchanged.

## Unit Tests
- Add focused tests for the docs library behavior:
  - `getDocBySlug(["agent-skills"])` returns the Agent Skills index.
  - `getDocBySlug(["agent-skills", "challenge-pack-skills"])` returns a category page.
  - `getDocBySlug(["agent-skills", "challenge-pack-skills", "<skill>"])` returns nested individual skill pages with copyable `SKILL.md` content.
  - `getDocBySlug(["agent-skills", "agent-build-skills", "<skill>"])` returns Agent Build and Deployment skill pages.
  - `getAllDocMarkdownPaths()` includes `/docs-md/agent-skills`, category paths, and all skill markdown paths.
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

Expected: build exits 0 and includes docs routes for `/docs/agent-skills`, `/docs/agent-skills/challenge-pack-skills`, `/docs/agent-skills/challenge-pack-skills/agentclash-challenge-pack-yaml-author`, and `/docs/agent-skills/agent-build-skills/agentclash-agent-deployment-setup`.
