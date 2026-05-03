# Codex Issue 446 Challenge Pack Planner Skill - Test Contract

## Functional Behavior
- Expand `web/content/agent-skills/challenge-pack-skills/agentclash-challenge-pack-planner/SKILL.md` from scaffold into a source-backed planning workflow for issue #446.
- The skill must help a coding agent turn a vague eval idea into pack boundaries, cases, input sets, scoring strategy, tools, artifacts, publish criteria, and handoff plan without reading AgentClash source code.
- The skill must preserve accurate frontmatter, use hosted production examples where applicable, and avoid claiming CLI use is required for planning.
- The skill must link related skills in dependency order and include a concrete report-back format.

## Unit Tests
- `web/src/lib/docs.test.ts` should continue to pass for agent skill discovery and docs/llms coverage.
- No dedicated unit test is required for prose-only content beyond the existing docs tests.

## Integration / Functional Tests
- The docs generator must discover the expanded `SKILL.md` under `/docs-md/agent-skills/challenge-pack-skills/agentclash-challenge-pack-planner`.
- `/llms.txt` and `/llms-full.txt` coverage must remain driven by existing docs discovery.

## Smoke Tests
- Run the relevant web docs tests from `web/`.
- Run a markdown/source sanity pass that checks the skill references current challenge-pack concepts: `pack`, `version`, `challenges`, `input_sets`, `evaluation_spec`, `prompt_eval`, and `native`.

## E2E Tests
- N/A - this is a documentation/skill content change. The existing docs export tests are the appropriate coverage for this PR.

## Manual / cURL Tests
- Manually inspect the expanded skill against:
  - `backend/internal/challengepack/bundle.go`
  - `backend/internal/challengepack/validation.go`
  - `cli/cmd/challenge_pack.go`
  - `web/content/docs/concepts/challenge-packs-and-inputs.mdx`
  - `web/content/docs/guides/write-a-challenge-pack.mdx`
- Confirm examples and advice do not ask agents to read root repo source as a normal workflow.
