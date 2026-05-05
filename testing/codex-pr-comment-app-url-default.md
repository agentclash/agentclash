# codex/pr-comment-app-url-default — Test Contract

## Functional Behavior
- AgentClash CI PR comments default to a reachable production web app origin.
- The default app URL must be `https://www.agentclash.dev`, because live smoke testing showed `https://app.agentclash.dev` does not resolve while the same workspace routes on `www.agentclash.dev` match the deployed Next.js routes.
- Existing `app-url` overrides continue to work.
- Generated workspace, run, comparison, failures, scorecard, replay, and regression-case paths do not change.

## Unit Tests
- PR comment formatter tests expect default generated links on `https://www.agentclash.dev`.
- Custom `app-url` formatter test continues to prove override behavior.

## Integration / Functional Tests
- Action metadata default for `app-url` is `https://www.agentclash.dev`.
- The shell wrapper fallback default matches the action metadata and Python helper default.
- README and CI/CD guide document the same default.

## Smoke Tests
- `python3 -m unittest discover -s .github/actions/agentclash-ci -p comment_test.py` passes.
- `python3 -m py_compile .github/actions/agentclash-ci/comment.py .github/actions/agentclash-ci/comment_test.py` passes.
- `bash -n .github/actions/agentclash-ci/run.sh` passes.
- Action metadata parses as YAML.
- `git diff --check` passes.

## E2E Tests
- Follow-up demo PR comment uses `https://www.agentclash.dev` links, and each link route returns the expected deployed Next.js matched route or auth redirect.

## Manual / cURL Tests
```bash
python3 -m unittest discover -s .github/actions/agentclash-ci -p comment_test.py
python3 -m py_compile .github/actions/agentclash-ci/comment.py .github/actions/agentclash-ci/comment_test.py
bash -n .github/actions/agentclash-ci/run.sh
ruby -e 'require "yaml"; YAML.load_file(".github/actions/agentclash-ci/action.yml"); puts "action yaml ok"'
git diff --check
# Expected: all commands succeed.
```
