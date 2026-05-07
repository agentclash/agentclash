# codex/homepage-category-h1 - Test Contract

## Functional Behavior

- Homepage hero H1 includes the exact high-intent category phrase `AI agent evaluation`.
- Hero support copy keeps the differentiated race/replay/CI-gate positioning.
- Homepage layout, auth behavior, pricing, and downstream sections remain unchanged.

## Unit Tests

- N/A - content-only marketing copy change.

## Integration / Functional Tests

- `pnpm -C web lint`
- `pnpm -C web build`

## Smoke Tests

- Built homepage route should include the updated H1 copy.
- Text should remain short enough for the existing hero container.

## E2E Tests

- N/A - no workflow or interaction changes.

## Manual / cURL Tests

After deploy:

```bash
curl -s https://www.agentclash.dev | rg "Open-source AI agent evaluation"
```

Expected: homepage HTML contains the updated category-forward hero copy.
