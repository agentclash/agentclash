---
name: agentclash-dataset-workflows
description: Use when managing AgentClash datasets via CLI — create versions, import/export examples, run evals, CI gates, synthetic generation, trace import, candidate review, and regression suite sync.
metadata:
  agentclash.role: dataset
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Dataset Workflows

## Purpose
End-to-end dataset operations in a workspace: versioned example banks, eval runs against challenge packs, CI gating, synthetic generation, production trace import, and promotion into regression suites.

## Use When
- Building or curating labeled examples for prompt or agent evals.
- Gating merges on dataset eval pass rate vs a baseline.
- Importing OTEL/Braintrust/LangSmith/Phoenix/AgentClash traces as reviewable candidates.
- Syncing a dataset version into a linked regression suite.

## Do Not Use When
- The user only needs a one-off challenge-pack run — use `agentclash-eval-runner`.
- Prompt matrix experiments without a dataset artifact — use `agentclash-prompt-eval-playground`.
- Harness coding tasks — use `agentclash-agent-harness-setup`.

## Inputs Needed
- Workspace ID and dataset ID (create datasets via API/UI if none exist).
- For eval/gate: dataset version ID, challenge pack version ID, challenge key, deployment IDs.
- For gate: baseline ID and candidate run ID (or `--eval` to start eval inline).
- For generate: `--count`, `--provider-account`, `--model-alias`.

## Environment
```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
agentclash workspace use <WORKSPACE_ID>
agentclash dataset list
agentclash dataset get <DATASET_ID> --json
```

## Procedure
1. Inspect dataset and versions (`list`, `get`, `versions list`).
2. Import or export examples; optionally create a version snapshot.
3. Run a dataset eval or attach an existing run.
4. Gate with `dataset test` against a baseline (CI-friendly `--format junit`).
5. Optionally generate synthetic examples, import traces, promote candidates, sync regression suite.

## Commands

### Inspect and mutate examples
```bash
agentclash dataset list
agentclash dataset get <dataset-id>
agentclash dataset versions list <dataset-id>
agentclash dataset versions create <dataset-id> --label "v2-seeds"
agentclash dataset import <dataset-id> examples.jsonl
agentclash dataset export <dataset-id> --version <version-id> -o out.jsonl
agentclash dataset examples list <dataset-id> --version <version-id>
agentclash dataset examples add <dataset-id> --input '{"messages":[...]}' --expected '{"score":1}'
agentclash dataset examples update <dataset-id> <example-id> --expected-file expected.json
agentclash dataset examples delete <dataset-id> <example-id>
```

### Eval and CI gate
```bash
agentclash dataset eval <dataset-id> \
  --version <version-id> \
  --pack <pack-version-id> \
  --challenge <challenge-key> \
  --deployment <deployment-id>

agentclash dataset test <dataset-id> \
  --baseline <baseline-id> \
  --run <run-id> \
  --min-pass-rate 0.9 \
  --max-regressions 0 \
  --format junit

# Start eval then gate in one command
agentclash dataset test <dataset-id> \
  --eval \
  --version <version-id> \
  --pack <pack-version-id> \
  --challenge <challenge-key> \
  --deployment <deployment-id> \
  --baseline <baseline-id> \
  --timeout 30m
```

### Synthetic generation
```bash
agentclash dataset generate <dataset-id> \
  --count 50 \
  --provider-account <account-id> \
  --model-alias <alias-id> \
  --create-version \
  --version-label "synthetic-v1" \
  --follow
```

### Trace import and promotion
```bash
agentclash dataset import-traces <dataset-id> traces.json --source otel
agentclash dataset import-traces <dataset-id> --source agentclash --run <run-id> --run-agent <run-agent-id>
agentclash dataset trace-candidates list <dataset-id> --status pending
agentclash dataset promote <dataset-id> <candidate-id> --tag production --expected-file edited.json
```

### Regression suite sync
```bash
agentclash dataset sync-regression-suite <dataset-id> \
  --version <version-id> \
  --pack <pack-version-id> \
  --challenge <challenge-key> \
  --suite-name "Dataset regression bank"
```

## Expected Output
- Eval creates a run; gate returns pass/fail with regression counts.
- `dataset test --format junit` exits 0 on pass, 1 on gate failure (422).
- Generate with `--follow` polls job until completion.

## Failure Modes
- Gate without `--baseline` → required.
- Gate without `--run` and without `--eval` → provide one.
- Generate missing provider/model → all three of count, provider-account, model-alias required.
- Sync regression without version/pack/challenge → all three flags required.

## Safety Notes
- Trace imports may contain production data — apply `--redaction` JSON when importing sensitive metadata.
- Baseline comparisons affect release gates — confirm baseline ID before CI integration.
- Exported JSONL may include prompts with secrets — scrub before sharing externally.

## Report Back Format
```text
Dataset: <id>
Version: <version-id or n/a>
Eval run: <run-id or n/a>
Gate: <pass/fail> — pass rate <x>, regressions <n>
Candidates: <pending count or n/a>
Regression suite: <suite-id or n/a>
Next: agentclash run scorecard <run-id>
```

## Related Skills
- `agentclash-hub`
- `agentclash-eval-runner`
- `agentclash-regression-flywheel`
- `agentclash-ci-release-gate`
- `agentclash-scorecard-reader`
- `agentclash-prompt-eval-playground`

## Related Docs
- `/docs-md/guides/datasets-overview`
- `/docs-md/guides/ci-cd-workflow-recipes`
- `/docs-md/reference/cli`
