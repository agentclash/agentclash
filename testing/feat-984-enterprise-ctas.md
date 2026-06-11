# feat-984-enterprise-ctas — Test Contract

Parent: #983 · Issue: #984

## Functional Behavior

Research-stage visitors (blog, benchmarks, homepage) see demo-first CTAs aligned with enterprise eval intent—not CLI-only conversion.

- **Blog index (`/blog`)**: After intro copy, primary CTA is "Book eval workshop" (Cal.com via `DemoButton`); secondary links to `/platform/agent-evaluation`.
- **Blog post (`/blog/[slug]`)**: After article body, panel with demo CTA plus link to `/platform/agent-evaluation`.
- **Benchmarks index (`/benchmarks`)**: Demo CTA after intro (including coming-soon state).
- **Benchmark report (`/benchmarks/[slug]`)**: After scoreboard (and at page end), "Run this benchmark on your agents" panel with demo booking.
- **Homepage (`/`)**: Logged-out hero and closing sections use demo-first ordering; hero subcopy speaks to enterprise evaluators; auth/self-serve is secondary—not the white primary button.

Cal.com embed must initialize on pages using `DemoButton` outside the homepage client shell.

## Unit Tests

- `CTAStrip` passes custom `demoLabel` through to `DemoButton` — add test if component test file exists; otherwise covered by render smoke in blog index test.
- Existing blog index JSON-LD test still passes (`blog-index-schema.test.tsx`).
- Existing benchmarks metadata tests still pass (`benchmarks-metadata.test.ts`).

## Integration / Functional Tests

- Blog index renders `data-cal-link` on demo button and `/platform/agent-evaluation` secondary link.
- Benchmark report page renders benchmark-run CTA copy when report fixture is loaded.

## Smoke Tests

- `cd web && pnpm build` succeeds.
- `cd web && pnpm lint` passes on touched files.
- `cd web && npx tsc --noEmit` passes.

## E2E Tests

N/A — marketing CTA copy/layout only; manual browser verification sufficient.

## Manual / cURL Tests

1. Open `/blog` — confirm "Book eval workshop" is visible above post list; secondary links to enterprise evaluation page.
2. Open any `/blog/{slug}` — confirm CTA panel after article with demo + platform link.
3. Open `/benchmarks` — confirm demo CTA (coming-soon state OK).
4. Open `/benchmarks/{slug}` for a sample report — confirm scoreboard footer CTA.
5. Open `/` logged out — hero primary is demo booking; "Start first race" / auth is secondary styling.
