# Agent Harness Evaluation SOTA Research

Date: 2026-05-06

## Executive Summary

AgentClash is already pointed at the right product category: long-running coding agents need a harness that can run inside a sandbox, touch a real repository, produce a diff or PR, and then be scored without forcing users to write challenge packs. The current implementation has useful foundations: Codex and Claude E2B runners, workspace secrets, GitHub App clone/PR plumbing, execution events, command validators, and a lightweight timeline UI.

The SOTA gap is that Agent Harnesses are still closer to "run a coding agent and maybe run commands" than to a full evaluation platform. Leading benchmarks and eval platforms converge on the same core ideas: versioned task contracts, reproducible environments, complete traces, deterministic validators, calibrated LLM/human judging, hidden checks, artifact capture, multi-run statistics, and rankings tied to cost/time/budget.

Important distinction: AgentClash's general scoring engine already has validators, scorecards, dimensions, runtime limits, pricing, and LLM judge declarations. The gap is that the Agent Harness execution workflow is not yet integrated with that richer scoring path. Today, harness evaluation decodes a local `evaluation_config`, supports command validators in the harness workflow, skips harness LLM judges, and emits a lightweight command-pass score.

The north star should be:

> A user chooses a repository or connected GitHub issue, describes a task, picks one or more coding harnesses, and AgentClash turns that into a reproducible experiment with live trace, PR artifact, validators, judges, scorecards, failure taxonomy, and comparable rankings.

This keeps Agent Harnesses separate from challenge packs while reusing AgentClash's existing scoring machinery.

## Current AgentClash Baseline

Verified against `origin/main` in the fresh worktree.

### What Exists

- Agent Harnesses are documented as workspace-scoped coding-agent task definitions, explicitly not challenge packs.
- API/workflow-supported harness kinds currently include `codex_e2b` and `claude_e2b`; the database check constraint already admits forward-looking `hermes_e2b` and `openclaw_e2b` values that are not yet runnable in the API/workflow path.
- Auth is `api_key_secret`; Codex receives `OPENAI_API_KEY` and `CODEX_API_KEY`, Claude receives `ANTHROPIC_API_KEY`.
- The worker provisions a sandbox, optionally clones a repository, runs the agent, captures git diff/changed files, and can create a draft GitHub PR for structured GitHub harnesses.
- Execution events are persisted for sandbox creation, clone/checkout, runner start/output/completion, git artifact capture, validators, scoring, and PR creation.
- Command validators run after the agent and can fail required executions.
- The UI lists harnesses, supports a chat-style run prompt, shows latest execution status/activity, and has an expandable execution timeline.

### Important Gaps In The Current Code

- Harness LLM judges are not wired. The workflow records `llm_judges.skipped` with the reason `agent harness LLM judge scoring is not wired yet`.
- Harness scoring is separate and shallow: `passed / (passed + failed)` for command validators, not the full scorecard/dimension model.
- Validator support is command-only in the harness workflow.
- No benchmark-suite abstraction exists for running a versioned set of harness tasks.
- No pass@k, multi-run variance, confidence interval, retry policy, or budget-aware ranking exists for harnesses.
- Hidden validators and contamination controls are not first-class for harnesses.
- Agent failure taxonomy is not first-class: environment failure, auth failure, no-op diff, bad PR, test failure, timeout, and judge failure are not scored separately.
- Trace UI exists, but it is still a compact event list rather than a debugger-grade replay with commands, stdout/stderr, decisions, diff, files, PR, validators, and scores as linked evidence.
- E2B template strategy is still a product risk for arbitrary customer stacks unless setup/bootstrap becomes a first-class harness stage.

## What "SOTA" Means Here

AgentClash sits across four partially overlapping markets. The SOTA target should be explicit:

| Quadrant | Examples | Should AgentClash Lead? | Why |
| --- | --- | --- | --- |
| Benchmark provider | SWE-bench, Terminal-Bench, GAIA, OSWorld, tau-bench | Selectively | AgentClash should support curated suites, but it does not need to own every public benchmark. |
| Evaluation platform | Inspect AI, Braintrust, LangSmith, Phoenix, Langfuse, Promptfoo | Yes for coding agents | AgentClash should be best at repo/PR-native agent evaluation, not generic prompt evals. |
| Race / arena | Chatbot Arena, Copilot Arena, Aider leaderboards | Yes | This is AgentClash's strongest differentiation: same task, same budget, multiple agents, visible comparison. |
| Agent product | Codex, Claude Code, Devin, Copilot, Jules, Cursor, Replit Agent | No | These are contestants and integrations, not the platform AgentClash should become. |

