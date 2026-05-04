# codex/issue-449-challenge-pack-tools-sandbox-skill — Test Contract

## Functional Behavior
- Expand `web/content/agent-skills/challenge-pack-skills/agentclash-challenge-pack-tools-sandbox/SKILL.md` for issue #449.
- The skill must document exact source-backed challenge pack tool and sandbox YAML placement and shapes.
- It must distinguish challenge-pack top-level `tools.custom` from workspace infra tools/runtime profiles.
- It must explain `prompt_eval` restrictions and `native` requirements for tools/sandbox.
- It must list exact supported `version.tool_policy.allowed_tool_kinds`: `browser`, `build`, `data`, `file`, `network`; no `shell`.
- It must document custom tool fields: `name`, `parameters`, `implementation.primitive`, `implementation.args`, mock exception, placeholders, and secret-reference safety.
- It must document sandbox fields: `network_access`, `network_allowlist`, `env_vars`, `additional_packages`, `sandbox_template_id`, including CIDR/env/package validation rules.
- It must include validation commands using hosted production defaults.

## Unit Tests
- Add source-backed docs assertions in `web/src/lib/docs.test.ts` for the tools/sandbox skill.
- Assertions must cover: top-level `tools.custom`, `version.tool_policy.allowed_tool_kinds`, exact allowed kinds, no `shell`, `prompt_eval` restrictions, `${secrets.SECRET_KEY}`, `network_allowlist`, `additional_packages`, `sandbox_template_id`, and `agentclash challenge-pack validate`.

## Integration / Functional Tests
- Run `npm test -- src/lib/docs.test.ts` from `web/`.
- Run `go test ./internal/challengepack` from `backend/`.

## Smoke Tests
- Run `git diff --check`.
- Keyword sanity: `tools.custom`, `implementation.primitive`, `implementation.args`, `allowed_tool_kinds`, `browser`, `build`, `data`, `file`, `network`, `network_allowlist`, `env_vars`, `additional_packages`, `sandbox_template_id`, `prompt_eval`, `native`.

## E2E Tests
N/A locally — PR blind harness covers hosted self-containment.

## Manual / cURL Tests
- Manually review against `backend/internal/challengepack/bundle.go`, `backend/internal/challengepack/validation.go`, and template placeholder utilities.
- Manually review CLI validation command claims against `cli/cmd/challenge_pack.go`.
