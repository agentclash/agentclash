# codex/vibe-eval-phase-1-drafts — Test Contract

## Functional Behavior

- Users can create and list Vibe Eval conversations scoped to a workspace.
- Users can fetch a conversation they can access, including its active draft id.
- Users can create, list, fetch, and update Vibe Eval drafts scoped to a conversation.
- Draft kinds are restricted to `eval_plan`, `eval_pack`, `input_cases`, `scoring`, and `runtime`.
- Draft validation state is restricted to `unknown`, `valid`, and `invalid`; validation errors are stored as JSON arrays.
- Draft updates can change content and validation fields, but cannot publish packs, create runs, create provider secrets, or spend credit.
- Workspace viewers can read conversations/drafts, but only workspace members/admins can create or update them.

## Unit Tests

- API service tests cover:
  - member can create a conversation and draft.
  - viewer can list/read but cannot create/update draft state.
  - invalid draft kind and invalid validation state are rejected.
  - draft reads verify conversation/workspace ownership.

## Integration / Functional Tests

- `cd backend && go test ./internal/api ./internal/repository`
- `cd backend && go test ./...` if time permits.

## Smoke Tests

- `git diff --check`
- `cd backend && go test ./internal/api`

## E2E Tests

N/A — this slice exposes backend persistence surfaces only; no web workbench UI is implemented.

## Manual / cURL Tests

- `POST /v1/workspaces/{workspaceID}/vibe-eval/conversations` with a title creates a draft-only conversation.
- `GET /v1/workspaces/{workspaceID}/vibe-eval/conversations` lists durable conversations.
- `POST /v1/workspaces/{workspaceID}/vibe-eval/conversations/{conversationID}/drafts` creates a draft artifact.
- `PATCH /v1/workspaces/{workspaceID}/vibe-eval/drafts/{draftID}` updates content/validation state.