SOTA for Agent Harnesses therefore means: **benchmark-grade reproducibility, eval-platform-grade scoring, and arena-grade head-to-head comparison for long-running coding agents on real repositories.**

## Competitor And Benchmark Landscape

There is no permanently exhaustive "competitor" list because Agent Harnesses overlap benchmarks, eval platforms, sandbox infrastructure, and commercial coding agents, and this market changes monthly. This is the broad practical universe as of 2026-05-06.

### Coding-Agent Benchmarks

| Project | What It Evaluates | SOTA Lessons For AgentClash |
| --- | --- | --- |
| SWE-bench / SWE-bench Verified | Real GitHub issue resolution. Verified is a human-filtered 500-task subset. | Real repos and issue-to-patch tasks matter; human validation improves benchmark reliability; leaderboard comparability requires stable setup. |
| SWE-bench Multilingual / Multimodal / Live variants | Expanded software engineering tasks beyond Python/static sets. | Avoid one-language assumptions; freshness and modality matter for contamination resistance. |
| Multi-SWE-bench and related multilingual SWE-bench efforts | Multilingual repository-level issue resolution. | Customer repos are polyglot; Python-only benchmarks are not enough for buyer decisions. |
| SWE-rebench / SWE-bench Live | Fresh or continuously collected SWE tasks. | Contamination-resistant task release is now a first-order methodology requirement. |
| SWE-Lancer | Freelance software engineering tasks with economic value and end-to-end tests. | Buyers care about monetary value and real task economics, not only academic pass rates. |
| SWE-agent / mini-SWE-agent | Minimal agent loops over SWE-bench-style tasks. | Separate agent scaffold from benchmark task; publish config/version used for leaderboard comparability. |
| OpenHands evaluation harness | Runs agents through controlled runtimes and returns history, metrics, test results, and errors. | Preserve trajectories and metrics, not only final outputs. |
| Aider polyglot/benchmark work | Coding assistant comparisons across repositories and languages. | Language breadth and practical edit workflows matter for customer trust. |
| Terminal-Bench | Sandboxed terminal tasks with instructions, verifiers, and oracle solutions. | Every task needs an executable contract; terminal traces expose recovery behavior. |
| METR RE-Bench | Long-horizon research-engineering tasks with human baselines. | Hard tasks need budgets, human comparison, and time-to-completion metrics. |
| MLE-bench / PaperBench / related research-reproduction benchmarks | ML engineering and research-reproduction tasks. | Long-horizon work often needs hierarchical rubrics, objective scripts, and human baselines. |
| Commit-0 | From-scratch library implementation tasks. | Greenfield implementation is a different capability than bug fixing. |
| Security and cyber benchmarks | Security, exploit, patch, and CTF-style tasks. | Domain-specific harnesses need stricter sandbox, network, and safety controls. |
| LiveCodeBench, HumanEval, MBPP, EvalPlus | Code generation or function-level coding. | Useful component baselines, but not sufficient for long-running repo agents. |

Key sources:

- SWE-bench Verified describes the human-validated 500-instance subset and a leaderboard spanning simple loops through multi-rollout systems: https://www.swebench.com/verified.html
- SWE-rebench focuses on automated collection and decontaminated evaluation: https://openreview.net/forum?id=nMpJoVmRy1
- Multi-SWE-bench covers multilingual issue resolution beyond Python: https://github.com/multi-swe-bench/multi-swe-bench and https://arxiv.org/abs/2504.02605
- SWE-Lancer maps real freelance engineering tasks to monetary value: https://openai.com/index/swe-lancer/
- Terminal-Bench frames tasks as instruction + sandbox + verifier + oracle, with success checked by tests: https://terminalbench.lol/ and https://www.tbench.ai/
- OpenHands documents an evaluation harness shape that separates runtime, controller execution, history, metrics, and output: https://docs.openhands.dev/openhands/usage/developers/evaluation-harness
- OpenAI MLE-bench positions ML engineering as an agent benchmark with open-source benchmark code: https://openai.com/index/mle-bench/
- PaperBench uses hierarchical rubrics and LLM judges for research replication: https://openai.com/index/paperbench/
- Commit-0 covers from-scratch AI coding challenges: https://commit-0.github.io/

