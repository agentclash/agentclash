# Agent Harnesses: Codex on E2B

Agent Harnesses are workspace-scoped coding-agent task definitions. They are not
challenge packs: users describe a task, choose a harness runtime, and attach
evaluation config that can later be scored with AgentClash validators and LLM
judges.

## Research Summary

- E2B provides a prebuilt `codex` sandbox template with filesystem, terminal,
  and git access. Their Codex guide shows `e2b sbx create codex` for interactive
  use and `codex exec --full-auto --skip-git-repo-check` for headless execution
  inside the sandbox. Source: https://e2b.dev/docs/agents/codex
- E2B's coding-agent guide frames this as the intended isolation model for
  autonomous coding agents: create an isolated sandbox, let the agent work with
  terminal/filesystem/git, then extract a diff or structured output. Source:
  https://e2b.dev/docs/use-cases/coding-agents
- Codex CLI supports ChatGPT login, device-code login for headless machines, and
  API-key login. The OpenAI Codex CLI auth docs recommend ChatGPT login where
  available, while API-key auth is the reliable non-interactive option when a
  worker cannot complete an OAuth/device flow itself. Source:
  https://www.mintlify.com/openai/codex/authentication
- Codex exec mode is intended for automation. The docs describe JSON output,
  schema-constrained final messages, `--full-auto`, and API keys as CI-friendly
  execution pieces. Source:
  https://www.mintlify.com/openai/codex/advanced/exec-mode
- Codex cloud itself runs background tasks in isolated cloud environments and
  can create PRs from connected repositories, but this PR models the CLI-on-E2B
  path because AgentClash needs to orchestrate and score runs itself. Source:
  https://developers.openai.com/codex/cloud
- OpenHands' benchmark harnesses separate runtime setup, agent execution,
  trajectory/history capture, metrics, and final evaluation output. Their docs
  show benchmark code creating a sandbox/runtime, running the controller, then
  returning an `EvalOutput` with history, metrics, test result, and errors.
  Source: https://docs.openhands.dev/openhands/usage/developers/evaluation-harness
- The OpenHands benchmarks repository also highlights real-time rich logging of
  tool calls and end-of-instance summaries, which is the shape Agent Harnesses
  should mirror for useful debugging and ranking. Source:
  https://github.com/OpenHands/benchmarks

## Product Shape

The first implementation stores a `codex_e2b` harness with:

- the human task prompt
- E2B template ID, defaulting to `codex`
- optional Codex model override
- auth mode:
  - `chatgpt_device` for user-assisted ChatGPT login
  - `api_key_secret` for workspace-secret-backed API execution
  - `bring_your_own_env` for externally prepared sandboxes
- optional repository URL and base branch
- JSON `execution_config`
- JSON `evaluation_config` for validators, LLM judges, metrics, and scoring

The execution worker should set both `OPENAI_API_KEY` and `CODEX_API_KEY` from
the configured OpenAI secret when running the E2B Codex template. The Codex CLI
docs emphasize `OPENAI_API_KEY`; E2B's Codex example uses `CODEX_API_KEY`.
Setting both avoids coupling AgentClash to a template-specific environment name.

## Next Execution Step

A follow-up worker can turn a persisted harness into an execution by:

1. Resolving the workspace secrets for E2B and OpenAI.
2. Creating an E2B sandbox from `codex_template`.
3. Cloning `repository_url` at `base_branch` when provided.
4. Running `codex exec --full-auto --skip-git-repo-check --json`.
5. Persisting stdout/stderr, JSONL events, final message, and git diff as
   artifacts.
6. Feeding artifacts plus `evaluation_config` into the existing scoring layer.

This keeps challenge packs optional for coding-harness evaluations while still
letting AgentClash reuse validators and LLM judges.

## Runtime Lessons

- E2B command execution must receive the OpenAI/Codex key in the per-command
  environment. Sandbox-level env is useful for templates, but the AgentClash E2B
  process bridge does not assume command inheritance.
- Use Codex's `-C <repo>` flag in addition to setting the process working
  directory. This matches the E2B Codex examples and makes the intended repo
  context explicit in the recorded command trace.
- A harness run is not complete when Codex exits. It must run configured
  validators, record pass/fail scoring events, and fail the execution when a
  required validator fails.
