# AGENTS.md (web)

Guidance for AI agents editing the Next.js marketing site under `web/`.

## Typography

### Banned on marketing pages

Do **not** use `--font-display` (`Instrument Serif`) for headlines, hero copy, section titles, or CTAs on marketing pages (`web/src/app/**` routes that are public-facing: landing, `/enterprise`, `/pricing`, `/compare`, `/platform/*`, `/blog`, `/benchmarks`, etc.).

Instrument Serif reads thin and editorial at marketing sizes. It undermines enterprise conversion pages that need crisp, confident hierarchy.

**Use instead:**

- Headlines: `font-sans font-semibold tracking-tight` (Geist). Scale with `text-[clamp(...)]` or `text-3xl sm:text-5xl`.
- Body: default sans (`text-white/60`, `leading-7`).
- Labels / eyebrows: `font-mono text-[11px] uppercase tracking-[0.14em] text-white/40`.

`--font-display` remains allowed for the logo wordmark in `marketing-header.tsx` until a deliberate rebrand.

### Copy

- No em dashes (`—`) in marketing copy. Use periods, commas, colons, or parentheses.
- Headlines: outcome-first ("Ship agents with evidence…"), not category jargon ("Governed agent release gates").
- Keep paragraphs short (2–3 sentences). Enterprise buyers scan.

## Layout patterns that convert (B2B)

1. Hero: outcome headline + one-sentence proof + primary/secondary CTA + product visual (not abstract cards).
2. Trust bar: MIT, BYOK, open source, pilot terms near the fold.
3. Problem or buyer questions: numbered steps or scannable cards with clear hierarchy.
4. How it works: 3–4 steps max.
5. Offer block: one focal card for pilot/pricing, not a wall of equal-weight tiles.
6. FAQ + final CTA with repeated primary action.

Reference implementations: `web/src/app/platform/agent-evaluation/page.tsx`, revamped `web/src/app/enterprise/page.tsx`.

## Commands

```bash
cd web && pnpm dev
cd web && pnpm build
cd web && pnpm exec vitest run src/app/public-canonical-metadata.test.ts src/app/sitemap.test.ts
```
