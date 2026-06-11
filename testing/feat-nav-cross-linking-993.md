# feat/nav-cross-linking-993 — Test Contract

## Functional Behavior

Improve discovery of high-intent marketing pages per #993:

1. **Marketing nav:** `/compare` appears in the default marketing header nav (footer already links to Compare tools).
2. **Blog related resources:** Blog post template renders a topic-selected "Related resources" block (2–3 links from the SEO registry) via a shared component and slug map. Links are not generic across every post.
3. **Changelog rollup:** A monthly blog post summarizes recent product updates and links prominently to `/changelog`.
4. **SEO cross-links:** Registry pages in `SEO_PAGE_REGISTRY` include hub links to peer SEO pages and `/compare` (excluding self-links). Canonical benchmark hub remains `/benchmarks`; `/ai-agent-benchmark` and `/agent-reliability-benchmark` are topic pages, not duplicate hubs.

## Unit Tests

- `marketing-header` or nav config test: default nav includes `/compare`.
- `blog-related-resources.test.ts`: slug map returns 2–3 links for mapped posts; unmapped posts return empty; no duplicate hrefs per slug.
- `seo-pages/registry` or dedicated test: S-tier pages include `/compare` in `relatedLinks`; pages do not link to themselves.
- `sitemap.test.ts`: every path from `getAllSeoPagePaths()` appears in sitemap output.

## Integration / Functional Tests

- Blog post page renders `BlogRelatedResources` when slug has mapped links.
- SEO landing pages render cross-link section including `/compare` where configured.

## Smoke Tests

```bash
cd web && pnpm build
cd web && pnpm exec vitest run src/lib/blog-related-resources.test.ts src/app/sitemap.test.ts
cd web && pnpm lint
```

## E2E Tests

N/A — marketing navigation and internal linking only.

## Manual / cURL Tests

- Open `/` and confirm header shows Compare linking to `/compare`.
- Open `/blog/pass-k-reliability-enterprise-teams` and confirm Related resources block with SEO links.
- Open `/blog/product-updates-june-2026` and confirm link to `/changelog`.
- Open `/agent-evals` and `/llm-agent-evaluation`; confirm related links include `/compare` and peer SEO pages.
- Fetch sitemap and spot-check SEO registry paths are present.
