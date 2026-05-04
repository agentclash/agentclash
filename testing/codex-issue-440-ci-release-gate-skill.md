# codex/issue-440-ci-release-gate-skill — Test Contract

## Functional Behavior
- Expand `web/content/agent-skills/agentclash-ci-release-gate/SKILL.md` from a thin placeholder into a source-faithful skill for wiring AgentClash release gates into CI.
- The skill must describe only commands, flags, manifest fields, JSON shapes, exit codes, artifacts, and GitHub Action inputs that exist in the codebase.
- The skill must align with the CLI implementation in `cli/cmd/ci.go`, `cli/cmd/ci_run.go`, `cli/cmd/ci_report.go`, `cli/cmd/ci_regressions.go`, and the local GitHub Action under `.github/actions/agentclash-ci/`.
- The skill must warn agents to validate manifests locally before remote validation, preserve hosted-only setup guidance, avoid printing tokens, and use `AGENTCLASH_TOKEN`, `AGENTCLASH_WORKSPACE`, and `AGENTCLASH_API_URL` exactly as the current CLI/action do.
- The final PR must not leave this `testing/*.md` contract artifact committed.

## Unit Tests
- `web/src/lib/docs.test.ts` must assert the CI release gate skill exists and contains high-risk source-fidelity anchors:
  - `agentclash ci validate .agentclash/ci.yaml --remote --json`
  - `agentclash ci should-run --manifest .agentclash/ci.yaml --base origin/main --head HEAD --json`
  - `agentclash ci run --manifest .agentclash/ci.yaml --json --artifact-dir agentclash-artifacts`
  - exact manifest keys such as `candidate.build.agent_build_id`, `candidate.deployment.runtime_profile_id`, `evaluation.challenge_pack_version_id`, `baseline.run_id`, `gate.fail_on`, and `regressions.promote_failures`
  - exact exit-code meanings for `0`, `1`, `2`, `3`, `10`, `20`, `30`, and `31`
  - exact report artifact filenames.

## Integration / Functional Tests
- Run the docs unit test for the docs registry and skill content.
- Run focused CLI CI tests to catch command/manifest drift.
- Run focused backend release-gate tests if present.
- Run `git diff --check`.

## Smoke Tests
- Review the full diff against the source files listed above.
- Run an external Claude review with `claude --dangerously-skip-permissions` and act on any source-fidelity findings.
- Open a PR against `main` using the review-checkpoint process.

## E2E Tests
- N/A — this is a documentation/skill change. The CLI and hosted AgentClash execution path are covered by existing focused CLI/backend tests and the existing blind skill harness in CI.

## Manual / cURL Tests
- N/A — no REST API behavior changes. Manual review consists of comparing the skill claims to the CLI/action/backend source and the generated PR diff.
