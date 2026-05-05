# AgentClash CI Gate Action

Run a manifest-based AgentClash CI gate from GitHub Actions.

This action is a thin wrapper around the published `agentclash` CLI:

1. optionally install `agentclash` from npm
2. validate the repo-tracked CI manifest
3. run `agentclash ci should-run`
4. run `agentclash ci run` when the manifest trigger matches
5. expose result paths and gate outputs for artifact upload or downstream steps

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
| `manifest` | `.agentclash/ci.yaml` | Path to the AgentClash CI manifest. |
| `token` | `AGENTCLASH_TOKEN` | AgentClash API token. |
| `workspace` | `AGENTCLASH_WORKSPACE` | AgentClash workspace ID. |
| `api-url` | `https://api.agentclash.dev` | AgentClash API base URL. |
| `install-cli` | `true` | Install `agentclash` from npm before running. |
| `cli-version` | `latest` | npm version or tag for the `agentclash` package. |
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
