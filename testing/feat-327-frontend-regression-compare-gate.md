# feat/327-frontend-regression-compare-gate — Test Contract

Closes [#327](https://github.com/agentclash/agentclash/issues/327) — Regression
workflow subissue I: frontend compare/gate with regressions.

This contract is locked before implementation. It is the definition of "done"
for this PR.

## Scope

Frontend-only. Backend APIs (regression coverage on Run, regression gate rules
on release-gate policy, `regression_violations` on evaluation details) already
exist from subissues E/G/H (#323, #325, #326) and must not be modified. No
backend or OpenAPI changes.

## Functional Behavior

### Compare view (`compare/page.tsx`, `compare/compare-client.tsx`)

1. When both `baselineRun.regression_coverage` and
   `candidateRun.regression_coverage` are present on the two runs, render a
   new **"Regression Coverage"** section beneath the existing Key Deltas
   table and above Release Gates.
2. The section renders one row per candidate-side regression suite, showing:
   - Suite name
   - Case count
   - Candidate counters: `pass_count`, `fail_count`, derived
     `warn_count = case_count - pass_count - fail_count` (clamped at ≥ 0)
   - Delta vs baseline: `(candidate.fail_count - baseline.fail_count)` for the
     same suite id. Suites present only on one side are marked "new" or
     "baseline-only".
   - If `candidate.fail_count > baseline.fail_count`, the row is highlighted
     (red tint) and a "new failures" badge is shown. If equal or lower, no
     highlight.
3. A multi-select filter above the table lets the user restrict the table to
   one or more `suite_id`s. Empty selection = show all suites.
4. Each row has a caret/chevron that expands to show the suite's per-suite
   detail: for candidate unmatched cases on that suite, list case title,
   severity badge, and outcome. (If no per-case data is exposed by the
   existing API, the expand-row shows the candidate vs baseline counts side
   by side — "Candidate N pass / M fail" vs "Baseline N' pass / M' fail" —
   and a text link to the full run.)
5. The bottom of each row includes a small "Open suite" link routing to
   `/workspaces/{workspaceId}/regression-suites/{suite.id}`.

### "New blocking regression" alert banner

6. When any `ReleaseGate` fetched for the comparison has verdict `fail` and
   `evaluation_details.regression_violations` contains an entry whose `rule`
   is `no_blocking_regression_failure` or
   `no_new_blocking_failure_vs_baseline`, render a red alert banner at the
   very top of the compare body (above Breadcrumb-adjacent title), with:
   - Title: "New blocking regression"
   - Summary line from the gate
   - A link "View regression case" pointing to the first offending case
     detail page (using `suite_id` + `regression_case_id` to build
     `/workspaces/{workspaceId}/regression-suites/{suite_id}/cases/{regression_case_id}`)
7. The banner is suppressed when no release gates exist or when none have
   regression violations with those rules.

### Release-gate editor (`compare/evaluate-release-gate-dialog.tsx`)

8. Add a new **"Regression rules"** section above the JSON editor with the
   following controls backed by `regression_gate_rules` on the policy:
   - Toggle (checkbox): `no_blocking_regression_failure`
   - Toggle: `no_new_blocking_failure_vs_baseline`
   - Numeric input: `max_warning_regression_failures` (0 = disabled / null)
   - Multi-select: `suite_ids` (empty = all suites; options sourced from
     `GET /v1/workspaces/{workspaceId}/regression-suites`). Scope must be
     restricted to active suites.
9. Editing these controls writes the normalised rule object back into the
   JSON editor (keeping JSON as the source of truth). Editing the JSON
   manually keeps the inputs in sync on re-parse.
10. If the user disables all three rule toggles and leaves suite scope empty,
    `regression_gate_rules` is omitted from the submitted policy (to stay
    compatible with existing policy fingerprints).

### Release-gate evaluation result — violations list

11. When an evaluation returns `evaluation_details.regression_violations`
    (either in the dialog result or in an existing `GateCard`), render a
    **"Regression violations"** sub-panel listing each violation with:
    - Rule label (human-friendly: "Blocking failure", "New blocking vs
      baseline", "Warning threshold exceeded")
    - Severity badge
    - Link to the case detail page using `suite_id` + `regression_case_id`
    - Link to the scoring result deep link using
      `/workspaces/{workspaceId}/runs/{candidateRunId}/agents/{candidateRunAgentId}/scorecard#{scoring_result_id}`
      when `candidate_run_agent_id` is available on the comparison, else
      fall back to the case page only.

### Suite detail — Run History tab
(`regression-suites/[suiteId]/suite-detail-client.tsx`)

12. Replace the current "Run History coming soon" empty state with a table
    populated from `GET /v1/workspaces/{workspaceId}/runs?limit=50` filtered
    client-side to runs whose `regression_coverage.suites` contains the
    current `suiteId`.
13. For each matching run show: run name (link to run detail), status, the
    candidate-side pass/fail/warn counts for this suite, finished-at
    timestamp. Sort newest-first by `finished_at ?? created_at`.
14. Empty state: "No runs have executed this suite yet."

### Case detail — Recent Outcomes
(`regression-suites/[suiteId]/cases/[caseId]/case-detail-client.tsx`)

15. Replace the current empty state in the "Recent Outcomes" section with a
    table sourced the same way as §12: runs whose regression_coverage
    includes the case's `suite_id`.
16. Per row: run name + link, finished-at timestamp, and — best-effort — a
    badge indicating whether the case was among the run's `unmatched_cases`
    (showing `pending` / `pass` / `fail` when available, else "executed"
    with a link to the run). Current API does not expose per-case matched
    outcome, so the fallback wording is intentional; see §18.
17. Empty state: "This case has not executed in the last 50 runs."

### Non-goals / out-of-scope

18. Backend changes (e.g. adding a per-case run-history endpoint) are
    out-of-scope per the issue. The "Recent Outcomes" panel is
    deliberately best-effort for matched cases.
19. Auto-promotion suggestions, failure clustering (Phase 4).
20. Changes to run-creation flow (that is subissue G / issue 326's scope).

## Unit Tests

New or updated vitest unit tests under `web/src/lib/api/__tests__/`:

- `release-gates.test.ts::listReleaseGates` — GET request hits
  `/v1/release-gates` with the correct query params and Authorization header
  and returns the typed response.
- `release-gates.test.ts::evaluateReleaseGate` — POST request hits
  `/v1/release-gates/evaluate` with the given policy and returns the typed
  response, including `regression_violations` survival through the JSON
  round-trip.
- `release-gates.test.ts::normalizeRegressionGateRules` — local helper that
  accepts the structured form values and emits `undefined` when all three
  rule toggles are off and scope is empty; emits a canonical object
  otherwise. Mirrors backend `normalizeRegressionGateRules` (non-negative
  integer constraint) and trims empty suite ids.

## Integration / Functional Tests

Covered by the unit tests above plus the manual checks below. No new
integration tests required; the existing
`web/src/lib/api/__tests__/integration.test.ts` suite must still pass.

## Smoke Tests

- `cd web && pnpm lint` — clean.
- `cd web && npx tsc --noEmit` — clean.
- `cd web && pnpm vitest run src/lib/api` — all API unit tests pass.

## E2E Tests

N/A — no E2E harness exists for the compare view. Manual checks below.

## Manual / cURL Tests

Run `./scripts/dev/start-local-stack.sh` and open the web app.

1. Create two comparable runs that include a shared regression suite (see
   subissue G's seed). Navigate to
   `/workspaces/{ws}/compare?baseline={a}&candidate={b}`. Assert the
   "Regression Coverage" section renders with per-suite counts.
2. With the Evaluate Release Gate dialog, enable
   `no_blocking_regression_failure` via the new section, submit, and confirm:
   - The returned gate card shows a "Regression violations" list.
   - The compare view top-of-page shows the "New blocking regression"
     banner.
   - The "View regression case" link opens the correct case detail page.
3. In the "Regression rules" section, toggle all rules off and empty the
   suite scope, submit. Confirm the request body serialises without a
   `regression_gate_rules` field (check via the browser devtools Network
   tab payload).
4. On a suite detail page, open the "Run History" tab and confirm runs that
   included this suite appear with pass/fail counts; runs that did not are
   absent.
5. On a case detail page, scroll to "Recent Outcomes" and confirm runs that
   executed the parent suite appear; otherwise the empty state is shown.

## Validation checklist

- [ ] `cd web && pnpm lint`
- [ ] `cd web && npx tsc --noEmit`
- [ ] `cd web && pnpm vitest run src/lib/api`
- [ ] PR ≤ 1500 LOC (excluding `pnpm-lock.yaml`)
- [ ] No backend / OpenAPI changes
