# codex/seo-rss-autodiscovery - Test Contract

## Functional Behavior

- Add RSS autodiscovery metadata for the public blog index and blog post pages.
- The alternate feed URL points to `/feed.xml` with `application/rss+xml`.
- The change must not alter visible homepage/landing UI, shared marketing footer, authenticated/internal UI, DB code, or destructive behavior.

## Unit Tests

- `pnpm -C web test -- blog-metadata.test.ts`

## Integration / Functional Tests

- `pnpm -C web lint`
- `pnpm -C web build`

## Smoke Tests

- Built blog pages include the RSS autodiscovery link in generated metadata/head output.

## E2E Tests

- N/A - metadata-only SEO discovery change.

## Manual / cURL Tests

After deploy:

```bash
curl -s https://www.agentclash.dev/blog | rg 'application/rss\\+xml|/feed.xml'
```

Expected: blog HTML includes an RSS alternate link for `/feed.xml`.