### Web, Desktop, And General Agent Benchmarks

| Project | What It Evaluates | SOTA Lessons For AgentClash |
| --- | --- | --- |
| WebArena | Autonomous web agents in realistic self-hosted web environments. | Reproducibility often requires hosted/local clones instead of live external websites. |
| WebArena-Infinity | Generated browser environments with verifiable tasks at scale. | AgentClash should eventually generate/derive private task suites, not only hand-author them. |
| VisualWebArena | Web tasks requiring visual grounding. | Some agent failures are observation/UI failures, not reasoning failures. |
| AssistantBench | Realistic, time-consuming web tasks across many live websites. | Long-horizon tasks need planning, memory, and realistic information-gathering checks. |
| BrowserGym / WorkArena | Web agents on ServiceNow-style knowledge work. | Enterprise workflows need stateful apps and task families. |
| OSWorld | Real computer tasks across OS/apps with execution-based evaluation scripts. | Initial state setup and execution-based checks are core to reproducible open-ended tasks. |
| GAIA | General assistant tasks requiring reasoning, tools, web, and multimodality. | Human-simple but agent-hard tasks are useful; hidden answers protect leaderboard credibility. |
| HCAST / METR autonomy work | Human-calibrated autonomy tasks and time-horizon measurement. | Measure how long an agent can work autonomously, not only whether it solved a short task. |
| TheAgentCompany / simulated company benchmarks | Consequential multi-step workplace tasks. | AgentClash can differentiate by being repo/PR-native, but should learn from simulated business state and multi-artifact grading. |

Key sources:

- WebArena-x lists WebArena, VisualWebArena, WebArena-Infinity, and TheAgentCompany as autonomous web-agent benchmark projects: https://webarena.dev/
- WebArena-Infinity focuses on generating browser environments with verifiable tasks: https://github.com/web-arena-x/webarena-infinity
- AssistantBench evaluates realistic time-consuming web tasks: https://assistantbench.github.io/
- OSWorld defines open-ended computer tasks with initial state setup and custom execution-based evaluation scripts: https://arxiv.org/abs/2404.07972
- GAIA defines real-world assistant tasks involving reasoning, multimodality, browsing, and tool use with hidden leaderboard answers: https://ai.meta.com/research/publications/gaia-a-benchmark-for-general-ai-assistants/
- HCAST introduces human-calibrated autonomy software tasks and time-horizon evaluation: https://metr.org/hcast.pdf

### Tool-Use And Simulated Workflow Benchmarks

| Project | What It Evaluates | SOTA Lessons For AgentClash |
| --- | --- | --- |
| tau-bench and successors | Tool-agent-user interaction in business domains with simulated users, policies, and APIs. | Multi-turn tasks need user simulation, policy adherence, pass@k, and error attribution. |
| AppWorld | Interactive coding agents over 9 simulated apps and 457 APIs with state/unit-test evaluation. | State-based tests can allow many valid solutions while detecting collateral damage. |
| BFCL | Function/tool calling accuracy, moving toward agentic tool-use evaluation. | Tool selection and argument correctness are measurable components of an agent harness. |
| ToolBench-style tool-use benchmarks | API/tool-use planning and execution. | Large tool surfaces require task-specific tool routing and failure analysis. |
| MCP-oriented tool-use benchmarks | Tool-use via MCP servers. | AgentClash should expect customers to evaluate MCP-enabled coding agents. |

Key sources:

- tau-bench's repository describes dynamic conversations between simulated users and tool-using agents, pass metrics, and LLM-assisted auto error identification: https://github.com/sierra-research/tau-bench
- AppWorld provides a high-fidelity app/API environment with robust state and execution-based unit tests, including collateral-damage checks: https://appworld.dev/
- BFCL V4 evaluates tool/function calling accuracy: https://gorilla.cs.berkeley.edu/leaderboard

### Head-To-Head And Arena Evaluation

