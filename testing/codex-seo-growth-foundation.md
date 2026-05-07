# codex/seo-growth-foundation - Test Contract

## Functional Behavior

- Remove the hidden `/v2` marketing route tree so production has one canonical marketing surface.
- Update public SEO metadata to target the category terms AgentClash should rank for.
- Use one canonical host consistently across metadata, sitemap, robots, and JSON-LD.
- Keep agent-readable docs available while avoiding duplicate indexed docs content from `/docs-md`.
- Add initial indexable landing/content pages that can attract non-brand organic searches and route visitors toward trial, docs, GitHub, or demo actions.
- Keep manual/non-code tasks, such as Google Search Console verification, documented for follow-up rather than pretending code can perform them.

## Unit Tests

- N/A - this is a Next.js marketing/content change with metadata and static route behavior rather than isolated logic.

## Integration / Functional Tests

- `pnpm -C web lint` should pass.
- `pnpm -C web build` should pass if the local environment has all required production build variables.
- `rg '/v2' web/src web/content` should not find remaining website route links, aside from irrelevant dependency hashes or external/version strings.
- Generated sitemap should no longer include `/docs-md` mirrors after the duplicate-content strategy is implemented.

## Smoke Tests

- Public routes should compile:
  - `/`
  - `/blog`
  - `/blog/why-we-built-agentclash`
  - `/platform/agent-evaluation`
  - `/platform/agent-regression-testing`
  - `/platform/coding-agent-benchmarks`
  - `/compare/langsmith`
  - `/compare/promptfoo`
- `/v2` should no longer resolve to a real route in the app source.

## E2E Tests

- N/A - no authenticated product behavior changes are intended.

## Manual / cURL Tests

```bash
curl -I https://www.agentclash.dev/sitemap.xml
# Expected after deploy: 200 and sitemap entries use the canonical host.

curl -s https://www.agentclash.dev | grep -E "AI agent evaluation|canonical|SoftwareApplication"
# Expected after deploy: homepage metadata/category copy and structured data are present.
```

## Follow-Up Outside Code

- Verify Google Search Console ownership for `agentclash.dev`.
- Submit the corrected sitemap.
- Use URL Inspection for key landing pages.
- Track organic conversions from landing pages to signup, demo, GitHub, and docs.
