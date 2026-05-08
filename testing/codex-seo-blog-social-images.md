# codex/seo-blog-social-images - Test Contract

## Functional Behavior
- Blog post metadata keeps existing title, description, canonical, RSS alternate, article type, published time, and author fields.
- Blog post metadata adds explicit `siteName` and default Open Graph image data so shared blog links have stable preview images.
- Blog post metadata adds explicit Twitter card metadata using the post title, post description, and default Twitter image.
- The change must be metadata-only: no homepage UI changes, no shared footer changes, no authenticated/internal UI changes, no database changes, and no destructive behavior.

## Unit Tests
- `web/src/app/blog/blog-metadata.test.ts` verifies blog post metadata includes the explicit Open Graph image and Twitter card fields.
- Existing RSS autodiscovery assertions continue to pass.

## Integration / Functional Tests
- `pnpm -C web test -- blog-metadata.test.ts`
- `pnpm -C web lint`
- `pnpm -C web build`

## Smoke Tests
- Build the web app and confirm blog routes still prerender.
- Optionally inspect a built blog HTML page for page-specific `og:image` and `twitter:image` tags.

## E2E Tests
- N/A - metadata-only SEO change with unit and build coverage.

## Manual / cURL Tests
```bash
curl -s https://www.agentclash.dev/blog/ai-agent-evaluation-regression-testing | rg 'og:image|twitter:image|summary_large_image'
# Expected after deploy: the blog post emits explicit OG and Twitter image metadata.
```
