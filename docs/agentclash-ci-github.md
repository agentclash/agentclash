# AgentClash GitHub CI

AgentClash CI turns a pull request into an agent regression gate. The product UI
generates a repository manifest at `.agentclash/ci.yaml` and a GitHub Actions
workflow at `.github/workflows/agentclash.yml`; the CLI then runs the configured
workspace resources from the PR.

## What Runs

AgentClash CI runs when the workflow receives a pull request event and either:

- a changed file matches the manifest `triggers.paths` entries, such as
  `agents/**`, `prompts/**`, `.agentclash/**`, or `.github/workflows/agentclash.yml`
- the PR has one of the force-run labels in `triggers.labels`, such as
  `agentclash/eval`

The manifest points at workspace resources that define the gate:

- candidate agent build and runtime profile
- optional provider account or model alias override
- eval pack version and optional input set
- regression suites
- locked baseline run or deployment-derived baseline
- release gate policy and regression-promotion behavior

## Setup From The UI

1. Open `https://agentclash.dev`.
2. Choose a workspace, then open **CI setup**.
3. Connect or select an installed GitHub repository.
4. Choose the agent build, runtime profile, eval pack, baseline, and gate
   policy.
5. Save the setup as a CI profile when the configuration should be reused later.
6. Click **Open setup PR** to create a draft PR with the generated manifest and
   workflow.
7. If target files already exist, review the conflict list, then confirm
   overwrite to open a setup PR that replaces those files on a branch.

The repository also needs GitHub Actions secrets:

```text
AGENTCLASH_TOKEN
AGENTCLASH_WORKSPACE
```

Use the hosted backend unless you are intentionally testing a local stack:

```text
AGENTCLASH_API_URL=https://api.agentclash.dev
```

## PR Feedback

The workflow posts one sticky PR comment. It should include the verdict, score
deltas, run links, comparison links, replay links, artifact pointers, and
regression-tracking status so the PR author can inspect failures without guessing
which AgentClash page to open.

The GitHub check fails only when `agentclash ci run` returns a blocking gate
verdict. Non-blocking warnings still appear in the comment and uploaded artifacts.

## Regression Promotion

Use `regression_promotion: proposed` when CI should convert failed cases into
reviewable regression work. Use `auto_on_main` only when your default branch run
is trusted enough to promote failures automatically after merge. Use `disabled`
when CI should only report the gate result.

## Live E2E Runbook

1. In production, create or select a workspace with a runnable eval pack,
   runtime profile, agent build, baseline run, and GitHub App installation.
2. Save a CI profile from the CI setup page.
3. Open a setup PR into a CI test repository.
4. Verify the generated PR contains `.agentclash/ci.yaml` and
   `.github/workflows/agentclash.yml`.
5. Re-run setup against the same repo and confirm the UI reports file conflicts
   before creating a replacement PR.
6. Merge the setup PR.
7. Open a deliberate regression PR that changes a watched agent, prompt, model,
   or runtime file.
8. Confirm GitHub Actions runs AgentClash only for matched paths or labels.
9. Confirm the sticky PR comment links to the AgentClash run, comparison, and
   replay pages.
10. Confirm the GitHub check blocks the PR on a regression verdict and uploads
    result artifacts.
11. Confirm failed cases show up as proposed regression work, or auto-promote on
    the default branch when that profile option is selected.

## Local Dry Run

From a repository with a generated manifest:

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
export AGENTCLASH_TOKEN="..."
export AGENTCLASH_WORKSPACE="workspace-id"
npx agentclash ci run --config .agentclash/ci.yaml --dry-run
```

Then run without `--dry-run` on a CI branch after confirming the selected
workspace resources are correct.
