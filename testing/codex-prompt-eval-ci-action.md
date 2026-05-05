# Codex Prompt Eval CI Action - Test Contract

Issue: #592
Branch: `codex/prompt-eval-ci-action`

## Functional Behavior

- The composite action accepts `mode: ci` (default) and `mode: prompt-eval`.
- Existing manifest-based CI behavior remains unchanged for default mode.
- In prompt-eval mode, the action resolves a CLI that supports `prompt-eval validate` and `prompt-eval run`.
- In prompt-eval mode, the action validates `.agentclash/prompt-eval.yaml` by default, or `prompt-eval-config` when provided.
- Prompt-eval remote validation uses `agentclash prompt-eval validate <config> --remote --ci` when `remote-validate` is true, otherwise local `--ci` validation.
- Prompt-eval change detection runs from provided `changed-files` or git base/head. It should run when the prompt-eval config or configured prompt-eval watch paths changed, and skip when unmatched if `skip-if-unmatched` is true.
- Prompt-eval execution invokes `agentclash prompt-eval run <config> --json --follow --ci`, plus optional timeout, poll interval, and threshold inputs.
- The action parses and preserves structured output when the CLI exits `3` for a gate failure, posts a PR comment, writes outputs, and exits `3`.
- Prompt-eval infrastructure/runtime failures preserve their CLI exit codes and still post a structured comment when JSON exists or an early error result can be synthesized.
- The prompt-eval PR comment is sticky and uses `<!-- agentclash:prompt-eval -->`. Re-runs update the existing comment rather than creating duplicates.
- The prompt-eval comment includes pass/fail summary, execution errors, top failed assertions, AgentClash UI links, a local reproduction command, and redacted/truncated snippets.

## Unit Tests

- `run_test.py` covers prompt-eval mode should-run from config path changes.
- `run_test.py` covers prompt-eval skip when changed files do not match.
- `run_test.py` covers prompt-eval CLI pass and captures result outputs.
- `run_test.py` covers prompt-eval gate failure exit `3` with a result file and comment helper invocation.
- `run_test.py` covers prompt-eval infrastructure failure exit `4`.
- `comment_test.py` covers prompt-eval comment formatting, links, failed assertions, redaction, reproduction command, and idempotent update marker.

## Integration / Functional Tests

- Run `.github/actions/agentclash-ci/run_test.py`.
- Run `.github/actions/agentclash-ci/comment_test.py`.

## Smoke Tests

- Run the action shell tests with fake CLIs; no hosted AgentClash call is required.
- Run shell syntax checks for `.github/actions/agentclash-ci/run.sh`.

## E2E Tests

N/A - live GitHub PR E2E is deferred until a stable hosted prompt-eval fixture workspace exists.

## Manual / cURL Tests

```bash
cd .github/actions/agentclash-ci
python3 -m unittest run_test.py comment_test.py
bash -n run.sh
```
