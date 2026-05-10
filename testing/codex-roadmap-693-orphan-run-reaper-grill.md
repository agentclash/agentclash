# Grill Review: codex/roadmap-693-orphan-run-reaper

## Decisions tested

- Predicate scope — recommendation: clean only `queued`/`provisioning` rows with both Temporal IDs NULL and older than cutoff — trade-off accepted: some bad `running`/`scoring` rows remain for future/manual remediation, but this PR avoids killing real workflows.
- Status target — recommendation: transition to `failed`, not `cancelled` — trade-off accepted: these are infrastructure failures, not user-requested cancellations.
- Quota behavior — recommendation: release active concurrency by changing terminal status, but do not touch monthly race counters — trade-off accepted: billing/accounting semantics stay unchanged.
- Worker wiring — recommendation: periodic worker loop with interval-disable guard and conservative threshold — trade-off accepted: cleanup is eventually consistent, not immediate.

## Evidence checked

- Local `RunCreationManager.CreateRun` persists queued runs before calling `StartRunWorkflow`, so workflow-start failures can strand active rows.
- Local `CountActiveWorkspaceRuns` counts `queued`, `provisioning`, `running`, and `scoring`, so stranded rows consume concurrency.
- Local run status transitions allow `queued` and `provisioning` to transition to `failed`.
- Local `runs.sql` status update already sets `finished_at`/`failed_at` for failed terminal transitions.

## Blockers

- None.

## Major concerns

- **Severity: major** — If cleanup is implemented as a broad SQL update without checking both Temporal IDs and cutoff, it could fail legitimate work. Recommended fix: keep predicate exact and test skip cases.
- **Severity: major** — If cleanup updates status without history, operators cannot audit why runs failed. Recommended fix: write status history for every cleaned run.

## Questions for autonomous resolution

- None.