| Project | What It Evaluates | SOTA Lessons For AgentClash |
| --- | --- | --- |
| Chatbot Arena / LM Arena | Blind pairwise human preference for chat models, moving from Elo to Bradley-Terry style modeling. | Arena ranking needs pairwise statistics, confidence intervals, model/version pinning, and bias controls. |
| Copilot Arena | Code completion/assistant suggestions evaluated in the wild through pairwise judgments. | Coding-agent preference data should be gathered from real developer workflows where possible. |
| Aider polyglot leaderboard | Practical code-editing benchmark across multiple languages. | AgentClash should expose language/task slices, not only one overall score. |
| WebDev Arena / web-app arenas | Pairwise preference over generated websites or app tasks. | For ambiguous coding tasks, human/LLM pairwise preference may complement validators. |
| AlpacaEval / MT-Bench style pairwise judging | Automated or human pairwise model comparison. | Pairwise judging is useful but needs position-bias and judge-bias mitigation. |

AgentClash's arena opportunity is stronger than generic eval platforms: run multiple harnesses against the same repo/task under identical budgets, then rank by final score, cost, time, PR quality, and pairwise preference.

Key sources:

- LMSYS documents the move from online Elo to Bradley-Terry modeling for arena rankings: https://www.lmsys.org/blog/2023-12-07-leaderboard/
- Chatbot Arena paper describes open human-preference evaluation: https://arxiv.org/abs/2403.04132
- Copilot Arena studies in-the-wild pairwise code assistant evaluation: https://arxiv.org/abs/2502.09328
- WebDev Arena applies arena scoring to web development tasks: https://webdev.lmarena.ai/leaderboard and https://arena.ai/blog/webdev-arena/
- Aider leaderboard docs: https://aider.chat/docs/leaderboards/

### Evaluation Frameworks And Platforms

| Platform | What It Does | SOTA Lessons For AgentClash |
| --- | --- | --- |
| OpenAI Evals | Framework and benchmark registry for evaluating LLMs and systems. | Custom/private evals matter; eval definitions must be portable and versioned. |
| EleutherAI lm-evaluation-harness | Widely used framework with many standard benchmarks and model backends. | Standard task adapters and reproducible CLI runs create ecosystem trust. |
| HELM | Holistic model evaluation across scenarios and metrics. | Multi-metric reporting beats one headline score. |
| Inspect AI | Extensible eval framework with tasks, solvers, scorers, logs, and sandboxing. | AgentClash should model tasks, solvers/runners, scorers, and logs as separate objects. |
| METR Vivaria | Eval infrastructure for agent elicitation research, now transitioning toward Inspect. | AgentClash should watch the task-runner-log architecture and the shift toward Inspect-compatible evals. |
| LangSmith | Traces, datasets, experiments, evaluators. | Production traces can become replayable eval datasets. |
| Braintrust | Datasets, scorers, experiments, immutable snapshots, CI/production monitoring. | Treat each harness run as an immutable experiment snapshot. |
| Langfuse | Traces plus typed scores from humans, LLM judges, programmatic checks, or feedback. | Scores should attach to traces/sessions/dataset runs with comments/reasons. |
| Phoenix / Arize | OpenTelemetry traces plus deterministic and LLM-as-judge evaluators. | Evaluator traces are themselves debuggable evidence. |
| Promptfoo | CLI-driven evals/red-teaming, matrix comparisons, assertions, CI. | Lightweight YAML/CLI adoption is a strength; harness suites should be runnable outside the UI too. |
| DeepEval | Pytest-style assertions, agent/tool-use metrics, synthetic datasets, tracing. | Developers understand evals when they look like tests. |
| W&B Weave, Comet Opik, Helicone, Humanloop, Patronus, Galileo, Giskard, TruLens, Ragas, Lakera | Managed eval/monitoring/governance layers. | Human review, trace analysis, governance, and safety monitoring matter once harnesses affect purchasing decisions. |

Key sources:

- OpenAI Evals is both a framework and open-source registry: https://github.com/openai/evals
- lm-evaluation-harness emphasizes many benchmarks, many model backends, YAML config, and reproducibility: https://github.com/EleutherAI/lm-evaluation-harness
- Braintrust describes data, task, scorers, immutable experiments, and continuous monitoring: https://www.braintrust.dev/docs/evaluate
- Langfuse models scores as a universal object attached to traces, observations, sessions, or dataset runs: https://langfuse.com/docs/evaluation/scores/overview
- Phoenix supports deterministic evaluators, LLM-as-judge evaluators, OpenTelemetry tracing, structured judge output, and evaluator traces: https://arize.com/docs/phoenix/evaluation/llm-evals
- Inspect AI frames evals around tasks, solvers, scorers, logs, and sandboxing: https://inspect.aisi.org.uk/
- Vivaria is METR's open-source tool for evaluations and agent elicitation research: https://github.com/METR/vivaria
- Promptfoo documents CLI evals, assertions, red-teaming, CI, caching, concurrency, and custom providers: https://www.promptfoo.dev/docs/intro/
- DeepEval provides Pytest-style LLM output assertions and agent/tool-use metrics: https://deepeval.com/docs/introduction

