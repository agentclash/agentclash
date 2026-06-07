# Model Benchmark Workflow (Race Report Runbook)

This is the repeatable process for turning an AgentClash race into a published
benchmark report on the marketing site. Run it **every time a major model
launches** (≈ monthly) to ride the launch-week attention.

The published surface is `/benchmarks` (index) and `/benchmarks/<slug>` (report),
backed by MDX files in `web/content/benchmarks/`. There are no backend changes —
a report is a content file with a structured scoreboard in its frontmatter plus a
narrative body.

## Why this exists

AgentClash already produces the asset the AI ecosystem shares: head-to-head agent
races on real tasks with defensible scoring. A "We raced **\<new model\>** against
the field on 5 real agentic tasks — here's who won" post is high-signal, evergreen,
and SEO-friendly. The workflow below makes producing one fast and consistent.

## One-time setup

- A workspace on the hosted backend with a challenge pack of **real agentic tasks**
  (not trivia). The seed report uses five: an auth bug fix, a p99 latency hunt, a
  flaky-test triage, a safe schema migration, and a log-driven incident RCA.
- The `agentclash` CLI pointed at the hosted backend (see the repo `CLAUDE.md`,
  "CLI Against Hosted Backend"):

  ```bash
  export AGENTCLASH_API_URL="https://api.agentclash.dev"
  cd cli && go run . auth login --device
  go run . workspace use <workspace-id>
  ```

## Per-launch steps

### 1. Run the race

Create a run with the model lineup (the new model plus the current field) on the
challenge pack and follow it to completion:

```bash
cd cli
go run . run create --follow   # pick the pack + the models when prompted
```

Note the **run ID** when it finishes.

### 2. Export the ranking JSON

Fetch the run's ranking (the scoreboard the backend computes — see
`backend/internal/api/run_ranking.go`). Either save the CLI/API output to a file
or pipe it straight into the scaffold:

```bash
# via the API directly:
curl -s -H "Authorization: Bearer $AGENTCLASH_TOKEN" \
  "$AGENTCLASH_API_URL/workspaces/$AGENTCLASH_WORKSPACE/runs/<run-id>/ranking" \
  > ranking.json
```

The scaffold accepts either the full response (`{ state, ranking: {...} }`) or the
bare `ranking` payload.

### 3. Scaffold the report

```bash
node scripts/benchmarks/scaffold.mjs \
  --ranking ranking.json \
  --title "We raced <Model> against the field on 5 real agentic tasks" \
  --model "<Model>" \
  --slug <model>-vs-the-field \
  --share-url "https://www.agentclash.dev/share/<token>"   # optional
```

This writes `web/content/benchmarks/<slug>.mdx` with:

- `sample: false` (real data),
- the **scoreboard pre-filled** from the ranking (rank, winner, composite,
  correctness, reliability, latency, cost, $/correct), with a provider hint per
  model, and
- a prose skeleton (`How the race worked`, `The tasks`, `What we saw`, `Takeaway`).

It refuses to overwrite an existing file unless you pass `--force`.

### 4. Make a public share link (optional but recommended)

So readers can drill into the live race, create a public share of the run
scorecard and paste its URL into the report's `runShareUrl` frontmatter (the
scaffold's `--share-url` does this for you). The report page renders a
"View the live race scorecard" link to `/share/<token>`.

### 5. Edit the narrative

Fill in every `TODO` in the frontmatter (`verdict`, `challengePack`, each task's
`name`/`summary`) and the body. Keep the verdict to one line — it's the OG-card
subtitle and the index-card blurb. Confirm the provider on each scoreboard row.

### 6. Verify locally

```bash
cd web
npx tsc --noEmit
pnpm lint
pnpm test
pnpm build
pnpm dev    # open /benchmarks and /benchmarks/<slug>
```

Check: the scoreboard renders with the winner highlighted, the OG card
(`/og?title=...&kind=Benchmark`) looks right, and the sample-data banner is
**absent** (since `sample: false`).

### 7. Publish & syndicate

Merge to `main`. The report is automatically picked up by:

- `/benchmarks` index and `/benchmarks/<slug>` page,
- `sitemap.xml` (with a tailored OG image),
- `/benchmarks/feed.xml` (RSS),
- JSON-LD (`Article` + `Dataset`) for search/answer-engine discovery.

Then post the link to X and Hacker News on launch day. The verdict line is your
hook; the scoreboard is the proof.

## Authoring a report by hand

You don't need a real run to publish — set `sample: true` in the frontmatter and
the page shows a clear "Sample data" banner. This is how the seed report
(`claude-opus-4-8-vs-the-field.mdx`) works. Flip it to `false` once the numbers
are real.
