# codex/freemium-pricing — Test Contract

## Functional Behavior

- Public pricing and enterprise/service marketing copy describe AgentClash as freemium: users can run a meaningful number of evals first, see value, and then pay when they need more capacity, retention, collaboration, or governance.
- Pricing removes all seat-based language from public tiers and metadata. No public copy should say "seat", "seats", "per seat", "5-seat minimum", or equivalent.
- Public marketing copy removes the 45-day trial offer. No public copy should say "45-day", "45 day", "free trial", "trial", or "no credit card" as the primary paid-tier offer.
- Paid tiers should use workspace/month pricing and run quotas instead of seat/month pricing and per-seat quotas.
- Pro and Team CTAs should preserve plan intent through login using `returnTo` or another explicit mechanism so a visitor who clicks a paid-plan CTA is routed toward billing/trial/upgrade intent after authentication.
- Enterprise and services pages should no longer advertise a 45-day pilot/trial. They should emphasize a paid pilot, evaluation workshop, or fixed-scope service as appropriate.
- Machine-readable pricing JSON-LD must stay consistent with human-visible pricing data.

## Unit Tests

- `web/src/components/marketing/json-ld.test.tsx` pricing/schema tests should pass if they cover price specifications.
- Existing metadata and sitemap tests should continue to pass.
- Add or update tests if the pricing data structure changes in a way existing tests do not cover.

## Integration / Functional Tests

- `pnpm exec vitest run src/app/public-canonical-metadata.test.ts src/app/sitemap.test.ts src/app/seo-landing-pages.test.tsx src/components/marketing/json-ld.test.tsx` should pass from `web/`.
- Search the public marketing source for banned phrases and confirm remaining matches are unrelated product internals, historical changelog entries, or authenticated billing UI rather than current public pricing claims.

## Smoke Tests

- Build or targeted test run should confirm the Next metadata and sitemap routes still compile.
- Public `/pricing`, `/enterprise`, and homepage pricing block should render without TypeScript errors.

## E2E Tests

- N/A — this PR changes marketing/pricing copy and CTA routing only. A browser QA pass would be useful before release but is not required for the PR.

## Manual / cURL Tests

- Verify `web/src/lib/pricing-data.ts` contains no seat-based public tier language.
- Verify public marketing pages no longer mention the 45-day/free-trial offer.
- Verify Pro and Team CTA URLs include a return target that can preserve selected plan intent after login.
