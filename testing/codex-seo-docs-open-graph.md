# codex/seo-docs-open-graph - Test Contract

## Functional Behavior
- Public docs pages keep their existing title, description, and canonical metadata.
- Public docs pages add page-specific Open Graph metadata using the doc title, doc description, doc href, `website` type, and the existing default social image.
- Public docs pages add page-specific Twitter metadata using the doc title, doc description, `summary_large_image`, and the existing default Twitter image.
- The change must be metadata-only: no homepage UI changes, no shared footer changes, no authenticated/internal UI changes, no database changes, and no destructive behavior.

## Unit Tests
- `web/src/app/docs/docs-metadata.test.ts` verifies `generateMetadata` returns canonical, Open Graph, and Twitter metadata for a fixture docs page.
- The test verifies missing docs still return an empty metadata object.

## Integration / Functional Tests
- `pnpm -C web test -- docs-metadata.test.ts`
- `pnpm -C web lint`
- `pnpm -C web build`

## Smoke Tests
- Build the web app and confirm docs route generation still succeeds.
- Optionally inspect a built docs HTML page for page-specific OG title/description.

## E2E Tests
- N/A - metadata-only SEO change with unit and build coverage.

## Manual / cURL Tests
```bash
curl -s https://www.agentclash.dev/docs/getting-started/quickstart | rg 'og:title|twitter:title|Quickstart'
# Expected after deploy: metadata reflects the docs page title/description.
```
