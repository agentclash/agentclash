# Codex Prompt Eval Failure Promotion - Test Contract

Issue: #594
Branch: `codex/prompt-eval-failure-promotion`

## Functional Behavior

- Add a design document for prompt-eval failure promotion into AgentClash regression assets.
- The design maps playground test cases, prompt-eval result rows, challenge-pack concepts, and regression cases.
- The design decides CLI vs UI ownership and explains the recommended rollout.
- The design covers ownership, data model changes, API shape, UI links, dedupe/idempotency, CI visibility, and how promoted failures show up in future CI.
- The design explicitly states that promotion is asynchronous/user-initiated and must not block the native prompt-eval run loop.
- The design defines the follow-up implementation surface for `prompt-eval promote-failures`.

## Unit Tests

N/A - design-only PR.

## Integration / Functional Tests

- Documentation review against the issue acceptance criteria.

## Smoke Tests

- Confirm the document is committed under `docs/`.

## E2E Tests

N/A - implementation is intentionally deferred.

## Manual / cURL Tests

```bash
sed -n '1,260p' docs/prompt-eval-failure-promotion.md
```
