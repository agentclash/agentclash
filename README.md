![AgentClash banner](docs/assets/agentclash-readme-banner.png)

# AgentClash

Open-source AI agent evaluation for real tasks. AgentClash lets you run multiple agents against the same challenge, under the same constraints, then compare what happened with scorecards, transcripts, replays, and regression gates.

[agentclash.dev](https://www.agentclash.dev)

## Why Teams Use It

AgentClash is for teams building agents that need evidence, not vibes.

- Compare agents on real work: coding tasks, support workflows, tool-use tasks, recovery scenarios, and domain-specific evaluations.
- Replay every run: inspect tool calls, model turns, failures, costs, latency, and final outputs.
- Promote failures into regression cases: turn bad runs into repeatable tests.
- Gate releases in CI: compare a candidate agent against a baseline before shipping.
- Run from the CLI: create challenge packs, start evaluations, inspect runs, and publish results without leaving your terminal.

## Install

Install the CLI from npm:

```bash
npm i -g agentclash
agentclash --help
```

Or run it without installing:

```bash
npx agentclash --help
```

Direct release binaries and installer scripts are available in [GitHub Releases](https://github.com/agentclash/agentclash/releases). More distribution details are in [CLI Distribution](docs/cli-distribution.md).

## Connect To AgentClash

Use the hosted API unless you intentionally run your own backend:

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
agentclash auth login
agentclash link
```

`agentclash link` saves your default workspace. You can also use:

```bash
export AGENTCLASH_TOKEN="..."
export AGENTCLASH_WORKSPACE="workspace-id"
```

The API URL resolves in this order:

```text
--api-url > AGENTCLASH_API_URL > saved user config > default
```

## Start An Evaluation

If your workspace already has challenge packs and deployments:

```bash
agentclash eval start --follow
agentclash run list
agentclash eval scorecard
```

For lower-level control:

```bash
agentclash run create --follow
agentclash run transcript <run-id>
agentclash run scorecard <run-id>
```

## Create A Challenge Pack

Challenge packs describe the tasks agents compete on. Start with a scaffold, edit it for your workload, then publish it to your workspace:

```bash
agentclash challenge-pack init support-eval.yaml
agentclash challenge-pack validate support-eval.yaml
agentclash challenge-pack publish support-eval.yaml
```

Then run it:

```bash
agentclash eval start --pack support-eval --follow
```

Challenge packs can model multi-turn work, hidden grading criteria, scripted validators, tool access, input sets, and failure promotion. See [Challenge Pack v0](docs/evaluation/challenge-pack-v0.md) and the examples in [docs/challenge-packs](docs/challenge-packs).

## Compare Against A Baseline

Once you have a trusted run, save it as the workspace baseline:

```bash
agentclash baseline set <run-id>
agentclash eval scorecard <candidate-run-id>
```

AgentClash will show the candidate scorecard and the comparison against the bookmarked baseline.

## Use It In CI

AgentClash can gate an agent change before it ships:

```bash
agentclash ci init .agentclash/ci.yaml
agentclash ci validate .agentclash/ci.yaml --remote
agentclash ci run --manifest .agentclash/ci.yaml --json --artifact-dir agentclash-artifacts
```

In GitHub Actions, use the bundled action:

```yaml
- id: agentclash
  uses: agentclash/agentclash/.github/actions/agentclash-ci@main
  with:
    token: ${{ secrets.AGENTCLASH_TOKEN }}
    workspace: ${{ secrets.AGENTCLASH_WORKSPACE }}
```

See [CI/CD Agent Gates](web/content/docs/guides/ci-cd-agent-gates.mdx) and [AgentClash CI for GitHub](docs/agentclash-ci-github.md).

## Human Takeover

When a run pauses for operator input, inspect the turn and submit a response:

```bash
agentclash run turn status <run-agent-id> --run <run-id>
agentclash run turn submit <run-agent-id> --run <run-id> --message "Your message here"
```

## Useful Commands

```bash
agentclash quickstart
agentclash workspace list
agentclash workspace use <workspace-id>
agentclash challenge-pack list
agentclash deployment list
agentclash run list
agentclash run get <run-id>
agentclash run events <run-id>
agentclash run transcript <run-id>
agentclash eval scorecard <run-id>
agentclash baseline set <run-id>
```

Run `agentclash --help` or `agentclash <command> --help` for the full command reference.

## Local CLI Development

The CLI lives in `cli/`:

```bash
cd cli
go run . auth login --device
go run . workspace list
go run . workspace use <workspace-id>
go run . eval start --follow
```

Point a local CLI build at the hosted API:

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
```

Before shipping CLI changes:

```bash
cd cli
go build ./...
go vet ./...
go test -short -race -count=1 ./...
```

## Docs

- [CLI Distribution](docs/cli-distribution.md)
- [Challenge Pack v0](docs/evaluation/challenge-pack-v0.md)
- [Agent Harnesses on Codex + E2B](docs/agent-harnesses-codex-e2b.md)
- [CI/CD Agent Gates](web/content/docs/guides/ci-cd-agent-gates.mdx)
- [Local API Development](docs/api-server/local-development.md)

## License

AgentClash is released under [FSL-1.1-MIT](https://fsl.software), the Functional Source License with an MIT Future License clause. See [LICENSE](LICENSE) for the full text.
