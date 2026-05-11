# codex/seo-docs-social-image-alt - Test Contract

## Functional Behavior
- Public docs pages keep existing title, description, canonical, Open Graph, Twitter, and structured-data behavior.
- Docs page social image alt text becomes page-specific from the doc title and description.
- Twitter image metadata uses an object with `url` and `alt`, matching the blog and platform metadata shape.
- The change must be metadata-only: no homepage UI changes, no shared footer changes, no authenticated/internal UI changes, no database changes, and no destructive behavior.

## Unit Tests
- `web/src/app/docs/docs-metadata.test.ts` verifies docs Open Graph image alt and Twitter image alt match the docs page title/description.

## Integration / Functional Tests
- `pnpm -C web test -- docs-metadata.test.ts`
- `pnpm -C web lint`
- `pnpm -C web build`

## Smoke Tests
- Build the web app and confirm a docs page still prerenders.
- Optionally inspect built docs HTML for Twitter image metadata.

## E2E Tests
- N/A - metadata-only SEO change with unit and build coverage.

## Manual / cURL Tests
```bash
curl -s https://www.agentclash.dev/docs/getting-started/quickstart | rg 'twitter:image|twitter:image:alt|og:image:alt'
# Expected after deploy: docs social images include page-specific alt metadata.
```
