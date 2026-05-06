# AgentClash Roadmap

Last updated: 2026-04-21

This file is the source of truth for what AgentClash is building, who it's for, and in what order. It exists to prevent scope drift — every feature, marketing surface, and doc should trace back to a specific tier below.

## Current positioning

**Git-bisect for agent behavior.** Race two agent configs on the same task, see exactly where they diverged — tool call, retrieval, latency, final answer — step by step. Built for engineers at AI startups shipping agents who hit the Promptfoo-gave-me-a-score-I-can't-debug wall.

Not "a governed decision artifact." Not "the benchmark control room." Not yet. Those come later if the engineer wedge earns the right.

## v1 — Engineer Dev Tool (next 12 weeks)

**Audience:** Engineers at AI startups shipping agents. Series A to Series B scale. Bottom-up, self-serve, OSS-first.

**Ship list:**

- [ ] `agentclash race --a v1.yaml --b v2.yaml --task my-task.yaml` CLI subcommand
- [ ] `agentclash login` CLI flow wiring into existing auth (producer is authenticated — every early user is an email address we can ask for a screen-share). Reuse existing auth, do not rewrite.
- [ ] `run_share_tokens` migration + anonymous-read handler for `/r/<id>` URLs. Default-public on race completion; `--private` CLI flag suppresses token mint. No redaction UI in v1.
- [ ] Next.js `/r/[id]` route: two-column synchronized-scroll replay, first-divergence highlighted (divergence = first step where tool-call name / normalized args / final-answer text differ)
- [ ] `task.yaml` schema: Promptfoo-compatible baseline + `agentclash:` extension fields (sandbox, tool policy, expected artifacts) — RFC as PR this week
- [ ] Landing page copy: lead with "git-bisect for agent behavior" (or equivalent), demote governance/enterprise framing
- [ ] 1 pre-baked public demo (e.g., Sonnet 4.7 vs Opus 4.7 on the knowledge-agent task) on the landing page
- [ ] Homebrew cask + curl installer verified end-to-end before launch

**Success criteria (all three, 12 weeks, across 3 launch waves):**

- 15+ engineers at 10+ different companies (not the founder) have run `agentclash race` on their own agent configs
- 5+ screen-share observation sessions with real users on real agents
- 3+ unsolicited public mentions (HN/Twitter/Discord) from people the founder doesn't know

**Failure signals (any = stop + re-diagnose):**

- Wave 1 usage drops to zero within 2 weeks with no inbound questions
- Founder cannot find 5 engineers willing to screen-share across all three waves
- Observation sessions reveal the diff replay doesn't actually explain real-codebase failures
- Acid test failure: the founder's own prod failure (the one that seeded this product) would not have been caught by Approach A alone → vNext Prod Trace Ingest becomes v1

## vNext — Workflow Depth (months 4–9, if v1 earns it)

Only unlocked by v1 success signals. Order within vNext is pulled by screen-share observation feedback, not pushed by founder intuition.

- OpenTelemetry GenAI trace ingest → diff prod run vs known-good offline run (the "origin story second half" — failures offline eval misses)
- CI/GitHub Actions integration: run your challenge pack on every PR, block merges on regression
- Team workspace concept: multiple engineers sharing task libraries + eval histories
- Judge library: common LLM-judge rubrics for correctness, faithfulness, tool-use quality
- Replay annotations + shareable post-mortems

## v3 — Enterprise / Governance (year 2+, only if pulled)

Vision doc: `docs/vision/enterprise-future.md` (the John Menon POV).

This tier exists as a direction, not a roadmap driver. It does not get implementation attention until there are ≥20 paying engineer-tier customers, ≥3 unsolicited inbound requests from enterprise buyers, and a clear ACV/CAC model that funds the multi-quarter buildout.

Features that belong here (and explicitly do NOT belong in v1/vNext):

- Evidence tier labeling (black_box / structured_trace / native_full_replay)
- Release gate policies (correctness regression thresholds, cost/latency budgets)
- Exportable evidence bundles for security/finance/procurement
- Vendor comparison governance (native + hosted agent side-by-side with evidence-quality scoring)
- SOC 2, private deployment, SSO, audit logs
- Challenge pack versioning + immutable environment snapshots for audit

If you find yourself tempted to build any of the above while in v1 or vNext: stop. Move the feature request to `docs/vision/enterprise-future.md`, close the tab, and ship the engineer thing.

## What NOT to ship — explicit do-not-build list

Things the founder instinct will want to build that are wrong for v1:

- Team workspaces, org membership, role-based access (v1 producer flow uses the existing single-user auth; consumer flow is anonymous via share token)
- Billing/paid tiers (no paid tier until ≥10 engineers are using it weekly and ask how to pay)
- Evidence tiering, governance, release gates (v3 — see vision doc)
- Multi-agent orchestration, agent frameworks, agent marketplaces (scope creep)
- Polished admin dashboards (everything lives on the public `/r/<id>` URL for v1)
- Cross-run aggregation UI beyond what already ships (pass@k UI is enough for v1)
- Writing more positioning docs. When tempted, write a competitor UX teardown or a `task.yaml` schema refinement PR instead.

## Design decision log

- **2026-04-21** — Moved `docs/product/enterprise-user-pov.md` to `docs/vision/enterprise-future.md`. Rationale: founder had written enterprise positioning without any enterprise conversations. v1 refocused on bottom-up engineer wedge. Source: `/office-hours` session design doc at `~/.gstack/projects/agentclash-agentclash/atharva-feat-323-regression-suites-frontend-design-20260421-140445.md`.
- **2026-04-21 (correction)** — Producer flow (CLI user running `agentclash race`) is authenticated via existing auth. Consumer flow (public `/r/<id>` URL) is anonymous via share token. Initial design proposed skipping auth entirely; founder pushed back — login is already built, and every early CLI user becomes an email address we can ask for a screen-share, which is more valuable than shaving 15 seconds off first-run friction. Precedent: `promptfoo share` works the same way (login to produce, public to view).
