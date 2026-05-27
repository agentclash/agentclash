# codex/vibe-eval-phase-2-validate-publish — Test Contract

## Functional Behavior

- Users can validate a Vibe Eval `challenge_pack` draft against the existing AgentClash challenge-pack validator.
- Validation updates the draft with `valid` or `invalid` state and structured validation errors.
- Validation does not publish challenge packs, create runs, create provider secrets, or spend credit.
- Publishing requires an explicit confirmation token derived from a stable payload hash.
- Publishing is allowed only after the current draft validates successfully.
- Publishing promotes the validated draft through the existing challenge-pack publish path and links the resulting pack/version ids back to the draft.
- Publish decisions are auditable with action, requester, workspace, draft, payload hash, and result ids.
- Workspace viewers can read validation/publish state, but only workspace members/admins can validate or publish.

## Unit Tests

- API service tests cover:
  - validating a challenge-pack draft records valid state and clears errors.
  - validation errors are normalized into draft `validation_errors`.
  - publish without confirmation is rejected with a payload hash summary.
  - publish requires a valid draft and records published pack/version ids.
  - viewers cannot validate or publish.

## Integration / Functional Tests

- `cd backend && go test ./internal/api ./internal/repository`
- `cd backend && go test ./...` if time permits.

## Smoke Tests

- `git diff --check`
- `cd backend && go test ./internal/api`

## E2E Tests

N/A — this slice extends backend draft validation/publish surfaces only; no workbench UI is implemented.

## Manual / cURL Tests

- `POST /v1/workspaces/{workspaceID}/vibe-eval/drafts/{draftID}/validate` validates a draft-only challenge pack.
- `POST /v1/workspaces/{workspaceID}/vibe-eval/drafts/{draftID}/publish` without confirmation returns a required confirmation payload hash.
- `POST /v1/workspaces/{workspaceID}/vibe-eval/drafts/{draftID}/publish` with the matching confirmation token publishes and links the draft.
