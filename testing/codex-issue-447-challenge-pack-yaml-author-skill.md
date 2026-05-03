# Codex Issue 447 Challenge Pack YAML Author Skill - Test Contract

## Functional Behavior
- Expand `web/content/agent-skills/challenge-pack-skills/agentclash-challenge-pack-yaml-author/SKILL.md` for issue #447.
- The skill must let an agent write or edit challenge-pack YAML without reading AgentClash source code.
- The skill must document the exact current bundle shape: `pack`, `version`, optional `tools`, `challenges`, and `input_sets`.
- The skill must describe required fields, execution modes, case/input/expectation shapes, validation commands, common validation failures, and handoff to publish.
- Examples must use hosted production by default and must avoid unsupported fields or tool kinds.

## Unit Tests
- `web/src/lib/docs.test.ts` should include assertions for the YAML author skill's source-backed details.
- Existing docs tests must pass.

## Integration / Functional Tests
- The docs generator must discover the updated nested skill page at `/docs-md/agent-skills/challenge-pack-skills/agentclash-challenge-pack-yaml-author`.
- `llms.txt` and `llms-full.txt` inclusion must remain covered by existing docs tests.

## Smoke Tests
- Run `npm test -- src/lib/docs.test.ts` from `web/`.
- Run `git diff --check`.
- Run a keyword sanity pass for `pack`, `version`, `challenges`, `input_sets`, `evaluation_spec`, `prompt_eval`, `native`, and `agentclash challenge-pack validate`.

## E2E Tests
- N/A - this is a skill documentation/content change. The internal blind skill harness should run on the PR because `SKILL.md` changes.

## Manual / cURL Tests
- Manually inspect the skill against:
  - `backend/internal/challengepack/bundle.go`
  - `backend/internal/challengepack/validation.go`
  - `backend/internal/scoring/spec.go`
  - `cli/cmd/challenge_pack.go`
  - `web/content/docs/guides/write-a-challenge-pack.mdx`
- Confirm the skill does not claim unsupported `allowed_tool_kinds` values such as `shell`.
