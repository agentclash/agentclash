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
Wire AgentClash comparisons into release decisions and CI checks.

## Use When
- A user wants to compare a candidate run against a baseline.
- A release gate should block regressions.
- A GitHub Actions workflow needs AgentClash commands and exit-code behavior.

## Do Not Use When
- The user still needs to author the challenge pack.
- The user only wants a narrative readout from an existing scorecard.

## Inputs Needed
- Baseline run or configured baseline.
- Candidate run or command to create one.
- Release gate ID or gate policy.
- CI secret names for `AGENTCLASH_TOKEN`, workspace, and API URL.

## Environment
For CI against staging:

```bash
export AGENTCLASH_API_URL="https://staging-api.agentclash.dev"
export AGENTCLASH_TOKEN="<token>"
export AGENTCLASH_WORKSPACE="<workspace-id>"
```

## Procedure
1. Confirm which baseline and candidate should be compared.
2. Run or fetch the candidate result.
3. Evaluate the comparison or release gate.
4. In CI, fail the job when the gate command exits non-zero.
5. Report the candidate, baseline, gate verdict, and linkable follow-up commands.

## Commands
```bash
export AGENTCLASH_API_URL="https://staging-api.agentclash.dev"
agentclash baseline show
agentclash baseline set <run-id>
agentclash compare runs <baseline-run-id> <candidate-run-id>
agentclash compare gate <gate-id> --candidate <candidate-run-id>
agentclash release-gate list
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
      - uses: actions/setup-node@v4
        with:
          node-version: "22"
      - run: npm i -g agentclash
      - run: agentclash compare gate "$AGENTCLASH_GATE_ID" --candidate "$AGENTCLASH_CANDIDATE_RUN"
        env:
          AGENTCLASH_API_URL: https://staging-api.agentclash.dev
          AGENTCLASH_TOKEN: ${{ secrets.AGENTCLASH_TOKEN }}
          AGENTCLASH_WORKSPACE: ${{ secrets.AGENTCLASH_WORKSPACE }}
          AGENTCLASH_GATE_ID: ${{ vars.AGENTCLASH_GATE_ID }}
          AGENTCLASH_CANDIDATE_RUN: ${{ vars.AGENTCLASH_CANDIDATE_RUN }}
```

## Expected Output
- Comparison commands print pass/fail or verdict details.
- CI fails when the gate detects a blocking regression.
- The report includes exact commands to inspect the comparison manually.

## Failure Modes
- Missing token in CI: check secret name and workspace access.
- Candidate run is incomplete: wait for completion or rerun with `--follow`.
- Gate ID points at the wrong workspace: list gates under the same workspace.

## Safety Notes
- Confirm before changing the shared baseline.
- Do not echo tokens in CI logs.
- Treat production release gates as blocking unless the user explicitly overrides them.

## Report Back Format
```text
Baseline: <run-id>
Candidate: <run-id>
Gate: <gate-id>
Verdict: <pass/fail>
CI behavior: <exit code summary>
Next command: <command>
```

## Related Docs
- `/docs-md/reference/cli`
- `/docs-md/guides/interpret-results`
