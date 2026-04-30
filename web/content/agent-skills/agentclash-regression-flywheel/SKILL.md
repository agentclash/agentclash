---
name: agentclash-regression-flywheel
description: Use when inspecting AgentClash run failures, promoting useful failures into regression suites, editing regression cases, and verifying suite-only reruns.
metadata:
  agentclash.role: regression
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Regression Flywheel

## Purpose
Turn useful run failures into durable regression coverage.

## Use When
- A user wants to inspect failures from a run.
- A failure should become a regression case.
- A suite-only run should verify that a fix still handles prior failures.

## Do Not Use When
- The run has no failure evidence yet.
- The user only needs a high-level scorecard summary.

## Inputs Needed
- Run ID.
- Failure ID or failure selection criteria.
- Target regression suite ID or permission to create one.
- Expected behavior for the promoted case.

## Environment
Use production by default; only override when another backend is explicit:

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
```

## Procedure
1. List failures and identify ones with clear reproduction value.
2. Inspect failure detail before promotion.
3. Create or select the regression suite.
4. Promote the failure with a clear expected behavior.
5. Review and edit the generated case if needed.
6. Run the suite-only scope and report whether the promoted case catches the issue.

## Commands
```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
agentclash run failures <run-id>
agentclash regression-suite list
agentclash regression-suite create --name "<suite-name>"
agentclash run promote-failure <run-id> <failure-id> --suite <suite-id>
agentclash regression-suite cases <suite-id>
agentclash regression-suite case update <case-id>
agentclash run create --scope suite_only --follow
```

## Expected Output
- Promoted cases retain enough input and evidence to reproduce the failure.
- Suite-only runs exercise the regression suite without unrelated pack scope.
- The report names the suite, case, and verification run.

## Failure Modes
- Failure lacks a reproducible input: do not promote until the missing context is captured.
- Duplicate promotion: use the existing case and update it instead.
- Suite-only run passes unexpectedly: confirm the expected behavior and selected deployment.

## Safety Notes
- Confirm before modifying shared production regression suites.
- Prefer descriptive case names that explain the behavior, not the incident.
- Do not include secrets or private artifact contents in regression cases.

## Report Back Format
```text
Failure: <failure-id>
Suite: <suite-id>
Case: <case-id>
Verification run: <run-id>
Result: <pass/fail>
Next action: <recommendation>
```

## Related Docs
- `/docs-md/concepts/replay-and-scorecards`
- `/docs-md/reference/cli`
