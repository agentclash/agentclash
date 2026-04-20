# codex/validate-autosave-main-fix — Test Contract

## Functional Behavior
- Guided build authoring on `main` should validate the current draft contents, not a stale saved copy.
- Clicking `Validate` should save the current build-version draft first, then call the validate endpoint.
- Clicking `Mark Ready` should also save the current build-version draft first, then call the ready endpoint.
- Existing save behavior, payload shapes, and advanced JSON editing must remain unchanged.

## Unit Tests
- `VersionEditor` should still cover guided hydration and guided/json synchronization.
- `VersionEditor` should verify that clicking `Validate` first PATCHes the current draft and only then POSTs to `/validate`.

## Integration / Functional Tests
- The editor should continue using the current `/v1/agent-build-versions/{id}`, `/validate`, and `/ready` endpoints with the same payload format.

## Smoke Tests
- `pnpm vitest run 'src/app/(workspace)/workspaces/[workspaceId]/builds/[buildId]/versions/[versionId]/version-editor.test.tsx'`
- `pnpm exec tsc --noEmit`
- `pnpm exec eslint` on the touched editor files

## E2E Tests
- N/A — not applicable for this change in this session.

## Manual / cURL Tests
- Manual UI flow:
  1. Open a draft build version.
  2. Edit `Mission / Instructions` without clicking `Save Draft`.
  3. Click `Validate`.
  4. Confirm validation succeeds against the edited text instead of failing with `policy_spec must contain an 'instructions' field`.
