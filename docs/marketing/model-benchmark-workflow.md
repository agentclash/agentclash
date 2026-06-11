# Model Benchmark Workflow (Monthly Public Report Runbook)

Repeatable process for turning an AgentClash race into a **monthly public benchmark**:
a measured report on `/benchmarks/<slug>`, a shareable blog summary, and a changelog
entry. Run it when major models ship and on a **monthly reliability cadence**.

Reference implementation: [Coding agent benchmark — June 2026](https://www.agentclash.dev/blog/coding-agent-benchmark-june-2026) and the [full scoreboard](https://www.agentclash.dev/benchmarks/gpt-generations-expression-evaluator).

## Roles and cadence

| Role | Responsibility |
| ---- | ---------------- |
| **Monthly owner** | Picks the pack, runs the race, exports ranking JSON, drafts MDX + blog |
| **Review approver** | Verifies numbers against replay, checks caveats and sample size honesty |
| **Distribution** | Posts share URL to X / HN / LinkedIn on publish day; pings benchmarks RSS |

**Target window:** publish by the **second week** of each month (or within 48h of a major model launch if that falls mid-month).

**Checklist (copy each cycle):**

- [ ] Pin challenge pack version **and** git commit SHA in the report appendix
- [ ] Race every candidate on the same pack (tools, sandbox, budgets)
- [ ] Export ranking JSON and attach replay / validator evidence per lane
- [ ] Set `sample: false` only when numbers are measured
- [ ] Write narrative + methodology appendix; add monthly blog summary
- [ ] Link from `/benchmarks` hub and add changelog entry for the period
- [ ] Promote meaningful failures into pack coverage or regression suites
- [ ] Run web verification (`tsc`, `lint`, `test`, spot-check OG cards)

## Public fixtures to use

Start from versioned packs in `examples/challenge-packs/`. The first published report uses:

| Pack | Path | Why |
| ---- | ---- | --- |
| Expression Evaluator Arena v1 | `examples/challenge-packs/expr-eval-arena.yaml` | Real coding task with hidden tiers; separates efficiency and contract adherence |

Other packs suitable for future months (rotate or expand field size):

- `multi-turn-refund-recovery.yaml` (multi-turn reliability)
- `incident-response-*.yaml` (ops / tool use)
- Security and eval-awareness packs as they stabilize

Record in every report:

- `pack.slug`, `evaluation_spec.name`, execution mode, tool policy, runtime limits
- Git commit SHA of the repo when the race ran

## One-time setup

- Workspace on the hosted backend with the chosen challenge pack imported
- CLI pointed at production (see repo `CLAUDE.md`, "CLI Against Hosted Backend"):

  ```bash
  export AGENTCLASH_API_URL="https://api.agentclash.dev"
  cd cli && go run . auth login --device
  go run . workspace use <workspace-id>
  ```

## Per-report steps

### 1. Run the race

```bash
cd cli
go run . run create --follow   # pick pack + model lineup
```

Note the **run ID** when it finishes. Every lane must use the same pack version and deployment policy.

### 2. Export the ranking JSON

The scoreboard comes from the run ranking API (`backend/internal/api/run_ranking.go`).

```bash
# CLI (preferred):
go run . run ranking <run-id> --json > ranking.json

# or HTTP:
curl -s -H "Authorization: Bearer $AGENTCLASH_TOKEN" \
  "$AGENTCLASH_API_URL/workspaces/$AGENTCLASH_WORKSPACE/runs/<run-id>/ranking" \
  > ranking.json
```

The scaffold accepts either the full response (`{ state, ranking: {...} }`) or the bare `ranking` payload.

**Required fields per report:** rank, composite, correctness, reliability, latency, cost, cost per correct (when available), winner flag, provider (confirm manually).

### 3. Scaffold the benchmark MDX

```bash
node scripts/benchmarks/scaffold.mjs \
  --ranking ranking.json \
  --title "We raced <Model> on <task headline>" \
  --model "<Model>" \
  --slug <kebab-slug> \
  --share-url "https://www.agentclash.dev/share/<token>"   # optional
```

Writes `web/content/benchmarks/<slug>.mdx` with `sample: false` and a prose skeleton. Refuses to overwrite unless `--force`.

### 4. Public share link (recommended)

Create a public share of the run scorecard and set `runShareUrl` in frontmatter. The report page renders "View the live race scorecard".

### 5. Edit narrative + methodology appendix

In the MDX body, include:

- How the race worked (lineup, constraints, n per model)
- Task description and why it was chosen
- What we saw (story behind the scoreboard)
- Caveats (sample size, provider scope, judge variance)
- **Methodology appendix:** frozen pack, models, scoring dimensions, reproduction commands

Keep `verdict` to one line (OG subtitle and hub blurb).

### 6. Publish the monthly blog summary

Add `web/content/blog/coding-agent-benchmark-<month>-<year>.mdx` (or launch-week variant) with:

- Shareable headline and scoreboard snapshot
- Link to `/benchmarks/<slug>`
- Methodology appendix (can mirror the report)
- Distribution-ready caveats

Update `BENCHMARKS_READING` / hub links in `web/src/lib/benchmarks-hub.ts`.

### 7. Verify locally

```bash
cd web
pnpm exec tsc --noEmit
pnpm lint
pnpm test
pnpm build
pnpm dev    # /benchmarks, /benchmarks/<slug>, blog post, OG cards
```

Confirm: scoreboard renders, sample banner absent when `sample: false`, hub shows latest report.

### 8. Changelog + syndicate

- Add an **Added** entry to the current period in `web/src/lib/changelog.ts` linking to the blog or `/benchmarks`.
- Merge to `main`. Surfaces update automatically: `/benchmarks`, sitemap, `/benchmarks/feed.xml`, JSON-LD.
- Post the **blog URL** (primary share link) to X, HN, LinkedIn. Use the verdict line as the hook; link the full scoreboard for proof.

## Promote failures into coverage

When a model fails or regresses on a dimension that should not recur:

1. Capture replay + validator evidence from the run
2. Add or tighten validators / cases in the pack (or promote to a regression suite)
3. Reference the promoted case in the next month's appendix

This keeps public benchmarks aligned with the same gates enterprise teams use in CI.

## Authoring without a live run

Set `sample: true` for illustrative scoreboards. The page shows a "Sample data" banner and is `noindex`. Flip to `sample: false` once numbers are measured.

## Scorecard export schema (reference)

Ranking items map to scoreboard rows:

| Ranking field | Report field |
| ------------- | ------------ |
| `label` | `model` |
| `rank` | `rank` |
| `run_agent_id === winner.run_agent_id` | `winner: true` |
| `composite_score` | `composite` |
| `correctness_score` | `correctness` |
| `reliability_score` | `reliability` |
| `latency_score` | `latency` |
| `cost_score` | `cost` |
| `cost_per_correct_usd` | `costPerCorrectUsd` |

Provider is not in the ranking document; confirm in frontmatter after scaffold guess.
