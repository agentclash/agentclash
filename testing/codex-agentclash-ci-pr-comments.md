# codex/agentclash-ci-pr-comments — Test Contract

## Functional Behavior
- The AgentClash GitHub composite action posts or updates one sticky PR comment after `agentclash ci run` when PR commenting is enabled.
- The comment is structured for reviewers: verdict, failure reason, candidate run, baseline run, score deltas, regression promotion summary, artifact/result pointers, and next actions.
- The comment is updated in place using a stable hidden marker, so repeated pushes do not create comment spam.
- The action must not fail the CI gate when comment posting is unavailable because of missing GitHub token, missing pull request context, fork permissions, or GitHub API errors; it should emit a notice and preserve the original AgentClash exit code.
- Commenting is configurable with an action input and defaults on for pull request workflows.
- Skipped runs may update the sticky comment with a skipped state when pull request context is available.

## Unit Tests
- A formatter test builds a failed-gate comment from a representative `agentclash ci run` JSON result and asserts it includes the verdict, reason, dimension deltas, candidate/baseline IDs, regression summary, and hidden sticky marker.
- A formatter test builds a skipped comment from `ci should-run` JSON and asserts it explains the skip reason.
- An upsert test updates an existing marked comment instead of creating a new one.
- An upsert test creates a new comment when no marked comment exists.
- A context test finds the pull request number from result CI metadata, GitHub refs, and event payload fallback.
- A permission-safety test returns a graceful skip/error result when the token or PR context is missing.

## Integration / Functional Tests
- The composite action wires the helper after `ci run` and before exiting with the original gate status.
- The helper receives the action manifest path, result JSON, should-run JSON, repository, event path, API URL, and token through action inputs/environment without printing secrets.

## Smoke Tests
- `python3 -m unittest discover -s .github/actions/agentclash-ci -p comment_test.py` passes.
- `bash -n .github/actions/agentclash-ci/run.sh` passes.
- Action metadata remains valid YAML.

## E2E Tests
- N/A in this PR — live GitHub posting requires a real pull request token and repository permissions. The unit tests cover the API calls with a fake transport, and a follow-up demo PR can verify live sticky comments.

## Manual / cURL Tests
```bash
python3 -m unittest discover -s .github/actions/agentclash-ci -p comment_test.py
bash -n .github/actions/agentclash-ci/run.sh
ruby -e 'require "yaml"; YAML.load_file(".github/actions/agentclash-ci/action.yml"); puts "ok"'
# Expected: all commands succeed.
```
