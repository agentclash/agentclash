![AgentClash banner](docs/assets/agentclash-readme-banner.png)

# AgentClash

Open-source AI agent evaluation for real tasks. AgentClash lets you race agents against the same workload, capture what they did, score the outcome, and turn failures into repeatable regression gates.

[Website](https://www.agentclash.dev) · [Docs](https://www.agentclash.dev/docs) · [Changelog](https://www.agentclash.dev/changelog) · [CLI Distribution](docs/cli-distribution.md) · [Challenge Packs](docs/evaluation/challenge-pack-v0.md) · [CI Gates](web/content/docs/guides/ci-cd-agent-gates.mdx)

## What AgentClash Does

AgentClash is built for teams shipping agents, not leaderboard demos. It evaluates the whole run: the final answer, the tool choices, the artifacts, the latency, the cost, and the evidence trail that explains why one agent passed while another failed.

- Race multiple agents on the same task with the same tools and constraints.
- Define repeatable workloads with challenge packs.
- Build versioned **eval datasets** — import from Braintrust, LangSmith, Phoenix, or production traces; run agent evals; gate CI on regressions.
- Run **multi-turn** workloads with scripted, LLM, or human user simulators.
- Evaluate **security** scenarios with dedicated packs, planted secrets, and vault-boundary harnesses.
- Execute external agents through **harness runners** (Claude Code, OpenClaw, Codex, Hermes) inside sandboxes.
- Watch runs live, then inspect transcripts, artifacts, replays, failures, and scorecards.
- Compare candidates against a saved baseline before release.
- Promote escaped failures into regression cases.
- Gate pull requests with the same evaluation workload you use during development.

## Product Surface

AgentClash gives you a workspace for the full evaluation loop:

| Area | What you use it for |
| --- | --- |
| Runs | Start and follow agent races across challenge packs and deployments. |
| Scorecards | Compare correctness, reliability, latency, cost, evidence, and pass/fail verdicts. |
| Replays | Review the step-by-step trajectory that produced the outcome. |
| Failures | Triage run failures, cluster by taxonomy, and promote important ones into regression coverage. |
| **Datasets** | Import, version, and eval example sets; ingest production traces; record baselines; fail CI on regressions. |
| Challenge packs | Package real tasks, inputs, validators, artifacts, and scoring rules — including multi-turn and security families. |
| Regression suites | Keep important failures covered across future model, prompt, and tool changes. |
| Compare and release gates | Decide whether a candidate is safe to ship against a baseline. |
| CI setup | Run AgentClash from GitHub Actions or another CI provider; PR comments link back to failure review. |
| **Try CLI** | [Interactive terminal demos](https://www.agentclash.dev/try) — README badges, disposable E2B sandboxes for CLI/TUI tools. |
| Agent harnesses | Run Claude Code, OpenClaw, Codex, or Hermes agents as first-class eval candidates in E2B sandboxes. |

<img width="1920" height="994" alt="AgentClash runs list showing completed benchmark runs" src="https://github.com/user-attachments/assets/b280c4be-3382-4151-af98-bb8000eba3c5" />


## Quickstart

Install the CLI:

```bash
npm i -g agentclash
agentclash --help
```

Or run it without installing:

```bash
npx agentclash --help
```

Log in and choose a workspace:

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
agentclash auth login
agentclash link
```

If your workspace already has challenge packs and deployments, start your first evaluation:

```bash
agentclash eval start --follow
agentclash eval scorecard
```

## Run Your First Agent Race

AgentClash can guide you through available packs and deployments:

```bash
agentclash eval start --follow
```

For lower-level control, create a run directly:

```bash
agentclash run create --follow
agentclash run list
agentclash run transcript <run-id>
agentclash run scorecard <run-id>
```

<img width="1774" height="887" alt="AgentClash run detail with agent lanes and ranking insights" src="https://github.com/user-attachments/assets/6f7f79e6-bea5-49e0-a651-3af5f474596f" />

For repeated evaluations, AgentClash summarizes child runs into session-level results:

<img width="1774" height="887" alt="AgentClash eval session aggregate result with pass@k metrics" src="https://github.com/user-attachments/assets/ad6e435a-b0fb-4684-b421-40cefec1667f" />

Multi-turn packs support human takeover mid-run — see [Multi-turn evaluation](#multi-turn-evaluation).

## Review Scorecards And Replays

Scorecards show whether each agent passed, how it ranked against peers, and which dimensions drove the verdict.

<img width="1774" height="887" alt="AgentClash scorecard with overall score, comparison ranking, dimensions, and validators" src="https://github.com/user-attachments/assets/a8578daa-6a1e-4268-b1c9-5fef542d8ad7" />

Replays keep the evidence behind the scorecard: model calls, tool calls, sandbox commands, scoring events, and final outputs.

<img width="1774" height="887" alt="AgentClash replay timeline showing run steps and JSON evidence" src="https://github.com/user-attachments/assets/a254d88a-e10f-4e9c-9c93-4169a06b35c4" />

## Define A Challenge Pack

Challenge packs are how AgentClash turns real work into repeatable evals. A pack can describe the task, inputs, tools, expected artifacts, hidden grading rules, validators, and regression cases.

Create and publish a pack:

```bash
agentclash challenge-pack init support-eval.yaml
agentclash challenge-pack validate support-eval.yaml
agentclash challenge-pack publish support-eval.yaml
```

Run it:

```bash
agentclash eval start --pack support-eval --follow
```

Learn more in [Challenge Pack v0](docs/evaluation/challenge-pack-v0.md) and the examples in [docs/challenge-packs](docs/challenge-packs).

## Eval Datasets

AgentClash datasets connect example sets to **real agent eval runs** — not just single-call prompt scoring. Import existing data, pin versions, run evals against challenge packs and deployments, record baselines, and fail CI when a candidate regresses.

**Import and export** — OpenAI, Braintrust, LangSmith, Phoenix, JSONL, and CSV:

```bash
agentclash dataset create --slug support-cases --name "Support cases"
agentclash dataset import <dataset-id> examples.jsonl --format braintrust
agentclash dataset export <dataset-id> --format langsmith --version <version-id>
agentclash dataset version create <dataset-id> --label "v1"
```

**Production trace ingest** — import OTEL, vendor exports, or AgentClash run replay as reviewable candidates before promotion:

```bash
agentclash dataset import-traces <dataset-id> traces.jsonl --source otel
agentclash dataset trace-candidates list <dataset-id>
agentclash dataset promote <dataset-id> <candidate-id> --expected '{"answer":"..."}'
```

**Synthetic generation** — Self-Instruct jobs expand seed examples into new cases:

```bash
agentclash dataset generate <dataset-id> \
  --strategy self-instruct \
  --count 50 \
  --seeds-tag seed \
  --create-version \
  --follow
```

**Dataset evals and CI gates** — run a pinned version against deployments, compare to a baseline, and emit JUnit for CI:

```bash
agentclash dataset eval <dataset-id> \
  --version <version-id> \
  --pack <pack-version-id> \
  --challenge support \
  --deployment <deployment-id> \
  --follow

agentclash dataset test <dataset-id> \
  --baseline <baseline-id> \
  --eval \
  --version <version-id> \
  --pack <pack-version-id> \
  --challenge support \
  --deployment <deployment-id> \
  --max-regressions 0 \
  --format junit
```

Sync a pinned dataset version into a linked regression suite:

```bash
agentclash dataset sync-regression-suite <dataset-id> \
  --version <version-id> \
  --pack <pack-version-id> \
  --challenge support
```

See [Dataset CI Gates](web/content/docs/guides/dataset-ci-gates.mdx).

## Multi-Turn Evaluation

Multi-turn challenge packs run conversations with scripted, LLM, or **human** user simulators. Operators can take over mid-run when a case needs a real person in the loop:

```bash
agentclash run turn status <run-agent-id> --run <run-id>
agentclash run turn submit <run-agent-id> --run <run-id> --message "Your message here"
```

Completed multi-turn runs expose conversation transcripts in the API and UI (replay rendering, scorecard view, PDF export). See [Multi-turn challenge packs](docs/challenge-packs/multi-turn.md) and the reference pack in [examples/challenge-packs/multi-turn-refund-recovery.yaml](examples/challenge-packs/multi-turn-refund-recovery.yaml).

## Security Evaluation

Security-family challenge packs score secret hygiene, prompt injection resistance, and vault-boundary behavior. AgentClash ships canonical packs, plants secrets into sandboxes at provisioning time, and supports stress-run tooling plus vault-framed harnesses (Infisical, HashiCorp Vault, agent-vault-stress).

```bash
agentclash security stress-run examples/challenge-packs/secret-hygiene-env.yaml --help
```

## Agent Harnesses

Run external coding agents as eval candidates without re-implementing their runtimes. Harness runners execute Claude Code, OpenClaw, Codex, and Hermes inside E2B templates and feed results back into the normal run/scorecard flow.

See [Agent Harnesses on Codex + E2B](docs/agent-harnesses-codex-e2b.md).

## Turn Failures Into Regression Tests

When an agent fails, AgentClash keeps the evidence around the failure: transcript, replay steps, artifacts, scorecard dimensions, and failure review metadata. Useful failures can become regression cases so the same mistake is tested again before the next release.

Typical workflow:

1. Run a pack against one or more candidate agents.
2. Inspect scorecards, replays, and failure details.
3. Promote important failures into a regression suite.
4. Re-run the suite whenever prompts, models, tools, or agent code changes.

> Screenshot placeholder: add a failure review or regression suite screenshot here.

## Gate Agent Changes In CI

AgentClash can compare a candidate run against a baseline and fail CI when the candidate regresses. Use manifest-based gates for challenge packs and regression suites, or [dataset CI gates](#eval-datasets) for pinned example sets.

Create and validate a CI manifest:

```bash
agentclash ci init .agentclash/ci.yaml
agentclash ci validate .agentclash/ci.yaml --remote
```

Run the gate and write artifacts:

```bash
agentclash ci run \
  --manifest .agentclash/ci.yaml \
  --json \
  --artifact-dir agentclash-artifacts
```

Use the bundled GitHub Action:

```yaml
- id: agentclash
  uses: agentclash/agentclash/.github/actions/agentclash-ci@main
  with:
    token: ${{ secrets.AGENTCLASH_TOKEN }}
    workspace: ${{ secrets.AGENTCLASH_WORKSPACE }}
```

See [CI/CD Agent Gates](web/content/docs/guides/ci-cd-agent-gates.mdx) and [AgentClash CI for GitHub](docs/agentclash-ci-github.md).

## Common Commands

```bash
agentclash quickstart
agentclash workspace list
agentclash workspace use <workspace-id>
agentclash challenge-pack list
agentclash deployment list
agentclash dataset list
agentclash eval start --follow
agentclash run list
agentclash run get <run-id>
agentclash run events <run-id>
agentclash run transcript <run-id>
agentclash run scorecard <run-id>
agentclash eval scorecard <run-id>
agentclash dataset eval <dataset-id> --version <version-id> --pack <pack-id> --challenge support --deployment <deployment-id>
agentclash dataset test <dataset-id> --baseline <baseline-id> --run <run-id>
agentclash baseline set <run-id>
```

Run `agentclash --help` or `agentclash <command> --help` for the full command reference.

## Configuration

Use the hosted API unless you intentionally run your own backend:

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
export AGENTCLASH_TOKEN="..."
export AGENTCLASH_WORKSPACE="workspace-id"
```

API URL resolution order:

```text
--api-url > AGENTCLASH_API_URL > saved user config > default
```

## Agent Skills

AgentClash ships [Agent Skills](https://agentskills.io) that teach coding agents
(Claude Code, Codex, Cursor, Gemini CLI, Copilot) how to drive the CLI — set up
auth, run evals, read scorecards, gate CI, and author challenge packs.

Install them straight into your agent with the CLI (idempotent; writes only
`SKILL.md` files, never your `CLAUDE.md`/`AGENTS.md`/`.mcp.json`):

```bash
agentclash integration claude install   # -> ~/.claude/skills/
agentclash integration codex install    # -> ~/.agents/skills/
agentclash integration claude doctor    # verify what's installed
```

Or, on [GitHub CLI](https://cli.github.com) 2.90+, install the published bundle
into any supported host (Claude Code, Copilot, Cursor, Codex, Gemini CLI,
Antigravity):

```bash
gh skill install agentclash/agentclash <skill>
```

See [Use with AI Tools](web/content/docs/guides/use-with-ai-tools.mdx) for usage
and [Publishing Agent Skills](docs/agent-skills-publishing.md) for maintainers.

## Local CLI Development

The CLI lives in `cli/` and can run against the hosted API:

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"

cd cli
go run . auth login --device
go run . workspace list
go run . workspace use <workspace-id>
go run . eval start --follow
```

Before shipping CLI changes:

```bash
cd cli
go build ./...
go vet ./...
go test -short -race -count=1 ./...
```

## Docs

- [Changelog](https://www.agentclash.dev/changelog)
- [CLI Distribution](docs/cli-distribution.md)
- [Challenge Pack v0](docs/evaluation/challenge-pack-v0.md)
- [Multi-turn challenge packs](docs/challenge-packs/multi-turn.md)
- [Dataset CI Gates](web/content/docs/guides/dataset-ci-gates.mdx)
- [Agent Harnesses on Codex + E2B](docs/agent-harnesses-codex-e2b.md)
- [CI/CD Agent Gates](web/content/docs/guides/ci-cd-agent-gates.mdx)
- [Local API Development](docs/api-server/local-development.md)

## License

AgentClash is released under the [MIT License](LICENSE).
