# codex/seo-blog-article-schema-refinement - Test Contract

## Functional Behavior
- Blog post `BlogPosting` JSON-LD includes an article image using the existing AgentClash Open Graph image asset.
- Existing blog post structured data remains intact: breadcrumb, headline, URL, publication date, author, and publisher.
- The change must be crawler-only: no homepage UI changes, no shared footer changes, no authenticated/internal UI changes, no database changes, and no destructive behavior.

## Unit Tests
- `web/src/app/blog/blog-post-schema.test.tsx` verifies the rendered blog post JSON-LD includes the image object.

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
curl -s https://www.agentclash.dev/blog/ai-agent-evaluation-regression-testing | rg 'BlogPosting|og-image.png'
# Expected: blog article structured data includes the existing AgentClash article image.
```
