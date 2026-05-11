# codex/seo-sitemap-test-coverage - Test Contract

## Functional Behavior
- The public sitemap continues to include the homepage, blog index, platform pages, docs pages, blog posts, and machine-readable `llms` files.
- Sitemap entries preserve expected priorities for the highest-value SEO surfaces.
- The change must be test-only: no homepage UI changes, no shared footer changes, no authenticated/internal UI changes, no database changes, and no destructive behavior.

## Unit Tests
- `web/src/app/sitemap.test.ts` verifies representative sitemap entries for homepage, blog, platform pages, docs, blog posts, and `llms` files.
- The test mocks blog posts and docs paths to keep assertions deterministic.

## Integration / Functional Tests
- `pnpm -C web test -- sitemap.test.ts`
- `pnpm -C web lint`
- `pnpm -C web build`

## Smoke Tests
- N/A - test-only guardrail change.

## E2E Tests
- N/A - test-only guardrail change.

## Manual / cURL Tests
```bash
curl -s https://www.agentclash.dev/sitemap.xml | rg 'platform/agent-evaluation|docs|get-started|llms.txt|blog/'
# Expected: representative public discovery URLs remain present.
```
