# codex/seo-robots-ai-crawler-policy - Test Contract

## Functional Behavior
- `robots.txt` continues to allow public crawlers to access the site.
- `robots.txt` continues to advertise the canonical production sitemap URL.
- The change must be test-only: no homepage UI changes, no shared footer changes, no authenticated/internal UI changes, no database changes, and no destructive behavior.

## Unit Tests
- `web/src/app/robots.test.ts` verifies the exported robots metadata allows `*` and points at `https://www.agentclash.dev/sitemap.xml`.

## Integration / Functional Tests
- `pnpm -C web test -- robots.test.ts`
- `pnpm -C web lint`
- `pnpm -C web build`

## Smoke Tests
- N/A - test-only guardrail change.

## E2E Tests
- N/A - test-only guardrail change.

## Manual / cURL Tests
```bash
curl -s https://www.agentclash.dev/robots.txt | rg 'Allow: /|Sitemap: https://www.agentclash.dev/sitemap.xml'
# Expected: public crawling remains allowed and the sitemap URL is present.
```
