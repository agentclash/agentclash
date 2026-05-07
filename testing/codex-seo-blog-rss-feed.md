# codex/seo-blog-rss-feed - Test Contract

## Functional Behavior

- Add a public `/feed.xml` RSS feed for blog posts.
- Feed includes site title, description, canonical blog URL, and each blog post with title, link, guid, publication date, author, and description.
- Feed XML escapes text safely for titles/descriptions.
- No homepage, main landing page UI, shared marketing footer, authenticated/internal UI, DB, or destructive changes.

## Unit Tests

- `pnpm -C web test -- rss.test.ts`

## Integration / Functional Tests

- `pnpm -C web lint`
- `pnpm -C web build`

## Smoke Tests

- Built `/feed.xml` route returns RSS XML with the new blog post URL.

## E2E Tests

- N/A - static XML route only.

## Manual / cURL Tests

After deploy:

```bash
curl -s https://www.agentclash.dev/feed.xml | rg "<rss|/blog/ai-agent-evaluation-regression-testing"
```

Expected: RSS XML includes the blog feed channel and the new post URL.
