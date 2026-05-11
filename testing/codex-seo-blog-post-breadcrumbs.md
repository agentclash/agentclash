# codex/seo-blog-post-breadcrumbs - Test Contract

## Functional Behavior

- Add invisible breadcrumb JSON-LD to public blog post pages.
- Breadcrumbs include Home, Blog, and the current post with absolute URLs.
- The change must not alter visible homepage/landing UI, shared marketing footer, authenticated/internal UI, DB code, or destructive behavior.

## Unit Tests

- `pnpm -C web test -- blog-post-schema.test.tsx`

## Integration / Functional Tests

- `pnpm -C web lint`
- `pnpm -C web build`

## Smoke Tests

- Rendered blog post output includes breadcrumb JSON-LD alongside the existing BlogPosting schema.

## E2E Tests

- N/A - public metadata-only SEO change.

## Manual / cURL Tests

After deploy:

```bash
curl -s https://www.agentclash.dev/blog/ai-agent-evaluation-regression-testing | rg 'BreadcrumbList|BlogPosting'
```

Expected: blog post HTML includes both article and breadcrumb structured data.
