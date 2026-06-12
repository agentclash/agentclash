# codex/tryouts-artifacts-model-selection — Test Contract

## Functional Behavior
- Public tryouts expose a concrete agent/model choice in the UI and API input.
- Supported public agent choices cover OpenAI/Codex, Anthropic/Claude, and OpenRouter/Gemini.
- If no choice is made, the tryout keeps the existing hosted default behavior.
- After any successful agent turn writes expected artifacts, the public tryout response includes the latest artifact previews even while the chat session remains open.
- A user can download or preview the latest artifacts after the opening turn, then send follow-up edit messages.
- After follow-up edit messages, downloadable artifacts update to the newest artifact snapshot.
- If the user sends several messages and then stops, artifacts remain downloadable while the session idles and later finalizes.
- Terminal completed summaries still include outputs and scorecards as before.
- The slide deck template no longer leaves users trapped behind a generic Thinking state after files are ready.

## Unit Tests
- Backend workflow tests cover successful opening turn with artifacts and verify outputs are persisted before terminal completion.
- Backend API tests cover selected public harness/model validation for OpenAI, Anthropic, and OpenRouter/Gemini choices.
- Frontend tests or focused type checks cover model option payload construction and artifact rendering from non-terminal summaries.

## Integration / Functional Tests
- Public create tryout accepts a selected model policy and selected harness kind.
- Public get tryout returns `summary.outputs` while status is still `running` when an artifact snapshot exists.
- Public events continue to show tool/model progress without leaking secrets.

## Smoke Tests
- `go test ./internal/api ./internal/workflow` from `backend/`.
- `pnpm exec vitest run` for the relevant tryouts or API tests from `web/`, if matching test harness exists.
- `pnpm exec tsc --noEmit` or the repo's equivalent web type check if no focused frontend test exists.

## E2E Tests
- Manual/live path: open `/tryouts`, choose slide deck, select an agent/model, submit a deck prompt, verify files become downloadable after the first successful turn without waiting for idle finalization, then send an edit and verify the session remains usable.

## Manual / cURL Tests
- `curl https://api.agentclash.dev/v1/agent-tryout-templates` should show public templates and model metadata once deployed.
- For local API, POST `/v1/agent-tryouts` with `template_slug=slide-deck`, `selected_harness_kind=codex_e2b`, and an OpenAI model policy should create a queued tryout.
- Repeat with `claude_e2b` and `openclaw_e2b`/OpenRouter Gemini policy validation.
