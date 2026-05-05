# codex/ci-setup-ui — Test Contract

## Functional Behavior

- The workspace app exposes a `CI Setup` page from the sidebar.
- The page loads available workspace resources needed for AgentClash CI setup:
  agent builds, active deployments, challenge packs, input sets for the selected
  runnable version, active regression suites, recent completed runs, runtime
  profiles, provider accounts, model aliases, and connected GitHub repositories.
- A user can configure repository context, trigger paths and labels, candidate
  build/deployment settings, evaluation workload, baseline strategy, release
  gate, and regression promotion policy.
- The page generates `.agentclash/ci.yaml` that matches the existing CLI
  manifest schema in `cli/cmd/ci.go`.
- The page generates `.github/workflows/agentclash.yml` that uses
  `agentclash/agentclash/.github/actions/agentclash-ci@main`, grants
  `contents: read` and `pull-requests: write`, passes AgentClash token/workspace
  secrets, and uploads AgentClash result artifacts.
- The UI explains why CI will run through visible watched paths and force-run
  labels.
- The UI explains what a PR author gets when the gate fails: GitHub check,
  sticky AgentClash PR comment, linked runs/comparison/failures/replay, and
  regression tracking.
- Empty or partial workspaces show actionable missing-resource states instead
  of a dead form.
- Copy controls copy each generated file body to the clipboard and surface
  success/failure feedback.

## Unit Tests

- Generator tests cover a complete configuration and assert exact YAML content
  for `.agentclash/ci.yaml`.
- Generator tests cover GitHub workflow output and assert action ref,
  permissions, secrets, artifact upload, and manifest path wiring.
- Generator tests cover optional fields being omitted when unset.
- Generator tests cover quoted YAML scalars for special characters.
- Readiness tests cover missing resource selections and human-readable blockers.

## Integration / Functional Tests

- React tests render the CI setup page with mocked API data and verify the page
  shows generated manifest/workflow previews after resource selections.
- React tests verify missing-resource guidance appears when the workspace has no
  required resources.
- React tests verify copy buttons call `navigator.clipboard.writeText` with the
  generated YAML.

## Smoke Tests

- `cd web && npm test -- --runInBand` or targeted Vitest equivalent passes for
  new CI setup tests.
- `cd web && npm run lint` passes.
- `cd web && npm run build` passes or any environment blocker is documented.

## E2E Tests

N/A — no browser automation is required for this PR. The UI is covered by unit
and component tests, and manual verification is enough for the first generator
surface.

## Manual / cURL Tests

- Open `/workspaces/<workspace-id>/ci-setup`.
- Select a repository, build, runtime profile, challenge pack version,
  baseline run, and optional regression suite.
- Confirm the manifest preview includes the selected resource IDs.
- Confirm the workflow preview uses `.agentclash/ci.yaml`, posts PR comments,
  and uploads artifacts.
- Copy both files and paste them into a scratch repo; run
  `agentclash ci validate .agentclash/ci.yaml --remote` with the same workspace.
