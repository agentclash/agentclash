# AgentClash — GEO/AEO + Agent-Adoption Strategy & Handoff

> **What this is.** The founder asked two questions after reading up on *agentic commerce*
> (ACP/Stripe) and *GEO/AEO*: how do we get recommended by AI answer engines, and does
> "selling to AI" apply to us? This doc is the answer, plus a record of what shipped in the
> first execution round and the manual (founder-driven) work that remains.
>
> **Caveat applied throughout.** The precise multipliers floating around GEO blogs ("3.2x",
> "8.2x", "76.4%") trace to vendor marketing, not primary data. Treat them as *directional*.

---

## TL;DR verdict

1. **Agentic commerce (ACP / Stripe Agentic Commerce Suite / Shared Payment Tokens) does NOT
   apply.** It is consumer-retail checkout — agents buying from Etsy/Shopify/Coach inside
   ChatGPT. We have no storefront and sell no SKUs. The term reads as B2C. **Skip entirely.**
2. **The dev-tool analog ("agent-initiated adoption" — an agent discovering, installing, and
   running the tool friction-free) is already largely built:** npm + Homebrew + curl + CLI +
   Agent Skills + AGENTS.md + `integration install/doctor`. This box is checked.
3. **The real remaining lever is GEO/AEO: being the *recommended answer*** when a developer
   asks an AI "best agent-eval tool / how do I regression-test my agent / open-source LangSmith
   alternative." Most of the highest-value work here is **off-page GTM the founder must drive**,
   not code.

---

## What shipped this round (code)

All under `web/` (Next.js). No backend/CLI changes.

| Area | Change | Files |
| --- | --- | --- |
| **Pricing page** | New `/pricing` route — human page (reuses the landing `PricingBlock`) + self-host note + pricing FAQ. | `web/src/app/pricing/page.tsx` |
| **Machine-readable pricing** | Pricing extracted to a single source of truth and exposed as schema.org `Offer`s (`UnitPriceSpecification` per tier; Enterprise as a contact offer) so buyer-agents can evaluate before a sales touch. | `web/src/lib/pricing-data.ts` (new), `web/src/components/marketing/pricing-block.tsx` (refactor), `pricingSchema()` in `web/src/components/marketing/json-ld.tsx`; `/pricing` added to `web/src/app/sitemap.ts`; footer link in `compare-shell.tsx` |
| **Comparison expansion** | Decoupled per-competitor pages from the landing/hub capability matrix, then added 4 honest, researched competitor pages: **DeepEval, Galileo, Patronus AI, Ragas**. Each gets `/compare/agentclash-vs-*`, a hub card, an `ItemList`/FAQ JSON-LD entry, and a sitemap entry automatically. | `web/src/lib/comparison-data.ts`, `web/src/lib/comparison-data.test.ts` |
| **Blog** | 2 decision-stage posts (auto-flow into `/blog`, sitemap, and `BlogPosting` JSON-LD). | `web/content/blog/how-agentclash-scores-agent-trajectories.mdx`, `web/content/blog/why-agentclash-races-agents-head-to-head.mdx` |

