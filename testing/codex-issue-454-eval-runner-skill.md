# codex/issue-454-eval-runner-skill — Test Contract

## Functional Behavior
- Expand `web/content/agent-skills/agentclash-eval-runner/SKILL.md` into a source-backed skill for creating, following, inspecting, and reporting AgentClash eval runs.
- Document the actual `agentclash eval start` flags, selector behavior, input-set behavior, scope behavior, follow behavior, repetitions/eval-session path, and reporting commands.
- Document the actual `agentclash run create` lower-level ID-first flow and distinguish `--deployment` from `--deployments`.
- Use hosted production examples with `https://api.agentclash.dev`.
- Avoid unsupported commands, flags, output fields, or claims about offline execution.

## Unit Tests
- Update `web/src/lib/docs.test.ts` so generated docs assert the source-backed eval-runner details are present.

## Integration / Functional Tests
- Run `npm test -- src/lib/docs.test.ts` from `web/`.
- Run focused CLI tests for eval/run creation behavior.

## Smoke Tests
- Run `git diff --check`.
- Inspect the final branch diff to ensure no `testing/*.md` artifact remains.

## E2E Tests
- N/A — this is documentation-only skill content and should not create real hosted runs from the autonomous loop.

## Manual / cURL Tests
- N/A — do not create production eval runs from the autonomous loop. Verify commands against checked-in CLI/backend code.
