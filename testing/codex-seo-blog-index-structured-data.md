# codex/seo-blog-index-structured-data - Test Contract

## Functional Behavior

- Add invisible JSON-LD structured data to the public `/blog` index.
- Structured data describes the AgentClash blog and lists current blog posts with absolute URLs, names, descriptions, and positions.
- The change must not alter visible homepage/landing UI, shared marketing footer, authenticated/internal UI, DB code, or destructive behavior.

## Unit Tests

- `pnpm -C web test -- json-ld.test.ts blog-index-schema.test.tsx`

## Integration / Functional Tests

- `pnpm -C web lint`
- `pnpm -C web build`

## Smoke Tests

- Rendered `/blog` output includes an `application/ld+json` script for the blog index schema.

## E2E Tests

- N/A - public metadata-only SEO change.

## Manual / cURL Tests

After deploy:

```bash
curl -s https://www.agentclash.dev/blog | rg 'agentclash-blog-index-schema|ItemList|Blog'
```

Expected: blog HTML includes JSON-LD for the blog index and post list.
