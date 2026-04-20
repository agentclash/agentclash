# Issue 149 Eval Session UI Contract

## Goal

Complete the missing user-facing product surface for repeated statistical evals from issue `#149` on top of the already-landed backend/API implementation.

## Functional Expectations

1. Workspace users can create an eval session from the web app without hand-writing JSON.
2. The create flow captures the repeated-eval concepts that are missing from the current UI:
   - participant selection by deployed agent
   - repetitions
   - aggregation method
   - variance reporting
   - confidence interval
   - optional reliability weight override
   - optional success threshold
   - optional routing/task snapshot fields needed by the backend contract
3. The runs area surfaces eval sessions as first-class objects instead of hiding them behind raw API calls.
4. Users can open an eval session detail screen and inspect:
   - eval session status and core configuration
   - child runs
   - run-count summary
   - evidence warnings
   - aggregate result when present
5. Aggregate result rendering covers the important repeated-eval semantics already produced by the backend:
   - overall and per-dimension aggregates
   - pass@k and pass^k
   - metric routing and composite agent score
   - participant aggregates for comparison sessions
   - repeated-session comparison outcome when present
6. The UI links cleanly back to underlying child runs so existing replay/scorecard flows remain reachable.
7. Existing single-run creation and run detail flows remain unchanged and working.

## Implementation Scope

1. Add frontend API types for eval-session create/list/detail responses.
2. Add a web create flow for eval sessions from the workspace runs area.
3. Add a list/read surface for eval sessions in the workspace UI.
4. Add an eval-session detail page and aggregate result presentation components.
5. Add focused frontend tests for the new create flow and detail rendering helpers/components.

## Tests To Add Or Run

1. Add or update frontend tests for eval-session creation request shaping and validation UX.
2. Add frontend tests for aggregate-result rendering logic or helper formatting.
3. Run targeted web tests covering:
   - `create-run-dialog.test.tsx` if touched
   - any new eval-session dialog/detail tests
   - any shared API type/client tests touched by the change
4. Run targeted backend eval-session tests only if an interface contract needs validation during integration.

## Manual Verification

1. Open the workspace runs page and verify both single-run and eval-session creation actions are visible.
2. Create a single-agent repeated eval session and confirm navigation to its detail page.
3. Create a comparison repeated eval session and confirm multiple participants are represented in the detail view.
4. Confirm detail view shows configuration, child runs, warnings, aggregate stats, pass metrics, and routing guidance when present.
5. Confirm child-run links open the existing run detail pages.
6. Confirm the existing single-run creation path still posts to `/v1/runs` and navigates to the run page.