### Sandbox And Execution Infrastructure

| Platform | What It Provides | SOTA Lessons For AgentClash |
| --- | --- | --- |
| E2B | Secure cloud sandboxes for AI-generated code and agent sessions. | Good default for coding agents, but AgentClash must own task/eval semantics above it. |
| Modal Sandboxes | Secure containers for untrusted code, git checkout/test commands, arbitrary dependencies, setup scripts, and long timeouts. | Dynamic environments and setup scripts are essential for arbitrary tech stacks. |
| Daytona | Developer environments and sandboxes for AI agents. | Persistent/reproducible dev environments may be useful for longer enterprise tasks. |
| GitHub Actions / Codespaces | Existing customer CI/dev environments. | Harnesses should be able to reuse repo-native CI/setup rather than rebuild everything. |
| Docker/Nix/devcontainers | Portable environment specs. | Accepting existing repo config is better than requiring a template per customer. |

Key sources:

- E2B describes isolated VMs for AI-generated code, separate sandboxes per agent/user/session, and codegen eval use cases: https://e2b.dev/docs/quickstart/migrating-from-v0
- Modal Sandboxes support arbitrary code, git checkout/test commands, arbitrary dependencies, setup scripts, custom images, volumes, secrets, and longer timeouts: https://modal.com/docs/guide/sandboxes

### Commercial Coding Agents And Direct Product Competitors

These are not all "evaluation harness" competitors, but they define buyer expectations for what a coding agent run should show.

| Product | Relevant Behavior | Implication For AgentClash |
| --- | --- | --- |
| OpenAI Codex cloud | Cloud sandbox per task, connected repos, PR creation. | AgentClash must compare Codex against other harnesses, not merely host Codex. |
| Claude Code / Claude Code on web / GitHub Action | Remote repo tasks and PR creation through GitHub integrations. | Claude harnesses need account/API-key clarity and GitHub permission handling. |
| GitHub Copilot coding agent | Assignable work that creates PRs inside GitHub. | GitHub-native PR workflow is table stakes. |
| Google Jules | Async secure cloud coding agent connected to GitHub. | Setup reuse, test results, and GitHub loop are expected UX. |
| Devin, Cursor, Replit Agent, Windsurf, Sourcegraph Amp, Factory, Qodo, OpenHands | Autonomous or semi-autonomous coding workflows over repositories. | AgentClash should be the neutral arena: same task, same repo, same budget, same scoring, multiple harnesses. |

Key sources:

- OpenAI Codex can work in cloud sandboxes, connect GitHub, and create PRs: https://platform.openai.com/docs/codex and https://openai.com/codex
- GitHub Copilot coding agent can be asked to create pull requests: https://docs.github.com/en/copilot/how-tos/use-copilot-agents/coding-agent/create-a-pr
- Claude Code GitHub Actions can run Claude Code in GitHub workflows and request contents/issues/pull-request permissions: https://docs.claude.com/en/docs/claude-code/github-actions
- Jules connects to GitHub and runs as an experimental coding agent: https://jules.google/docs

## What SOTA Harness Evaluation Means

### 1. The Task Is A Contract

SOTA benchmarks do not store only a prompt. They store an initial state, allowed tools, setup steps, hidden/public checks, expected artifacts, budgets, and metadata. For coding agents, the task contract should include:

- repository and base ref
- task prompt or source issue
- environment setup
- allowed network/secrets/tool policy
- expected changed artifacts
- validators and hidden validators
- judge rubrics
- time/token/tool budgets
- scoring dimensions
- contamination/publicity flags

### 1.5 The Arena Is A Contract Too

For head-to-head AgentClash runs, fairness invariants must be explicit:

