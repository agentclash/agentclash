# codex/vibe-eval-phase-0 — Test Contract

## Functional Behavior

- Add a durable Phase 0 Vibe Eval design artifact linked to GitHub issue #868 and parent #753.
- The artifact must inventory existing CLI/API capabilities that Vibe Eval can expose as typed semantic tools.
- The artifact must map the initial semantic tools from #753 to phase, risk tier, required role/action, confirmation policy, idempotency, redaction/audit, and budget policy.
- The artifact must explicitly call out exclusions/deferred actions, especially generic shell/API autonomy and admin-sensitive secret handling.
- The artifact must identify local org/workspace onboarding paths that later credit-wallet work needs to update.

## Unit Tests

N/A — this first slice is a documentation and decision-inventory artifact. No runtime behavior changes are expected.

## Integration / Functional Tests

- `rg "Vibe Eval Phase 0" docs testing` finds the checked-in Phase 0 artifact and this test contract.
- `rg "reserve_eval_credit|org_eval_credit|VibeEval" backend cli web` is expected to show no implementation introduced by this slice.

## Smoke Tests

- `git diff --check` passes.
- The new document renders as normal Markdown and contains no raw secrets or local credentials.

## E2E Tests

N/A — no user-facing workflow is implemented in this slice.

## Manual / cURL Tests

- Review `docs/vibe-eval-phase-0-inventory.md` against #868 acceptance criteria.
- Confirm the existing untracked `evals/` directory is not modified.
