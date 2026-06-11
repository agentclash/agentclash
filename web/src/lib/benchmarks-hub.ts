export const BENCHMARKS_HUB_FAQ = [
  {
    question: "How is AgentClash different from a static leaderboard?",
    answer:
      "Leaderboards summarize one-off scores on generic tasks. AgentClash runs head-to-head races on frozen challenge packs with the same tools, sandbox policy, and iteration budget, then publishes replay evidence and scorecards you can reuse as regression gates.",
  },
  {
    question: "What gets frozen in a public benchmark?",
    answer:
      "The challenge pack version, runtime constraints, tool policy, scoring spec, and input cases. Every model in a race sees the same workload so differences show up in trajectories, not setup drift.",
  },
  {
    question: "Can we run the same benchmark on our agents?",
    answer:
      "Yes. Book an eval workshop or start a Team pilot. We pin the same pack on your workloads, baseline your current agent, and deliver scorecards and CI gates your platform team can ship on.",
  },
  {
    question: "How often do you publish benchmark reports?",
    answer:
      "We publish when major models ship and on a monthly reliability cadence. Subscribe to the benchmarks RSS feed or check this hub for the latest head-to-head summary and links to full replays.",
  },
] as const;

export const BENCHMARKS_METHODOLOGY = [
  {
    title: "Frozen challenge pack",
    text: "Each race pins a versioned YAML pack: prompts, tools, sandbox, evaluation spec, and input cases. No moving targets mid-benchmark.",
  },
  {
    title: "Same runtime constraints",
    text: "Every candidate gets the same sandbox, tool policy, network rules, and iteration budget so comparisons stay fair.",
  },
  {
    title: "Baseline vs candidate",
    text: "Enterprise teams reuse the same packs to compare a ship candidate against a known baseline and fail CI when scorecards regress.",
  },
  {
    title: "Scorecard dimensions",
    text: "Composite scores fold in correctness, reliability, latency, cost, behavioral signals, and judge evidence from the full trajectory.",
  },
  {
    title: "Replay and evidence",
    text: "Runs preserve tool calls, artifacts, and judge rationale so reviewers can audit a verdict without rerunning the race.",
  },
  {
    title: "Gate verdict",
    text: "The same workload can power a public benchmark today and a release gate tomorrow once your team trusts the scoring rules.",
  },
] as const;

export const BENCHMARKS_CHILD_PAGES = [
  {
    href: "/ai-agent-benchmark",
    title: "AI agent benchmark",
    text: "Benchmark agents on workloads your team owns, with replay and scorecards instead of leaderboard-only snapshots.",
  },
  {
    href: "/agent-reliability-benchmark",
    title: "Agent reliability benchmark",
    text: "Track pass rates, cost drift, and promoted failures with repeatable packs and CI regression gates.",
  },
] as const;

export const BENCHMARKS_READING = [
  {
    href: "/blog/benchmark-ai-agents-on-your-own-data",
    title: "Benchmark AI agents on your own data",
  },
  {
    href: "/blog/how-agentclash-scores-agent-trajectories",
    title: "How AgentClash scores agent trajectories",
  },
  {
    href: "/blog/pass-k-reliability-enterprise-teams",
    title: "Pass@k reliability for enterprise teams",
  },
  {
    href: "/blog/why-agentclash-races-agents-head-to-head",
    title: "Why AgentClash races agents head-to-head",
  },
] as const;

export const BENCHMARKS_PACK_LINKS = [
  {
    href: "/docs/challenge-packs",
    title: "Challenge pack reference",
    text: "Author versioned YAML packs with scoring, tools, sandbox policy, and eval workflows.",
  },
  {
    href: "/docs/challenge-packs/eval-workflows-and-gates",
    title: "Eval workflows and gates",
    text: "Wire baseline versus candidate comparisons into CI release policy.",
  },
  {
    href: "https://github.com/agentclash/agentclash/tree/main/examples/challenge-packs",
    title: "Example packs in the repo",
    text: "Start from expression evaluators, refund recovery, incident response, and security stress packs.",
    external: true,
  },
] as const;

export const BENCHMARKS_MONTHLY_PROCESS = [
  "Pin a challenge pack version and publish the runtime constraints up front.",
  "Race every candidate on the same pack with identical tools, sandbox, and budgets.",
  "Score the full trajectory across correctness, reliability, latency, cost, and judge evidence.",
  "Attach replay links and artifact references reviewers can audit without rerunning.",
  "Summarize winners, regressions, and gate verdicts on this hub and in the RSS feed.",
] as const;
