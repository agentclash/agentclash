# feat-985-enterprise-page — Test Contract

Parent: #983 · Issue: #985

## Functional Behavior

Enterprise buyers get a dedicated `/enterprise` landing page (not docs/pricing alone) with pilot offer and eval architecture review CTAs.

- **Hero:** Governed agent release gates — prove which agent is safe to ship
- **Buyer questions:** trust, cost, failure evidence, compliance defense (from `docs/product/enterprise-user-pov.md`)
- **Product proof:** replay, scorecards, challenge packs, CI gates, Enterprise tier features (SSO, audit logs, SLA)
- **Offer:** 45-day Team pilot (no credit card); optional 2-week eval sprint intro via mailto until #986 `/services`
- **CTA:** Cal.com eval architecture review + `mailto:hello@agentclash.dev`
- **FAQ:** self-host vs hosted, BYOK, UAE/data residency note
- **Cross-links:** `/platform/agent-evaluation`, `/platform/agent-regression-testing`, `/compare`, `/pricing`

## Unit Tests

- `public-canonical-metadata.test.ts` includes `/enterprise` canonical
- Existing marketing tests continue to pass

## Integration / Functional Tests

- Sitemap includes `https://www.agentclash.dev/enterprise` with reasonable priority
- Page renders JSON-LD breadcrumb + FAQ schemas

## Smoke Tests

- `cd web && pnpm build`
- `cd web && pnpm exec vitest run src/app/public-canonical-metadata.test.ts src/app/sitemap.test.ts`

## E2E Tests

N/A — marketing page; manual browser verification.

## Manual / cURL Tests

1. Open `/enterprise` — hero, buyer questions, pilot offer, FAQ, cross-links visible
2. Demo button opens Cal.com embed
3. Email CTA opens mailto:hello@agentclash.dev
4. Footer and/or header link to `/enterprise`
5. Research CTAs on blog/benchmarks secondary link can point to `/enterprise`
