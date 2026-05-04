---
name: agentclash-ci-release-gate
description: Use when comparing AgentClash candidate runs against baselines, evaluating release gates, or adding CI/CD checks that fail on regressions.
metadata:
  agentclash.role: ci
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash CI Release Gate

## Purpose
Wire AgentClash manifest-based comparisons into release decisions and CI checks.

## Use When
- A user wants to gate an agent change in CI/CD.
- A user wants to compare a candidate run against a baseline.
- A release gate should block regressions.
- A GitHub Actions workflow needs AgentClash commands and exit-code behavior.

## Do Not Use When
- The user still needs to author the challenge pack.
- The user only wants a narrative readout from an existing scorecard.

## Inputs Needed
- Repo-tracked CI manifest path, usually `.agentclash/ci.yaml`.
- AgentClash workspace secret name.
- AgentClash API token secret name.
- Candidate agent build and deployment resources referenced by the manifest.
- Evaluation workload: challenge pack version, optional input set, and optional regression suites/cases.
- Baseline strategy: locked `baseline.run_id` for PRs, or an intentional `baseline.deployment_id` selector.
- Gate policy and `regressions.promote_failures` mode.

## Environment
For CI against production:

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
export AGENTCLASH_TOKEN="<token>"
export AGENTCLASH_WORKSPACE="<workspace-id>"
```

## Procedure
1. Confirm the manifest names the agent revision, workload, baseline, gate, and regression promotion policy.
2. Validate the manifest locally, then with `--remote` when workspace credentials are available.
3. Decide whether the change should run from manifest paths and labels.
4. Resolve the baseline so reviewers know the exact accepted run.
5. Run `agentclash ci run` or the reusable GitHub Action.
6. In CI, fail the job when the gate command exits non-zero.
7. Report the manifest, baseline, candidate, gate verdict, exit code, summaries, and artifact paths.

## Commands
```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
export AGENTCLASH_TOKEN="<token>"
export AGENTCLASH_WORKSPACE="<workspace-id>"

agentclash ci validate .agentclash/ci.yaml
agentclash ci validate .agentclash/ci.yaml --remote --json
agentclash ci should-run --manifest .agentclash/ci.yaml --base origin/main --head HEAD --json
agentclash ci baseline --manifest .agentclash/ci.yaml --json
agentclash ci run --manifest .agentclash/ci.yaml --json --artifact-dir agentclash-artifacts
```

## GitHub Actions Sketch
```yaml
name: AgentClash gate
on:
  pull_request:

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
      - uses: actions/upload-artifact@v4
        if: always() && steps.agentclash.outputs['should-run'] == 'true'
        with:
          name: agentclash-ci
          path: |
            ${{ steps.agentclash.outputs.result-file }}
            ${{ steps.agentclash.outputs.artifact-dir }}/*.json
```

## Expected Output
- `ci run` prints a JSON envelope with baseline, candidate, gate verdict, exit code, reports, and regression promotion outcomes.
- GitHub Actions receives a step summary when `$GITHUB_STEP_SUMMARY` is set.
- JSON artifacts include `result.json`, `run.json`, `scorecard.json`, `comparison.json`, and `gate.json` when available.
- CI fails when the gate detects a blocking regression, times out, or hits an API/setup error.
- The report includes exact commands or links to inspect the comparison manually.

## Failure Modes
- Missing token in CI: check secret name and workspace access.
- Manifest validation fails: fix the repo-tracked manifest before running the gate.
- Remote validation fails: verify workspace IDs, challenge pack versions, deployment resources, and baseline visibility.
- `ci should-run` skips unexpectedly: inspect `trigger.paths`, labels, checkout `fetch-depth`, and base/head refs.
- Candidate run is incomplete or slow: adjust `--timeout`, `--poll-interval`, or run with `follow: true`.
- `auto_on_main` regression promotion is blocked: ensure the workflow is on the default branch, not a pull request event.

## Safety Notes
- Confirm before changing a shared baseline or switching from `proposed` to `auto_on_main`.
- Do not echo tokens in CI logs.
- Treat production release gates as blocking unless the user explicitly overrides them.
- Prefer `baseline.run_id` for pull request gates so baseline movement is reviewed.

## Report Back Format
```text
Manifest: <path>
Should run: <true/false + reason>
Baseline: <run-id or selector>
Candidate run: <run-id>
Verdict: <pass/fail>
Exit code: <code + meaning>
Artifacts: <result.json/artifact-dir>
Regression candidates: <created/existing/skipped/blocked summary>
Next command: <command>
```

## Related Docs
- `/docs-md/guides/ci-cd-agent-gates`
- `/docs-md/guides/ci-cd-workload-recipes`
- `/docs-md/challenge-packs/eval-workflows-and-gates`
