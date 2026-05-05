# codex/pr-comment-live-smoke — Test Contract

## Functional Behavior
- A fresh pull request exercises the merged `agentclash/agentclash/.github/actions/agentclash-ci@main` action against real GitHub pull request context.
- The smoke workflow grants `issues: write` and `pull-requests: read`, so the action can post or update its sticky AgentClash PR comment.
- The action intentionally hits the setup-failure path without requiring AgentClash secrets or a real manifest, then posts an errored sticky comment that points reviewers to the GitHub Actions log.
- The smoke job remains non-blocking for the PR by using `continue-on-error`, because this PR exists to verify comment delivery rather than to merge a failing AgentClash gate.

## Unit Tests
- N/A — this PR is a live integration smoke for the already-tested action.

## Integration / Functional Tests
- The workflow calls `agentclash/agentclash/.github/actions/agentclash-ci@main` rather than the PR-local action implementation.
- The workflow leaves `install-cli: false`, `remote-validate: false`, and a missing manifest path so the merged action's early-failure comment path is exercised without external AgentClash credentials.

## Smoke Tests
- `ruby -e 'require "yaml"; YAML.load_file(".github/workflows/frontend.yml"); puts "workflow yaml ok"'` passes.
- After the PR opens, GitHub checks include `AgentClash PR Comment Smoke`.
- The PR receives one AgentClash sticky comment containing `AgentClash CI: Errored` and `action_failed_before_ci_run`.

## E2E Tests
- Opening the PR is the E2E test. Verify the comment appears on GitHub and repeated pushes update the same hidden-marker comment.

## Manual / cURL Tests
```bash
ruby -e 'require "yaml"; YAML.load_file(".github/workflows/frontend.yml"); puts "workflow yaml ok"'
gh pr checks <pr-number>
gh pr view <pr-number> --comments
# Expected: workflow YAML parses; checks include the smoke job; PR comments include the AgentClash errored sticky comment.
```
