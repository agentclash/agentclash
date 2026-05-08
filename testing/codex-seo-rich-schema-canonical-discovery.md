# codex/seo-rich-schema-canonical-discovery - Test Contract

## Functional Behavior
- Platform and homepage `SoftwareApplication` JSON-LD include richer feature/pricing fields without changing the landing page UI.
- Docs home emits FAQ structured data only for FAQ content that is visible on the docs home page.
- Public canonical metadata is guarded for homepage, blog, blog posts, docs, platform pages, why, team, and the new HTML sitemap page.
- A public HTML sitemap page improves crawler and human discovery for marketing pages, platform pages, docs, and blog posts.
- The XML sitemap includes the HTML sitemap route.
- The change must not modify the homepage/landing UI, shared marketing footer, authenticated/internal UI, database behavior, or destructive behavior.

## Unit Tests
- `web/src/components/marketing/json-ld.test.ts` covers richer `SoftwareApplication` fields and docs home FAQ schema.
- `web/src/app/platform/platform-pages.test.tsx` covers platform page `SoftwareApplication` feature lists.
- `web/src/app/docs/docs-page-schema.test.tsx` covers rendered docs home FAQ JSON-LD.
- `web/src/app/public-canonical-metadata.test.ts` covers canonical metadata across public pages.
- `web/src/app/sitemap-page.test.tsx` covers the HTML sitemap page route and links.
- `web/src/app/sitemap.test.ts` covers the XML sitemap entry for `/sitemap`.

## Integration / Functional Tests
- `pnpm -C web test -- json-ld.test.ts platform-pages.test.tsx docs-page-schema.test.tsx public-canonical-metadata.test.ts sitemap-page.test.tsx sitemap.test.ts`
- `pnpm -C web lint`
- `pnpm -C web build`

## Smoke Tests
- N/A - public metadata, structured data, and static discovery page only.

## E2E Tests
- N/A - no authenticated user journey changes.

## Manual / cURL Tests
```bash
curl -s https://www.agentclash.dev/sitemap | rg 'Agent evaluation|Docs|Blog'
curl -s https://www.agentclash.dev/sitemap.xml | rg 'https://www.agentclash.dev/sitemap'
# Expected: HTML sitemap exposes public discovery links, and XML sitemap advertises the HTML sitemap route.
```
