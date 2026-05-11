# codex/seo-docs-structured-data - Test Contract

## Functional Behavior

- Add invisible JSON-LD structured data to public docs pages.
- Structured data includes BreadcrumbList and TechArticle nodes with absolute URLs.
- The change must not alter visible homepage/landing UI, shared marketing footer, authenticated/internal UI, DB code, or destructive behavior.

## Unit Tests

- `pnpm -C web test -- json-ld.test.ts docs-page-schema.test.tsx`

## Integration / Functional Tests

- `pnpm -C web lint`
- `pnpm -C web build`

## Smoke Tests

- Rendered docs output includes BreadcrumbList and TechArticle JSON-LD.

## E2E Tests

- N/A - public metadata-only SEO change.

## Manual / cURL Tests

After deploy:

```bash
curl -s https://www.agentclash.dev/docs/getting-started/quickstart | rg 'BreadcrumbList|TechArticle'
```

Expected: docs HTML includes breadcrumb and TechArticle structured data.