- same base repository/ref
- same setup and hidden validators
- same wall-clock/token/tool budgets, unless ranking includes budget-normalized scoring
- same access to secrets and network
- same task prompt and public context
- isolated sandboxes unless a shared-environment race is deliberately selected
- blinded pairwise judging when humans or LLMs compare outputs
- model/harness version pinning

### 2. The Environment Is Reproducible

The benchmark must say what environment was used and how it was built. For customer repos, AgentClash should avoid requiring a custom E2B template per stack by supporting a layered strategy:

- default base template
- repo-native setup detection (`package.json`, `go.mod`, `Cargo.toml`, `pyproject.toml`, `devcontainer.json`, `flake.nix`, CI workflows)
- explicit setup commands
- optional custom template
- captured tool versions and setup logs

Environment failures should be scored separately from agent failures.

### 3. The Trace Is Evidence

A final message is not enough. The trace should preserve:

- model/harness/runtime versions
- each command/tool call
- stdout/stderr and exit codes
- structured agent events and decisions where available
- changed files and full diff
- generated artifacts
- PR branch/URL/commits
- validator logs
- judge prompts/results/reasons
- cost, tokens, duration, retries

This is why observability platforms attach scores to traces and why OpenHands-style harnesses return history and metrics.

### 4. Scoring Combines Deterministic Checks And Judging

The default hierarchy should be:

1. Required deterministic gates: setup, build, tests, validators, no forbidden files, no secret leaks.
2. Objective metrics: changed files, latency, cost, command count, token count, retries.
3. Domain-specific checks: hidden tests, snapshot diffs, API/database state, artifact existence.
4. LLM/human judges: maintainability, task completeness, autonomy, patch quality, risk.

LLM judges should never be the only signal for correctness on coding tasks unless the task has no executable oracle. Judge outputs need structured schemas, rationale, model/version, and calibration samples.

### 5. Rankings Need Statistics

One run is a demo. An evaluation needs:

- success@1 and pass@k
- pass^k / repeated-trial degradation where tasks are conversational or stateful
- repeated trials under the same budget
- confidence intervals
- paired bootstrap or Bradley-Terry-style ranking for head-to-head races
- failed-run taxonomy
- cost/time-normalized rankings
- immutable experiment snapshots
- comparisons by harness, model, prompt, template, repo, and task type

tau-bench's pass^k style and eval platforms' experiment snapshots are good patterns.

### 6. Methodology Threat Model

SOTA harnesses are attacked by subtle evaluation failure modes:

- Contamination: public tasks leak into model training or agent memory. Mitigations include time-versioned live releases, private holdouts, canary strings, paraphrase/mutation, and fresh customer task extraction.
- Judge bias: LLM judges prefer position, verbosity, familiar models, or their own outputs. Mitigations include pairwise randomization, position debiasing, multi-judge consensus, calibration tasks, and inter-rater agreement checks.
- Replay nondeterminism: package versions, network data, time, flaky tests, and changing model APIs invalidate comparisons. Mitigations include pinned commits, cached dependencies, frozen task artifacts, judge-call caching, and captured environment manifests.
- Capability elicitation: the harness and scaffolding are part of measured capability. Reports must distinguish model capability from harness/scaffold quality.
- Sandbagging and gaming: agents can optimize visible validators while ignoring hidden intent. Mitigations include hidden tests, anti-gaming judge clauses, and collateral-damage checks.
- Privacy and IP: customer code, secrets, traces, and PR diffs may be sent to third-party LLMs. Mitigations include provider controls, redaction, audit logs, retention controls, and explicit training-data settings.

### 7. Private And Public Benchmarks Must Be Separated

Public leaderboards are useful but vulnerable to contamination and overfitting. Customer harnesses need private suites that can hide validators and expected artifacts. Reports should distinguish:

- public benchmark results
- private customer task-bank results
- live production/replay results
- ad hoc one-off tasks

## AgentClash Gaps To Close

### Product Gaps

- No first-class harness benchmark suites.
- No multi-agent comparison page for the same task/suite.
- No arena-mode statistics for paired agent races.
- No hidden validation or private task-bank workflow.
- No buyer-facing report that answers "Is this harness right for my repo?"
- No failure taxonomy dashboard.
- Limited live run viewer for long autonomous work.
- No customer-signal loop in the product for turning real failed/successful harness runs into curated private eval tasks.

### Backend Gaps

