# codex/seo-blog-main-entity-schema - Test Contract

## Functional Behavior
- Blog post `BlogPosting` JSON-LD describes `mainEntityOfPage` as a `WebPage` node with the canonical article URL.
- Existing blog post structured data remains intact, including breadcrumb, image, dates, author, and publisher.
- The change must be crawler-only: no homepage UI changes, no shared footer changes, no authenticated/internal UI changes, no database changes, and no destructive behavior.

## Unit Tests
- `web/src/app/blog/blog-post-schema.test.tsx` verifies the rendered blog post JSON-LD includes a `WebPage` `mainEntityOfPage` object.

## Integration / Functional Tests
- `pnpm -C web test -- blog-post-schema.test.tsx`
- `pnpm -C web lint`
- `pnpm -C web build`

## Smoke Tests
- N/A - structured data only.

## E2E Tests
- N/A - structured data only.

## Manual / cURL Tests
```bash
curl -s https://www.agentclash.dev/blog/ai-agent-evaluation-regression-testing | rg 'mainEntityOfPage|WebPage'
# Expected: blog article structured data includes a WebPage mainEntityOfPage node.
```
