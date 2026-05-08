# codex/seo-secondary-page-social-metadata - Test Contract

## Functional Behavior
- Public `/why` and `/team` pages keep their existing title, description, and canonical metadata.
- Both pages add explicit Open Graph metadata using page-specific title, description, URL, `website` type, site name, and default OG image with page-specific alt text.
- Both pages add explicit Twitter card metadata using page-specific title, description, and default Twitter image with page-specific alt text.
- The change must be metadata-only: no homepage UI changes, no shared footer changes, no authenticated/internal UI changes, no database changes, and no destructive behavior.

## Unit Tests
- `web/src/app/secondary-pages-metadata.test.ts` verifies `/why` and `/team` metadata exports include explicit Open Graph image and Twitter image metadata.

## Integration / Functional Tests
- `pnpm -C web test -- secondary-pages-metadata.test.ts`
- `pnpm -C web lint`
- `pnpm -C web build`

## Smoke Tests
- Build the web app and confirm `/why` and `/team` still prerender.
- Optionally inspect built HTML for `og:image`, `twitter:image`, and `summary_large_image`.

## E2E Tests
- N/A - metadata-only SEO change with unit and build coverage.

## Manual / cURL Tests
```bash
curl -s https://www.agentclash.dev/why | rg 'og:image|twitter:image|summary_large_image'
curl -s https://www.agentclash.dev/team | rg 'og:image|twitter:image|summary_large_image'
# Expected after deploy: both pages emit explicit OG and Twitter image metadata.
```