- Harness LLM judges are skipped.
- Harness score is not a normal AgentClash scorecard.
- Harness validators are command-only.
- No persisted harness artifacts table/linkage for diffs, PRs, logs, and judge results.
- No budget/retry/pass@k model.
- No time-horizon or human-baseline model.
- No automatic setup/bootstrap stage.
- No judge calibration or structured judge result storage for harnesses.
- No contamination, replay determinism, or hidden-eval controls in the harness layer.

### UI Gaps

- The create flow is improved over the old boring form, but it still hides the evaluation model and benchmark-suite concept.
- The latest-event table is useful, but long-running coding agents need a full replay/debugger view.
- There is no side-by-side comparison of two harnesses solving the same task.
- There is no scorecard panel for harness runs.

### Infrastructure Gaps

- E2B template strategy needs a scalable path for arbitrary Rust, Go, Python, JS, Java, and polyglot repos.
- Secret and auth UX is still provider API-key based for agent runtime auth.
- GitHub App integration exists for structured harnesses, but needs to become the default workflow for repo selection, PR evidence, and permissions.
- Network/secrets/tool policy needs explicit reporting for security-sensitive customers.
- Enterprise buyers will ask about code IP, data residency, third-party model retention, trace redaction, and audit logs before trusting customer-repo evals.

## Recommended Target Architecture

### Core Objects

- `agent_harness`: a runner definition, such as Codex on E2B, Claude on E2B, OpenHands, Aider, custom CLI, or future provider.
- `agent_harness_task`: a single task contract over a repo/ref/environment.
- `agent_harness_suite`: a versioned task set with public/private metadata.
- `agent_harness_run`: one execution of one harness on one task.
- `agent_harness_scorecard`: persisted score dimensions and evidence links.
- `agent_harness_artifact`: diff, changed files, PR metadata, logs, generated files, screenshots, validator output.
- `agent_harness_trace_event`: normalized event stream from worker, sandbox, and agent.
- `agent_harness_race`: a paired or multi-agent experiment over the same task/suite with fairness constraints.
- `agent_harness_rating`: aggregate ranking metadata, such as win rate, Bradley-Terry/Elo-like rating, confidence interval, and cost/time-normalized rank.

### Execution Lifecycle

1. Resolve repo, base ref, secrets, and harness runtime.
2. Provision sandbox.
3. Run setup/bootstrap.
4. Run agent with budget and event capture.
5. Capture diff, changed files, artifacts, PR metadata.
6. Run deterministic validators.
7. Run LLM/human judges if configured.
8. Build scorecard and classify failures.
9. Aggregate into suite ranking.

Implementation note: AgentClash already has eval-session aggregation, repeated-comparison evidence, run comparisons, and release gates in the challenge-pack/run path. Harness suites and ratings should first evaluate whether those primitives can be reused or generalized before introducing a separate harness-only aggregator.

### Roadmap Priorities

P0:

1. Wire harness scoring into the existing scorecard engine: https://github.com/agentclash/agentclash/issues/583
   - This fixes the largest credibility gap: `llm_judges.skipped` and command-only score.
2. Add harness tasks/suites: https://github.com/agentclash/agentclash/issues/581
   - This turns one-off runs into evaluable experiments.
3. Build the run replay and comparison UI: https://github.com/agentclash/agentclash/issues/582
   - This addresses the user-visible "I can only see Run" problem and makes failures debuggable.
4. Add hidden/private validator support and artifact-linked evidence: https://github.com/agentclash/agentclash/issues/586
   - This makes customer-private evaluations credible.

P1:

5. Add multi-run ranking with success@1, pass@k, cost/time budgets, confidence intervals, and race-mode comparisons: https://github.com/agentclash/agentclash/issues/584
6. Add environment bootstrap and setup diagnostics for arbitrary tech stacks: https://github.com/agentclash/agentclash/issues/585
7. Add failure taxonomy and LLM-assisted trace review: https://github.com/agentclash/agentclash/issues/587
8. Add GitHub-issue/task ingestion and customer private task banks: https://github.com/agentclash/agentclash/issues/581 and https://github.com/agentclash/agentclash/issues/587
9. Add run-control and orchestration primitives for long harness work: cancellation, idempotent retry, persisted workflow handles, workspace concurrency caps, and cleanup guarantees.

P2:

9. Add public benchmark adapters for SWE-bench-like and Terminal-Bench-like suites.
10. Add human baseline workflows for high-value customer task suites.
11. Add governance controls for trace retention, model-provider data use, and enterprise audit.

