# feat/services-eval-program — Test Contract

## Functional Behavior

- `/services` is a public marketing page that productizes AgentClash adoption services (not generic consulting).
- Hero states: AgentClash is the platform; our team gets you to a first governed benchmark in 2 weeks.
- Four fixed offerings are listed with explicit duration, deliverable, and qualification criteria:
  1. **Eval Discovery** (1 week): audit agents, 5 failure modes, pack roadmap
  2. **Challenge Pack Build** (2–4 weeks): 3–10 custom packs from real workflows
  3. **Benchmark & Gate Setup** (2 weeks): baseline run, CI gate, exec scorecard template
  4. **Managed Eval Retainer** (monthly): release benchmarks + reliability report
- Guardrails section: every engagement produces customer-owned packs, baseline evidence, gate policy, or CI handoff in the customer workspace.
- Intake section lists what discovery captures: agent workflow, failure examples, compliance constraints, target release decision, current tooling, success criteria.
- CTAs: Book discovery call (Cal.com embed) and email `hello@agentclash.dev`.
- Cross-links to `/enterprise` (pilot) and `/pricing`.
- Typography follows `web/AGENTS.md`: sans headlines, no Instrument Serif, no em dashes in copy.

## Unit Tests

- `public-canonical-metadata.test.ts` — `/services` metadata has canonical `/services`.
- `sitemap.test.ts` — `/services` appears in sitemap with monthly changeFrequency and priority ≥ 0.75.

## Integration / Functional Tests

- Enterprise page links to `/services` from pilot/services context.
- Blog post CTA (`ResearchAudienceCTA`) includes a path to `/services` for eval workshop buyers.

## Smoke Tests

- `cd web && pnpm build` completes without errors.
- `cd web && pnpm exec vitest run src/app/public-canonical-metadata.test.ts src/app/sitemap.test.ts` passes.

## E2E Tests

N/A — static marketing page; manual browser check of `/services` layout and CTAs.

## Manual / cURL Tests

```bash
cd web && pnpm build
cd web && pnpm exec vitest run src/app/public-canonical-metadata.test.ts src/app/sitemap.test.ts
```

- Open `http://localhost:3000/services` and verify:
  - Four offering cards with duration + deliverable
  - Guardrails and intake sections visible
  - Primary CTA opens Cal.com; secondary mailto works
  - Links to `/enterprise` and `/pricing` resolve
