# codex/issue-453-challenge-pack-validation-publish-skill — Test Contract

## Functional Behavior
- Expand `web/content/agent-skills/challenge-pack-skills/agentclash-challenge-pack-validation-publish/SKILL.md` into a source-backed skill for validating, fixing, publishing, and reporting AgentClash challenge packs.
- Document the actual CLI commands: `agentclash challenge-pack validate <file> --json`, `agentclash challenge-pack publish <file> --json`, `agentclash challenge-pack list --json`, and follow-up run commands that exist today.
- State that validation and publish use hosted workspace APIs, require auth plus a workspace, and default examples to `https://api.agentclash.dev`.
- Document validation response shape, publish response fields, relevant failure modes, and safety notes without inventing unsupported flags or fields.
- Preserve related skill links in dependency order.

## Unit Tests
- Update `web/src/lib/docs.test.ts` so generated docs assert source-backed validation/publish details are present.

## Integration / Functional Tests
- Run `npm test -- src/lib/docs.test.ts` from `web/`.
- Run relevant Go tests for challenge-pack parsing/API behavior.

## Smoke Tests
- Run `git diff --check`.
- Inspect the final diff to ensure no temporary testing contract remains in the PR.

## E2E Tests
- N/A — this is documentation-only skill content, with behavior pinned to existing CLI/API source.

## Manual / cURL Tests
- N/A — do not publish a real challenge pack from the autonomous loop. Verify commands against checked-in CLI/backend code instead.