## What AgentClash Can Do Better Than Competitors

- Be provider-neutral for coding agents: Codex, Claude, OpenHands, Aider, Jules-like runners, custom CLIs.
- Evaluate on the customer's actual repositories and GitHub issues, not only public benchmark tasks.
- Produce PR-native evidence: branch, commits, diff, changed files, CI/validator output, reviewable artifact.
- Reuse the existing challenge-pack scoring engine without making users author challenge packs.
- Combine benchmark rigor with arena UX: run, watch, compare, score, and decide.
- Make private customer task banks first-class while still supporting public benchmark adapters.

## Customer Signal To Collect Next

Desk research cannot settle which roadmap items should lead. Before a large implementation push, AgentClash should interview or observe:

- teams choosing between Codex, Claude Code, Copilot, Devin, Cursor, Replit Agent, and OpenHands
- teams with private repos in Rust/Go/JS/Python who worry about setup reliability
- teams with compliance constraints around sending code to model providers
- users who already tried Agent Harnesses and failed because of auth, templates, missing live trace, or unclear scoring

Questions to answer:

- What decision are they trying to make: model selection, tool selection, vendor procurement, release gating, or debugging?
- Do they need public leaderboard comparability or private repo relevance more?
- What evidence would make them trust an agent-authored PR?
- What budget dimensions matter: wall time, token cost, engineer review time, CI time, or risk?

## Source Index

- SWE-bench Verified: https://www.swebench.com/verified.html
- SWE-rebench: https://openreview.net/forum?id=nMpJoVmRy1
- Multi-SWE-bench: https://github.com/multi-swe-bench/multi-swe-bench and https://arxiv.org/abs/2504.02605
- SWE-Lancer: https://openai.com/index/swe-lancer/
- Terminal-Bench: https://terminalbench.lol/ and https://www.tbench.ai/
- OpenHands evaluation harness: https://docs.openhands.dev/openhands/usage/developers/evaluation-harness
- PaperBench: https://openai.com/index/paperbench/
- Commit-0: https://commit-0.github.io/
- WebArena-x: https://webarena.dev/
- WebArena-Infinity: https://github.com/web-arena-x/webarena-infinity
- AssistantBench: https://assistantbench.github.io/
- OSWorld: https://arxiv.org/abs/2404.07972
- GAIA: https://ai.meta.com/research/publications/gaia-a-benchmark-for-general-ai-assistants/
- HCAST: https://metr.org/hcast.pdf
- tau-bench: https://github.com/sierra-research/tau-bench
- AppWorld: https://appworld.dev/
- BFCL: https://gorilla.cs.berkeley.edu/leaderboard
- Chatbot Arena methodology: https://www.lmsys.org/blog/2023-12-07-leaderboard/ and https://arxiv.org/abs/2403.04132
- Copilot Arena: https://arxiv.org/abs/2502.09328
- WebDev Arena: https://webdev.lmarena.ai/leaderboard and https://arena.ai/blog/webdev-arena/
- Aider leaderboards: https://aider.chat/docs/leaderboards/
- OpenAI Evals: https://github.com/openai/evals
- EleutherAI lm-evaluation-harness: https://github.com/EleutherAI/lm-evaluation-harness
- Braintrust evals: https://www.braintrust.dev/docs/evaluate
- Langfuse scores: https://langfuse.com/docs/evaluation/scores/overview
- Phoenix evals: https://arize.com/docs/phoenix/evaluation/llm-evals
- Inspect AI: https://inspect.aisi.org.uk/
- METR Vivaria: https://github.com/METR/vivaria
- Promptfoo: https://www.promptfoo.dev/docs/intro/
- DeepEval: https://deepeval.com/docs/introduction
- E2B docs: https://e2b.dev/docs/quickstart/migrating-from-v0
- Modal Sandboxes: https://modal.com/docs/guide/sandboxes
- OpenAI Codex: https://platform.openai.com/docs/codex and https://openai.com/codex
- GitHub Copilot coding agent PR docs: https://docs.github.com/en/copilot/how-tos/use-copilot-agents/coding-agent/create-a-pr
- Claude Code GitHub Actions: https://docs.claude.com/en/docs/claude-code/github-actions
- Jules: https://jules.google/docs