**Design note on the compare refactor:** `Competitor` now carries its own `verdicts[]` (one per
capability row) instead of a `columnIndex` into the shared matrix. The landing page and the
`/compare` hub capability *table* still iterate `COMPARISON_COLUMNS`/`COMPARISON_ROWS` and stay
at the original six prompt-eval tools, so they are visually unchanged. New competitors are added
via `EXTENDED_COMPETITORS` and surface only as dedicated pages — they don't widen the matrix.
Their verdicts are honest and complimentary (sandboxed execution and head-to-head racing are
AgentClash-specific, so they read "no"; multi-turn/trajectory/cross-provider read "partial" per
each tool's real capabilities).

---

## What we already had (audit — do NOT redo)

- **Structured data** (`web/src/components/marketing/json-ld.tsx`): SoftwareApplication,
  Organization, WebSite, FAQPage, BreadcrumbList, BlogPosting, TechArticle, Blog, ItemList,
  Offer (price 0, open-source) — now joined by the pricing `Offer` catalog.
- **Comparison pages** (`web/src/app/compare/`, data in `web/src/lib/comparison-data.ts`):
  hub + per-competitor pages, capability matrix, honest "where competitor fits" notes, FAQs.
- **llms.txt / llms-full.txt** (`web/src/app/llms.txt/route.ts`, `llms-full.txt/route.ts`).
- **robots.txt** (`web/src/app/robots.ts`): 14 AI crawlers explicitly allowed + sitemap.
- **Sitemap** (`web/src/app/sitemap.ts`): blog/docs/compare/platform entries with OG images.
- **Agent Skills** (`web/content/agent-skills/`), **AGENTS.md**, README capability table.

---

## Manual next steps (founder-driven — cannot be done in-repo)

> Legend: **[GTM]** = off-page, founder-executed. **[TRAP]** = explicitly do NOT do.

### Tier 1 — highest leverage

1. **[GTM] Stand up a G2 profile and accumulate *real* reviews.** Third-party review sites are
   the single strongest AI-recommendation signal for B2B SaaS; reviews cascade G2 → Google →
   AI Overviews → Perplexity/ChatGPT. Also claim Capterra. *Ongoing; only real reviews.*
2. **[GTM] Get listed where devs and agents look.** PRs to relevant awesome-lists
   (awesome-LLMOps, awesome-ai-agents, agent-evaluation lists); a credible **Show HN** launch;
   nudge GitHub stars. Feeds both human discovery and training-data/RAG corpora.
3. **[GTM] Create a Wikidata entity** (instance-of: software; developer; source-code repo;
   license; npm id). Cheap; agents increasingly query Wikidata at runtime for grounded facts.
   *Wikipedia article = deferred (notability bar not met yet).*

### Tier 2 — moderate effort

4. **[GTM] Verify presence in the Vercel Skills (skills.sh) ecosystem.** We already author Agent
   Skills; confirm they're discoverable in the agent-skills registry/leaderboard. Validate it's
   live and worth it before investing further.
5. **[GTM] Keep `/compare` + `/platform/*` + blog on a visible refresh cadence.** Freshness is a
   real citation factor. (Adding more competitors is now a cheap in-repo change — see below.)

### Gated on real users

6. **Case studies + first-party "experience" content** — the moment 2–3 real users exist.
   First-hand experience content is a durable citation lever. **Do not fabricate.**

### Traps — explicitly do NOT do

- **[TRAP] Review/AggregateRating schema with no real reviews** — fabrication violates Google
  policy and gets spam-filtered. Revisit only after Tier-1 #1 produces genuine reviews.
- **[TRAP] MCP registries (mcp.so / Smithery / Glama)** — require an MCP *server*, which we
  deliberately chose **not** to build. Out of scope unless that decision is reversed.
- **[TRAP] Stripe ACP / Shared Payment Tokens / GPT Store / "agentic commerce" branding** —
  consumer-only; irrelevant.
- **Don't over-invest in llms.txt** — already shipped; consumer value is contested. Leave as-is.
- **Don't** chunk/fragment content for AI, keyword-stuff, or plant synthetic quotes.

---

## Cheap follow-on code (greenlight individually)

- **More competitor pages** are now low-cost: add an entry to `EXTENDED_COMPETITORS` in
  `web/src/lib/comparison-data.ts` with researched, honest verdicts. Page, hub card, JSON-LD,
  and sitemap wire up automatically. Candidates: Humanloop, Confident AI platform, Arize/others.
- **`/api/pricing` JSON endpoint** — not built; the JSON-LD `Offer`s on `/pricing` already make
  pricing machine-readable. Add a REST endpoint only if a concrete buyer-agent integration needs
  it.
- **More blog posts** — drop an `.mdx` into `web/content/blog/`; it auto-flows everywhere.

---

## Measurement (how we'll know it's working)

- **AI-referral traffic** in analytics: referrers `chatgpt.com`, `perplexity.ai`,
  `gemini.google.com`, `claude.ai`.
- **Periodic prompt audit**: manually ask "best AI agent evaluation tool / open-source LangSmith
  alternative" across ChatGPT, Claude, Perplexity, Gemini; record whether AgentClash appears and
  how it's described. (A paid tracker like Peec/Profound only if budget justifies.)
- **Leading indicators**: G2 review count, GitHub stars, awesome-list inclusions, Wikidata live.
