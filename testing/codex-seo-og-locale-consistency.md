# codex/seo-og-locale-consistency - Test Contract

## Functional Behavior
- Public page-level Open Graph metadata should preserve `locale: "en_US"` when it overrides root Open Graph metadata.
- Blog index, blog post, docs pages, and platform pages add explicit `openGraph.locale`.
- Existing title, description, canonical, RSS, social-image, and structured-data behavior must remain unchanged.
- The change must be metadata-only: no homepage UI changes, no shared footer changes, no authenticated/internal UI changes, no database changes, and no destructive behavior.

## Unit Tests
- `web/src/app/blog/blog-metadata.test.ts` verifies blog index and blog post Open Graph metadata include `locale: "en_US"`.
- `web/src/app/docs/docs-metadata.test.ts` verifies docs Open Graph metadata includes `locale: "en_US"`.
- `web/src/app/platform/platform-pages.test.tsx` verifies both platform page metadata exports include `locale: "en_US"`.

## Integration / Functional Tests
- `pnpm -C web test -- blog-metadata.test.ts docs-metadata.test.ts platform-pages.test.tsx`
- `pnpm -C web lint`
- `pnpm -C web build`

## Smoke Tests
- Build the web app and confirm representative blog/docs/platform pages still prerender.

## E2E Tests
- N/A - metadata-only SEO change with unit and build coverage.

## Manual / cURL Tests
```bash
curl -s https://www.agentclash.dev/blog | rg 'og:locale'
curl -s https://www.agentclash.dev/docs/getting-started/quickstart | rg 'og:locale'
curl -s https://www.agentclash.dev/platform/agent-evaluation | rg 'og:locale'
# Expected after deploy: all pages emit og:locale en_US.
```
