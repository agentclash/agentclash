# AgentClash CI Gate Action

Run a manifest-based AgentClash CI gate or prompt-eval gate from GitHub Actions.

This action is a thin wrapper around the `agentclash` CLI:

1. optionally install `agentclash` from npm
2. verify the installed CLI exposes the required CI commands
3. fall back to the action checkout's Go source when npm has not caught up yet
4. validate the repo-tracked CI manifest
5. run `agentclash ci should-run`
6. run `agentclash ci run` when the manifest trigger matches
7. post or update a sticky structured PR comment when pull request context and permissions are available
8. expose result paths and gate outputs for artifact upload or downstream steps

## Example

```yaml
name: AgentClash gate

on:
  pull_request:
    paths:
      - ".agentclash/**"
      - "prompts/**"
      - "tools/**"

jobs:
  agentclash:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write

    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-node@v4
        with:
          node-version: "22"

      - id: agentclash
        uses: agentclash/agentclash/.github/actions/agentclash-ci@main
        with:
          token: ${{ secrets.AGENTCLASH_TOKEN }}
          workspace: ${{ secrets.AGENTCLASH_WORKSPACE }}
          manifest: .agentclash/ci.yaml

      - name: Upload AgentClash gate artifacts
        if: always() && steps.agentclash.outputs['should-run'] == 'true'
        uses: actions/upload-artifact@v4
        with:
          name: agentclash-ci
          path: |
            ${{ steps.agentclash.outputs.result-file }}
            ${{ steps.agentclash.outputs.artifact-dir }}/*.json
```

## Inputs

| Input | Default | Description |
| --- | --- | --- |
| `mode` | `ci` | Gate mode: `ci` for manifest-based agent CI, or `prompt-eval` for prompt eval configs. |
| `manifest` | `.agentclash/ci.yaml` | Path to the AgentClash CI manifest. |
| `prompt-eval-config` | `.agentclash/prompt-eval.yaml` | Path to the prompt eval config when `mode: prompt-eval`. |
| `prompt-eval-watch-paths` | `prompts/**`, `agents/**`, `tools/**`, `.agentclash/**` | Newline-separated globs that trigger prompt-eval mode in addition to the config path. |
| `prompt-eval-threshold` | config default | Optional assertion pass-rate threshold override passed to `prompt-eval run`. |
| `token` | `AGENTCLASH_TOKEN` | AgentClash API token. |
| `workspace` | `AGENTCLASH_WORKSPACE` | AgentClash workspace ID. |
| `api-url` | `https://api.agentclash.dev` | AgentClash API base URL. |
| `app-url` | `https://agentclash.dev` | AgentClash app base URL used for PR comment links. |
| `install-cli` | `true` | Install `agentclash` from npm before running. |
| `cli-version` | `latest` | npm version or tag for the `agentclash` package. |
| `source-fallback` | `true` | Use the action checkout's Go source when the installed CLI does not expose `ci should-run` and `ci run`. When enabled, the action sets up Go with `go-version` before resolving the CLI. |
| `go-version` | `1.25.x` | Go version used by the source fallback. |
| `remote-validate` | `true` | Validate manifest resource IDs against the API. |
| `skip-if-unmatched` | `true` | Skip `ci run` when manifest paths and labels do not match. |
| `base` | pull request base branch | Base ref for `ci should-run`. |
| `head` | `HEAD` | Head ref for `ci should-run`. |
| `changed-files` | empty | Newline-separated files to pass directly to `ci should-run`. |
| `labels` | GitHub event labels | Comma-separated pull request labels. Override only when the workflow cannot rely on the event payload. |
| `artifact-dir` | `agentclash-artifacts` | Directory for `ci run` JSON artifacts. |
| `result-file` | `agentclash-ci-result.json` | Top-level `ci run` JSON result path. |
| `timeout` | CLI default | Optional `ci run` timeout, for example `30m` or `0`. |
| `poll-interval` | CLI default | Optional `ci run` poll interval, for example `5s`. |
| `follow` | `false` | Stream run events while waiting. |
| `default-branch` | auto-detected | Default branch metadata override for `auto_on_main` regression promotion. |
| `pr-comment` | `true` | Post or update a sticky structured PR comment when pull request context is available. |
| `github-token` | `github.token` | GitHub token used for PR comments. Override only for custom permission setups. |

## Outputs

| Output | Description |
| --- | --- |
| `should-run` | `true` when the manifest trigger matched and `ci run` was attempted. |
| `skip-reason` | Reason returned by `ci should-run` when skipped. |
| `run-id` | Candidate AgentClash run ID. |
| `gate-verdict` | Release gate verdict from `ci run`. |
| `exit-code` | Exit code returned by `agentclash ci run`, or `0` when skipped. |
| `result-file` | Path to the top-level result JSON. |
| `artifact-dir` | Directory containing stable AgentClash JSON artifacts. |

The action preserves the CLI exit code. A blocking AgentClash gate fails the job with the same code that `agentclash ci run` returned.

## Pull Request Comments

When `pr-comment` is enabled, the action posts a single sticky comment on pull requests. The comment summarizes the gate verdict, failure reason, candidate and baseline runs, score deltas, regression tracking outcome, and next actions. When run metadata is available, it also links directly to the AgentClash candidate run, baseline run, comparison, failures, scorecard, replay, and regression cases. If the action fails before a candidate run is created, the comment reports an errored setup state and points reviewers at the GitHub Actions log. Later pushes update the existing AgentClash comment instead of creating a new one.

Commenting is best-effort. If the workflow is not running on a pull request, the token is missing, or the token lacks comment permissions, the action logs a notice and preserves the original AgentClash exit code. For GitHub-hosted PR comments, grant `pull-requests: write` in the job permissions.

## Prompt Eval Mode

Prompt eval mode runs `agentclash prompt-eval validate <config> --remote --ci`, checks whether the prompt eval config or watch paths changed, then runs `agentclash prompt-eval run <config> --json --follow --ci`. Gate failures exit `3`, keep the JSON result parseable, and post a sticky prompt-eval PR comment with failed assertions and AgentClash playground/experiment links.

```yaml
name: AgentClash prompt eval

on:
  pull_request:
    paths:
      - ".agentclash/prompt-eval.yaml"
      - "prompts/**"
      - "tools/**"

concurrency:
  group: agentclash-prompt-eval-${{ github.repository }}-${{ github.workflow }}-${{ vars.AGENTCLASH_WORKSPACE }}-.agentclash-prompt-eval-yaml
  cancel-in-progress: false

jobs:
  prompt-eval:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write

    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-node@v4
        with:
          node-version: "22"

      - uses: agentclash/agentclash/.github/actions/agentclash-ci@main
        with:
          mode: prompt-eval
          token: ${{ secrets.AGENTCLASH_TOKEN }}
          workspace: ${{ vars.AGENTCLASH_WORKSPACE }}
          prompt-eval-config: .agentclash/prompt-eval.yaml
```

The concurrency group intentionally excludes `github.ref`. Prompt eval V1 updates shared AgentClash playground resources by workspace and config path, so every PR targeting the same workspace/config should serialize instead of racing.
