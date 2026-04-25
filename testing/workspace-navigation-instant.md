# Workspace Navigation Instant — Test Contract

## Functional Behavior
- Workspace sidebar navigation should respond immediately by showing a loading shell instead of waiting on the next server-rendered page payload.
- Primary workspace list routes should render through thin shells and fetch their data client-side with stale-while-revalidate behavior.
- Shared workspace auth and membership checks must remain server-enforced.
- Shared workspace session and user fetches should be deduplicated through cached server helpers so settings and layout do not repeatedly call the same endpoints in a single request.
- Sidebar and workspace-switcher links should warm routes on hover/focus to improve perceived speed on common navigations.
- Representative create/update/delete flows on sidebar-backed pages should update their visible list state through SWR mutation rather than `router.refresh()`.
- Run and eval-session creation dialogs should warm their dependent SWR keys once when the dialog opens instead of repeatedly revalidating while form state changes.
- After a successful run or eval-session create, the first-page list cache should revalidate before redirect when possible so returning to the list shows fresh data.
- If post-create list revalidation fails, the user should still land on the newly created detail page and see explicit feedback that the background list refresh did not complete.
- Existing detail-page behavior, redirects, and permission gates must continue to work.

## Unit Tests
- `workspaceResourceKey` helpers return stable keys for list and paginated routes.
- Shared loading components render the expected skeleton landmarks for workspace list pages.
- Nested route loading shells render expected placeholders for heavy detail views.
- Run and eval-session creation dialogs trigger their warm-up invalidation once per open, not on later rerenders.
- Run and eval-session creation dialogs wait for list revalidation before redirecting, and surface a follow-up toast if revalidation fails after the create succeeds.

## Integration / Functional Tests
- Workspace shell loads under `AuthKitProvider` and SWR config without breaking existing route rendering.
- `runs`, `builds`, `deployments`, `challenge-packs`, `playgrounds`, `regression-suites`, `runtime-profiles`, `provider-accounts`, `model-aliases`, `tools`, `knowledge-sources`, `artifacts`, and `secrets` fetch via SWR and render their empty/table states correctly.
- Representative mutation flows invalidate the correct SWR keys and no longer call `router.refresh()` on those migrated pages.
- Run and eval-session creation keep their existing create payloads and redirects while making cache refresh ordering explicit.
- Workspace settings and workspace members still enforce admin access with cached session helpers.

## Smoke Tests
- `npm test -- --runInBand` or repo-equivalent Vitest run passes for the updated frontend tests.
- `npm run build` for `web/` completes successfully.
- Opening the app and clicking between primary sidebar destinations shows a loading shell immediately and then hydrated data.

## E2E Tests
- N/A — there is no dedicated browser E2E suite in this change.

## Manual / cURL Tests
- Manual: start the frontend, open a workspace, and click between `Runs`, `Builds`, `Deployments`, `Challenge Packs`, `Playgrounds`, and `Secrets`; confirm the page body swaps to a skeleton immediately instead of pausing on the previous page.
- Manual: hover or focus major sidebar links, then click them; confirm repeated visits feel warmer than the first navigation.
- Manual: create a build, deployment, challenge pack, run, eval session, regression suite, artifact upload, and secret; confirm each affected list updates without a full page refresh.
- Manual: open the run and eval-session dialogs, interact with multiple fields, and confirm dependent data is warmed once on open rather than re-fetching on every keystroke.
- Manual: create a run and eval session, then navigate back to the list; confirm the new item is present and any refresh failure shows a follow-up toast without blocking navigation.
- Manual: open workspace settings and members as an admin and non-admin user; confirm access rules are unchanged.
