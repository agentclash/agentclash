# codex/pr-comment-links — Test Contract

## Functional Behavior
- AgentClash PR comments must turn run evidence into direct AgentClash UI links instead of leaving reviewers with raw IDs only.
- When `workspace_id`, candidate run ID, baseline run ID, and candidate run-agent ID are present, the comment includes links for:
  - candidate run
  - baseline run
  - baseline-vs-candidate comparison
  - candidate failures
  - candidate scorecard
  - candidate replay
- When regression promotions include created or existing cases with suite and case IDs, the comment links to those regression cases.
- Links are generated against a configurable AgentClash app base URL and default to `https://app.agentclash.dev`.
- Unsafe/non-HTTP URLs must not be rendered as Markdown links.
- Comment posting remains best-effort and must not alter the original AgentClash CI exit code.
- GitHub workflow examples use the permission that live testing proved works for PR comments: `pull-requests: write`.

## Unit Tests
- Formatter test for a failing result without API-provided `run_url` asserts the comment includes workspace-scoped candidate, baseline, comparison, failures, scorecard, replay, and regression-case links.
- Formatter test for explicit safe API URLs asserts they are respected where appropriate.
- Formatter test for unsafe URLs asserts `javascript:` or similar values are not rendered as links.
- Existing formatting, skipped, context, create, update, and graceful-skip tests continue to pass.

## Integration / Functional Tests
- `action.yml` exposes an optional app URL input and passes it into `comment.py`.
- `run.sh` passes the app URL to the helper in normal, skipped, and early-error paths.
- Action README and CI/CD guide document the linked PR comment behavior and correct `pull-requests: write` permission.

## Smoke Tests
- `python3 -m unittest discover -s .github/actions/agentclash-ci -p comment_test.py` passes.
- `python3 -m py_compile .github/actions/agentclash-ci/comment.py .github/actions/agentclash-ci/comment_test.py` passes.
- `bash -n .github/actions/agentclash-ci/run.sh` passes.
- Action metadata parses as YAML.
- `git diff --check` passes.

## E2E Tests
- After this PR merges, a follow-up demo repo PR must trigger AgentClash CI and the PR comment links must open in the AgentClash app to the expected run, comparison, failures, scorecard, replay, and regression-case views.

## Manual / cURL Tests
```bash
python3 -m unittest discover -s .github/actions/agentclash-ci -p comment_test.py
python3 -m py_compile .github/actions/agentclash-ci/comment.py .github/actions/agentclash-ci/comment_test.py
bash -n .github/actions/agentclash-ci/run.sh
ruby -e 'require "yaml"; YAML.load_file(".github/actions/agentclash-ci/action.yml"); puts "action yaml ok"'
git diff --check
# Expected: all commands succeed.
```
