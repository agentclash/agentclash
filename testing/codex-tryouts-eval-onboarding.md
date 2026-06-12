# codex/tryouts-eval-onboarding — Test Contract

## Functional Behavior
- Public tryouts start with a short business-facing eval setup, not eval jargon.
- The setup asks a small number of plain-English questions about unacceptable mistakes, review owner, business priority, output style, and monthly volume.
- The setup derives rubric/validator intent and includes it in the tryout input sent to the backend.
- The agent prompt receives that eval intent so generated artifacts target the user's business criteria.
- The tryout UI explains the derived eval in simple language before launch and after results are available.
- Trace/event labels are more legible and less generic for non-engineering users.
- Public tryout cost caps are raised to support more realistic office tasks, including slide deck generation.
- Existing direct tryout URLs continue to load active/completed sessions.

## Unit Tests
- Backend tests cover eval context appearing in the public tryout task prompt.
- Backend tests cover the raised cost limits and quota defaults where relevant.
- Frontend/type checks cover the eval setup payload shape and tryout input construction.

## Integration / Functional Tests
- Creating a public tryout with eval setup fields persists the fields in `input_snapshot`.
- The public workflow prompt includes the eval setup JSON and runtime instructions.
- Active tryouts still expose artifacts while running.

## Smoke Tests
- `cd backend && go test ./internal/api ./internal/workflow`
- `cd web && npx tsc --noEmit`
- `cd web && npx eslint src/app/tryouts/tryouts-client.tsx src/lib/api/types.ts`
- `cd web && npm run build`

## E2E Tests
- Manual: open `/tryouts`, answer eval setup questions, choose a task/model, launch a slide deck tryout, verify the pre-run eval plan is visible and the resulting scorecard/trace language is understandable to a non-engineer.

## Manual / cURL Tests
- Inspect `/v1/agent-tryout-templates` and confirm higher `max_cost_usd` values are returned.
- POST `/v1/agent-tryouts` with `input.eval_setup` and confirm the created tryout returns the eval setup in `input_snapshot`.
