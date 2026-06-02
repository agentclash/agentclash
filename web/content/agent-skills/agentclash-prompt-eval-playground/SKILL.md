---
name: agentclash-prompt-eval-playground
description: Use when scaffolding, validating, or running prompt eval YAML configs and managing playground experiments, test cases, and prompt variants via the AgentClash CLI.
metadata:
  agentclash.role: prompt-eval
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Prompt Eval Playground

## Purpose
Local-first prompt evaluation workflows: scaffold `.agentclash/prompt-eval.yaml`, validate locally or remotely, compile configs into playground experiments, fetch results, import Promptfoo subsets, and manage playground CRUD from the CLI.

## Use When
- Comparing prompt variants or model aliases on fixed test cases before a full challenge-pack run.
- CI needs `prompt-eval validate --ci --remote` or `prompt-eval run --ci --follow`.
- Migrating a Promptfoo config into AgentClash format.
- Inspecting or rerunning playground experiments linked to prompt eval runs.

## Do Not Use When
- The eval is a full agent deployment on a challenge pack — use `agentclash-eval-runner`.
- The workflow is dataset versioning and baseline gates — use `agentclash-dataset-workflows`.
- The user needs harness repo tasks — use `agentclash-agent-harness-setup`.

## Inputs Needed
- Workspace with provider accounts and deployments referenced in the YAML.
- Prompt eval config path (default `.agentclash/prompt-eval.yaml`).
- For playground commands: playground ID from list/create output.
- Optional Promptfoo YAML for import.

## Environment
```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
agentclash workspace use <WORKSPACE_ID>
agentclash prompt-eval init
agentclash prompt-eval validate --remote
```

## Procedure
1. Scaffold or import a prompt eval config.
2. Validate locally; add `--remote` (and `--ci` in pipelines) for workspace-safe checks.
3. Run to compile and launch playground experiments; use `--follow` in CI.
4. Fetch results by experiment ID; compare assertion pass rates vs threshold.
5. Use `playground` subcommands for manual experiment CRUD when not driven by YAML.

## Commands

### Prompt eval lifecycle
```bash
agentclash prompt-eval init
agentclash prompt-eval init my-eval.yaml --name "Refund prompts"
agentclash prompt-eval validate
agentclash prompt-eval validate --remote --ci
agentclash prompt-eval run --follow --ci --threshold 0.95
agentclash prompt-eval results <experiment-id> --threshold 0.95
agentclash prompt-eval import-promptfoo promptfoo.yaml --out .agentclash/prompt-eval.yaml
```

Useful flags on `run`: `--max-cases`, `--poll-interval`, `--timeout`, `--threshold`.

### Playground CRUD (alias `pg`)
```bash
agentclash playground list
agentclash playground create --name "Refund A/B" --description "Tone variants"
agentclash playground get <playground-id>
agentclash playground update <playground-id> --name "Updated name"
agentclash playground delete <playground-id>

agentclash playground test-cases list <playground-id>
agentclash playground test-cases create <playground-id> --input '{"messages":[...]}'
agentclash playground test-cases update <playground-id> <case-id> --expected-file out.json
agentclash playground test-cases delete <playground-id> <case-id>

agentclash playground experiments list <playground-id>
agentclash playground experiments create <playground-id> --prompt-variant <variant-id>
agentclash playground experiments get <playground-id> <experiment-id>
agentclash playground experiments run <playground-id> <experiment-id> --follow
```

Config default path: `.agentclash/prompt-eval.yaml`. Schema version is stamped on `init`.

## Expected Output
- Validate prints errors/warnings; exits non-zero when invalid.
- Run compiles cases into experiments; `--follow` waits for completion.
- Results envelope includes assertion pass rate; sub-threshold exits non-zero in CI mode.

## Failure Modes
- `--ci` without `--remote` on validate → CI-safe remote checks required.
- Missing workspace references in YAML → fix provider accounts/deployments or run `--remote` validate.
- Import Promptfoo with unsupported features → use `--lossy` or edit converted YAML manually.
- Experiment not found → list experiments on the playground first.

## Safety Notes
- Remote validate/run touches live provider accounts — use CI workspace tokens, not personal prod keys in shared logs.
- Promptfoo import may drop unsupported assertions — review converted YAML before merging.
- Playground experiments incur model cost — cap `--max-cases` in exploratory runs.

## Report Back Format
```text
Config: <path>
Validate: <pass/fail> (<error count> errors)
Experiment: <id or n/a>
Pass rate: <rate vs threshold>
Playground: <id or n/a>
Next: agentclash eval-runner ... OR prompt-eval results <id>
```

## Related Skills
- `agentclash-hub`
- `agentclash-cli-setup`
- `agentclash-eval-runner`
- `agentclash-dataset-workflows`
- `agentclash-scorecard-reader`
- `agentclash-regression-flywheel`

## Related Docs
- `/docs-md/guides/use-with-ai-tools`
- `/docs-md/reference/cli`
- `/docs-md/getting-started/first-eval`
