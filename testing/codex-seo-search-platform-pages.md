# codex/seo-search-platform-pages - Test Contract

## Functional Behavior

- Add public platform page entries to `/docs/search.json` data only.
- Search index includes `/platform/agent-evaluation` and `/platform/agent-regression-testing` with search text for AI agent evaluation, regression testing, CI gates, replay evidence, scorecards, and challenge packs.
- No homepage, main landing page UI, shared marketing footer, authenticated/internal UI, DB, or destructive changes.

## Unit Tests

- `pnpm -C web test -- docs.test.ts`

## Integration / Functional Tests

- `pnpm -C web lint`
- `pnpm -C web build`

## Smoke Tests

- Built `/docs/search.json` output includes both platform page URLs and target phrases.

## E2E Tests

- N/A - search JSON data only.

## Manual / cURL Tests

After deploy:

```bash
curl -s https://www.agentclash.dev/docs/search.json | rg "platform/agent-evaluation|platform/agent-regression-testing"
```

Expected: public search JSON includes both platform URLs.
