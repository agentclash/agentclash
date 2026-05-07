# codex/seo-growth-foundation - Test Contract

## Functional Behavior

- Remove the hidden `/v2` marketing route tree so production has one canonical marketing surface.
- Remove or retarget shared marketing links that still point at `/v2` pages.
- Leave homepage SEO rewrites, canonical-host changes, docs-md indexing strategy, landing pages, and blog/report content for later small PRs.

## Unit Tests

- N/A - this is a Next.js marketing/content change with metadata and static route behavior rather than isolated logic.

## Integration / Functional Tests

- `pnpm -C web lint` should pass.
- `rg '/v2' web/src web/content` should not find remaining website route links, aside from irrelevant dependency hashes or external/version strings.

## Smoke Tests

- Public routes should compile:
  - `/`
  - `/blog`
  - `/blog/why-we-built-agentclash`
- `/v2` should no longer resolve to a real route in the app source.

## E2E Tests

- N/A - no authenticated product behavior changes are intended.

## Manual / cURL Tests

```bash
curl -I https://www.agentclash.dev/v2
# Expected after deploy: no longer a real 200 route.
```

## Follow-Up Outside Code

- Homepage SEO rewrites, canonical-host fixes, Google Search Console setup, new landing pages, and blog/report content will be handled in follow-up PRs.
