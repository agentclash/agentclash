# codex/public-page-seo-metadata - Test Contract

## Functional Behavior

- Public indexable pages (`/why`, `/blog`, `/blog/[slug]`, `/team`, and `/docs/...`) expose route-scoped canonical metadata.
- Blog post pages include Article JSON-LD based on existing frontmatter.
- Existing routes, sitemap generation, and authenticated product pages remain unchanged.

## Unit Tests

- N/A - this is a metadata-only App Router change with no isolated unit-test surface.

## Integration / Functional Tests

- `pnpm -C web lint`
- `pnpm -C web build`

## Smoke Tests

- Next.js should prerender the existing blog post and docs pages without metadata errors.
- Generated blog post structured data should use the shared canonical host.

## E2E Tests

- N/A - no user workflow, auth state, or browser interaction changes.

## Manual / cURL Tests

After deploy, inspect:

```bash
curl -s https://www.agentclash.dev/blog/why-we-built-agentclash | rg 'application/ld\\+json|canonical'
curl -s https://www.agentclash.dev/docs/getting-started/quickstart | rg 'canonical'
```

Expected: blog post HTML includes Article JSON-LD and canonical tags point at `https://www.agentclash.dev/...`.
