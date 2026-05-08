# codex/seo-platform-social-metadata - Test Contract

## Functional Behavior
- Public platform pages keep their existing title, description, canonical, and Open Graph title/description/url/type metadata.
- Public platform pages add explicit Open Graph site name and default image data with page-specific alt text.
- Public platform pages add explicit Twitter card metadata using the page title, page description, and default Twitter image with page-specific alt text.
- The change must be metadata-only: no homepage UI changes, no shared footer changes, no authenticated/internal UI changes, no database changes, and no destructive behavior.

## Unit Tests
- `web/src/app/platform/platform-pages.test.tsx` verifies both platform page metadata exports include explicit Open Graph image and Twitter image metadata.
- Existing platform structured-data tests continue to pass.

## Integration / Functional Tests
- `pnpm -C web test -- platform-pages.test.tsx`
- `pnpm -C web lint`
- `pnpm -C web build`

## Smoke Tests
- Build the web app and confirm platform routes still prerender.
- Optionally inspect built platform HTML for `og:image`, `twitter:image`, and `summary_large_image`.

## E2E Tests
- N/A - metadata-only SEO change with unit and build coverage.

## Manual / cURL Tests
```bash
curl -s https://www.agentclash.dev/platform/agent-evaluation | rg 'og:image|twitter:image|summary_large_image'
# Expected after deploy: platform page emits explicit OG and Twitter image metadata.
```
