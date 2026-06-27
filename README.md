![AgentClash banner](docs/assets/agentclash-readme-banner.png)

# AgentClash

Open-source AI-agent evaluation for real tasks. AgentClash helps teams find where agents break, replay the evidence, score the outcome, and turn failures into regression gates before release.

[Website](https://www.agentclash.dev) | [Docs](https://www.agentclash.dev/docs) | [Quickstart](https://www.agentclash.dev/docs/getting-started/quickstart) | [Challenge Packs](https://www.agentclash.dev/docs/challenge-packs) | [CI Gates](https://www.agentclash.dev/docs/guides/ci-cd-agent-gates) | [Changelog](https://www.agentclash.dev/changelog)

[![npm version](https://img.shields.io/npm/v/agentclash?logo=npm&color=cb3837)](https://www.npmjs.com/package/agentclash)
[![npm downloads](https://img.shields.io/npm/dm/agentclash?logo=npm&color=cb3837)](https://www.npmjs.com/package/agentclash)
[![License: MIT](https://img.shields.io/github/license/agentclash/agentclash?color=blue)](LICENSE)
[![Backend CI](https://github.com/agentclash/agentclash/actions/workflows/backend.yml/badge.svg)](https://github.com/agentclash/agentclash/actions/workflows/backend.yml)
[![CLI CI](https://github.com/agentclash/agentclash/actions/workflows/cli.yml/badge.svg)](https://github.com/agentclash/agentclash/actions/workflows/cli.yml)
[![Frontend CI](https://github.com/agentclash/agentclash/actions/workflows/frontend.yml/badge.svg)](https://github.com/agentclash/agentclash/actions/workflows/frontend.yml)
[![GitHub stars](https://img.shields.io/github/stars/agentclash/agentclash?style=flat&logo=github)](https://github.com/agentclash/agentclash)
[![Open in GitHub Codespaces](https://github.com/codespaces/badge.svg)](https://codespaces.new/agentclash/agentclash)

**Community:** [Website](https://www.agentclash.dev) · [Discussions](https://github.com/agentclash/agentclash/discussions) · X: `TODO(handle)` · LinkedIn: `TODO(handle)` · Discord: `TODO(handle)`

AgentClash is built for teams shipping agents, not leaderboard demos. It runs agents against the same workload with the same tools and constraints, then preserves the transcript, artifacts, replay, scorecard, and failure taxonomy that explain why an agent passed or failed.

<img width="1774" height="887" alt="AgentClash scorecard with overall score, comparison ranking, dimensions, and validators" src="https://github.com/user-attachments/assets/a8578daa-6a1e-4268-b1c9-5fef542d8ad7" />

<!-- TODO(demo): record an asciinema cast of `agentclash eval start --follow` and embed it here (see docs/maintainers/growth-checklist.md). -->

## Try in 60 seconds

No clone, no local backend — run the CLI straight from npm against the hosted API:

```bash
npx agentclash@latest auth login --device
npx agentclash@latest doctor
```

Released binaries default to the hosted API (`https://api.agentclash.dev`). Once a
workspace has a challenge pack and a deployment, start a real eval:

```bash
npx agentclash@latest eval start --follow
```

Prefer the browser? Use the interactive terminal at
[agentclash.dev/try](https://www.agentclash.dev/try) — see [docs/deployment/try-cli.md](docs/deployment/try-cli.md).

## Why AgentClash

- **For teams shipping agents, not leaderboard demos.** Every candidate runs the
  same workload with the same tools and constraints.
- **Evidence, not just a number.** Each run preserves the transcript, model and
  tool calls, sandbox commands, artifacts, replay, scorecard, and failure taxonomy.
- **Regression gates, not one-off scores.** Promote escaped failures into permanent
  checks and fail CI when an agent regresses.
- **Real agents in real sandboxes.** Run Claude Code, Codex, and other harnesses as
  first-class candidates.
- **Open source and self-hostable.** MIT-licensed, and it runs locally with zero API keys.

## Start Here

| Goal | Best first step | Docs |
| --- | --- | --- |
| Run an eval | `agentclash eval start --follow` | [Quickstart](https://www.agentclash.dev/docs/getting-started/quickstart) |
| Author a workload | `agentclash challenge-pack init support-eval.yaml` | [Write a challenge pack](https://www.agentclash.dev/docs/guides/write-a-challenge-pack) |
| Gate CI | `agentclash ci init .agentclash/ci.yaml` | [CI/CD agent gates](https://www.agentclash.dev/docs/guides/ci-cd-agent-gates) |
| Use from an AI coding tool | `agentclash integration codex install` | [Use with AI tools](https://www.agentclash.dev/docs/guides/use-with-ai-tools) |
| Hack on the stack | `./scripts/dev/start-local-stack.sh` | [Self-host](https://www.agentclash.dev/docs/getting-started/self-host) |

## Quickstart

Install the CLI and connect a workspace:

```bash
npm i -g agentclash

export AGENTCLASH_API_URL="https://api.agentclash.dev"
agentclash auth login --device
agentclash link
agentclash doctor
```

Released npm binaries default to the hosted API. Keep the `AGENTCLASH_API_URL` export when you want to be explicit or switch between hosted and self-hosted environments.

If the workspace already has challenge packs and deployments, start an eval:

```bash
agentclash eval start --follow
agentclash eval scorecard
```

If the workspace is empty, scaffold and publish a pack first:

```bash
agentclash challenge-pack init support-eval.yaml
agentclash challenge-pack validate support-eval.yaml
agentclash challenge-pack publish support-eval.yaml
agentclash eval start --pack support-eval --follow
```

Or start from a ready-made pack in [`examples/challenge-packs/`](examples/challenge-packs/) (12 included):

```bash
agentclash challenge-pack validate examples/challenge-packs/fibonacci-e2e-showcase.yaml
```

For a specific completed run, use the run-first scorecard command:

```bash
agentclash eval scorecard <run-id> --agent <agent-label-or-run-agent-id>
```

`agentclash run scorecard` is lower-level and expects a run-agent ID. Use `agentclash run agents <run-id>` when you need that ID directly.

## What You Can Evaluate

- **Challenge packs** package prompts, tools, sandboxes, input sets, validators, judges, expected artifacts, and scoring rules. Start with the [challenge pack reference](https://www.agentclash.dev/docs/challenge-packs).
- **Replay and scorecards** preserve the full trajectory: model calls, tool calls, sandbox commands, artifacts, verdicts, latency, cost, and failure evidence. See [interpreting results](https://www.agentclash.dev/docs/guides/interpret-results).
- **Regression suites** promote escaped failures into permanent checks so the same mistake is tested before future releases.
- **Datasets** import or curate pinned examples, run real agent evals, record baselines, sync regression suites, and gate CI. See [datasets overview](https://www.agentclash.dev/docs/guides/datasets-overview).
- **Multi-turn packs** support scripted, LLM-driven, and human user simulators with takeover commands for operator input. See [multi-turn packs](https://www.agentclash.dev/docs/challenge-packs/multi-turn).
- **Security evals** test prompt injection, secret hygiene, and sandbox or vault boundaries without copying real secrets into docs. See [security evaluation](https://www.agentclash.dev/docs/guides/security-evaluation).
- **Agent harnesses** run external coding agents such as Claude Code, Codex, OpenClaw, and Hermes as first-class eval candidates in sandboxes.

## CI And Release Gates

AgentClash can compare a candidate run against a baseline and fail CI when the candidate regresses.

```bash
agentclash ci init .agentclash/ci.yaml
agentclash ci validate .agentclash/ci.yaml --remote
agentclash ci run \
  --manifest .agentclash/ci.yaml \
  --json \
  --artifact-dir agentclash-artifacts
```

Use the bundled GitHub Action when you want PR comments and uploaded artifacts:

```yaml
- id: agentclash
  uses: agentclash/agentclash/.github/actions/agentclash-ci@main
  with:
    manifest: .agentclash/ci.yaml
    token: ${{ secrets.AGENTCLASH_TOKEN }}
    workspace: ${{ secrets.AGENTCLASH_WORKSPACE }}
```

`AGENTCLASH_TOKEN` is the automation token used by CI. `AGENTCLASH_WORKSPACE` is the workspace ID that should own the run and artifacts. For local CLI sessions, `agentclash link` can save the workspace; CI should pass both values explicitly through repository or organization secrets.

API URL resolution order is:

```text
--api-url > AGENTCLASH_API_URL > saved user config > default
```

Manifest gates, dataset gates, and release-gate policies are covered in [CI/CD agent gates](https://www.agentclash.dev/docs/guides/ci-cd-agent-gates) and [dataset CI gates](https://www.agentclash.dev/docs/guides/dataset-ci-gates).

## Agent Skills

AgentClash ships Agent Skills that teach coding agents how to use the CLI, read scorecards, author packs, and gate releases.

Install first-class integration skills with the CLI:

```bash
agentclash integration claude install
agentclash integration codex install
agentclash integration cursor install
agentclash integration claude doctor
```

Supported CLI integration hosts are `claude`, `codex`, `cursor`, `openclaw`, `hermes`, and `opencode`. GitHub CLI skill bundles for additional hosts are documented in [Use with AI tools](https://www.agentclash.dev/docs/guides/use-with-ai-tools).

## Local Development

AgentClash is a monorepo:

- `backend/` - Go API server and Temporal worker.
- `cli/` - Go CLI module published through the `agentclash` npm package.
- `web/` - Next.js marketing, app, and docs site.

Run CLI checks from `cli/`:

```bash
cd cli
go build ./...
go vet ./...
go test -short -race -count=1 ./...
```

For the full stack, start with [self-host](https://www.agentclash.dev/docs/getting-started/self-host), [local API development](docs/api-server/local-development.md), and the repo-specific guidance in [AGENTS.md](AGENTS.md).

## Contributing

New contributors are welcome — and **most contributions don't need the backend**.
Docs and web work need only Node + pnpm; CLI work needs only Go against the hosted
API. See [CONTRIBUTING.md](CONTRIBUTING.md) for the tiered setup, or open the repo
in a ready-to-code environment:

[![Open in GitHub Codespaces](https://github.com/codespaces/badge.svg)](https://codespaces.new/agentclash/agentclash)

Looking for a first task? Check the
[`good first issue`](https://github.com/agentclash/agentclash/labels/good%20first%20issue)
label.

## Star AgentClash

If AgentClash is useful to you, please ⭐ the repo — it helps other teams find it.

[![Star History Chart](https://api.star-history.com/svg?repos=agentclash/agentclash&type=Date)](https://star-history.com/#agentclash/agentclash&Date)

## Contributors

Thanks to everyone who has contributed! ✨ ([emoji key](https://allcontributors.org/docs/en/emoji-key))

[![All Contributors](https://img.shields.io/github/all-contributors/agentclash/agentclash?color=ee8449&style=flat-square)](#contributors)

<!-- ALL-CONTRIBUTORS-LIST:START - Do not remove or modify this section -->
<!-- prettier-ignore-start -->
<!-- markdownlint-disable -->
<!-- markdownlint-restore -->
<!-- prettier-ignore-end -->
<!-- ALL-CONTRIBUTORS-LIST:END -->

This project follows the [all-contributors](https://allcontributors.org) spec —
comment `@all-contributors please add @user for code, doc` on any issue or PR to
recognize a contributor.

## Project

- [Contributing](CONTRIBUTING.md)
- [Code of Conduct](CODE_OF_CONDUCT.md)
- [Security policy](SECURITY.md)
- [CLI distribution](docs/cli-distribution.md)
- [License](LICENSE)

AgentClash is released under the [MIT License](LICENSE).
