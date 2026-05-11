# codex/seo-blog-index-social-metadata - Test Contract

## Functional Behavior
- The public blog index keeps its existing title, description, canonical URL, and RSS autodiscovery metadata.
- The blog index adds explicit Open Graph metadata using the page title, page description, `/blog` URL, `website` type, site name, and default OG image with blog-specific alt text.
- The blog index adds explicit Twitter card metadata using the page title, page description, and default Twitter image with blog-specific alt text.
- The change must be metadata-only: no homepage UI changes, no shared footer changes, no authenticated/internal UI changes, no database changes, and no destructive behavior.

## Unit Tests
- `web/src/app/blog/blog-metadata.test.ts` verifies the blog index metadata includes explicit Open Graph image and Twitter image metadata.
- Existing RSS autodiscovery assertions continue to pass.

## Integration / Functional Tests
- `pnpm -C web test -- blog-metadata.test.ts`
- `pnpm -C web lint`
- `pnpm -C web build`

## Smoke Tests
- Build the web app and confirm the blog index still prerenders.
- Optionally inspect built blog HTML for `og:image`, `twitter:image`, and `summary_large_image`.

## E2E Tests
- N/A - metadata-only SEO change with unit and build coverage.

## Manual / cURL Tests
```bash
curl -s https://www.agentclash.dev/blog | rg 'og:image|twitter:image|summary_large_image'
# Expected after deploy: the blog index emits explicit OG and Twitter image metadata.
```
