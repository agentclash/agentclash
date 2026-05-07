# codex/homepage-seo-foundation - Test Contract

## Functional Behavior

- Update the homepage/root metadata so search engines see AgentClash as an AI agent evaluation platform, not only a brand name.
- Use `https://www.agentclash.dev` consistently for canonical public web signals touched in this PR.
- Add an explicit canonical URL for `/`.
- Add homepage `SoftwareApplication` and `Organization` JSON-LD so crawlers can identify the product and official profiles.
- Keep this PR narrow: no new landing pages, no blog posts, no docs-md duplicate-content changes.

## Unit Tests

- N/A - metadata/structured-data changes do not have isolated unit coverage.

## Integration / Functional Tests

- `pnpm -C web lint` should pass.
- `pnpm -C web build` should pass if the local environment has all required production build variables.
- `rg "https://agentclash.dev" web/src/app web/src/lib/docs.ts web/src/components/marketing/json-ld.tsx` should only report intentional documentation/examples, not canonical metadata constants.

## Smoke Tests

- `/` should still render the existing homepage for anonymous users.
- `/robots.txt` should point at the chosen canonical sitemap host after deploy.
- `/sitemap.xml` should emit URLs using the chosen canonical host after deploy.

## E2E Tests

- N/A - no authenticated product behavior changes are intended.

## Manual / cURL Tests

```bash
curl -s https://www.agentclash.dev | grep -E "AI agent evaluation|SoftwareApplication|canonical"
# Expected after deploy: category metadata, structured data, and canonical signal appear.
```

## Follow-Up Outside Code

- Submit the canonical sitemap in Google Search Console.
- Use URL Inspection for `/` after deploy.
- Monitor impressions for `AI agent evaluation platform`, `agent evaluation platform`, and `open source agent evals`.
