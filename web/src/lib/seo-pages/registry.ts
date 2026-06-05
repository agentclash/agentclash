import { AGENT_EVALUATION_FEATURES } from "@/lib/seo-features";
import type { SeoPageConfig } from "./types";

const sharedProofPoints = AGENT_EVALUATION_FEATURES;

const sharedDocsLinks = [
  {
    title: "Quickstart",
    text: "Validate the CLI and get to your first runnable command.",
    href: "/docs/getting-started/quickstart",
  },
  {
    title: "Write a challenge pack",
    text: "Turn a real task into a repeatable agent evaluation.",
    href: "/docs/guides/write-a-challenge-pack",
  },
  {
    title: "CI/CD agent gates",
    text: "Fail a pull request when an agent regresses.",
    href: "/docs/guides/ci-cd-agent-gates",
  },
] as const;

function page(
  config: Omit<
    SeoPageConfig,
    "proofPoints" | "relatedLinks" | "workflow" | "faqItems"
  > & {
    proofPoints?: string[];
    relatedLinks?: SeoPageConfig["relatedLinks"];
    workflow?: SeoPageConfig["workflow"];
    faqItems: SeoPageConfig["faqItems"];
  },
): SeoPageConfig {
  return {
    proofPoints: sharedProofPoints,
    relatedLinks: [...sharedDocsLinks],
    workflow: [
      {
        title: "Package the task",
        text: "Describe the workload as a challenge pack with inputs, tools, scoring rules, and artifacts.",
      },
      {
        title: "Race the agents",
        text: "Run every candidate against the same task with the same constraints.",
      },
      {
        title: "Replay the evidence",
        text: "Inspect tool calls, outputs, artifacts, latency, cost, and judge evidence after the run.",
      },
      {
        title: "Gate the release",
        text: "Compare candidate and baseline runs, then fail CI before a regression reaches users.",
      },
    ],
    ...config,
  };
}

export const SEO_PAGE_REGISTRY: SeoPageConfig[] = [
  page({
    path: "/open-source-ai-agent-evaluation",
    tier: "S",
    keyword: "open source AI agent evaluation",
    intent: "OSS users/founders",
    pageTitle: "Open Source AI Agent Evaluation Platform - AgentClash",
    metaDescription:
      "AgentClash is an open-source AI agent evaluation platform for real-task races, replay evidence, scorecards, challenge packs, and CI regression gates.",
    socialImageAlt: "AgentClash open source AI agent evaluation social preview.",
    eyebrow: "Open source",
    h1: "Open source AI agent evaluation for real tasks",
    heroDescription:
      "AgentClash is MIT-licensed and self-hostable. Run head-to-head agent races on your workloads, keep replay evidence, and turn failures into reusable regression gates without a black-box vendor loop.",
    proofSectionTitle: "What open-source eval should include",
    proofSectionDescription:
      "Open source matters when your eval stack needs to run inside your repo, your CI, and your sandbox policy — not just inside someone else's hosted UI.",
    workflowSectionTitle: "From local race to team-wide gate",
    docsSectionTitle: "Start with the repo",
    docsSectionDescription:
      "Clone AgentClash, boot the local stack, and wire the same challenge packs into hosted runs or pull request gates.",
    faqSectionTitle: "Questions OSS teams ask before adopting",
    applicationSubCategory: "Open source AI agent evaluation platform",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "Open source AI agent evaluation", url: "/open-source-ai-agent-evaluation" },
    ],
    schemaId: "agentclash-open-source-ai-agent-evaluation-schema",
    searchKeywords:
      "open source AI agent evaluation OSS agent eval platform self-hosted agent evaluation MIT agent benchmark replay scorecards challenge packs",
    sitemapTitle: "Open source AI agent evaluation",
    sitemapDescription:
      "MIT-licensed, self-hostable AI agent evaluation with replay, scorecards, and CI gates.",
    faqItems: [
      {
        question: "Is AgentClash actually open source?",
        answer:
          "Yes. AgentClash is MIT-licensed. You can self-host the API server, worker, and web app, or use the hosted product when that is faster for your team.",
      },
      {
        question: "Can we run evals entirely on our own infrastructure?",
        answer:
          "Yes. AgentClash supports local stacks, self-hosted deployments, and sandbox providers you control. Challenge packs and scorecards work the same whether the run is local or hosted.",
      },
      {
        question: "How does open source help with agent regression testing?",
        answer:
          "You can inspect scoring rules, pack definitions, replay artifacts, and CI gate manifests in git. That makes agent eval auditable instead of a dashboard-only black box.",
      },
    ],
    relatedLinks: [
      {
        title: "Self-host guide",
        text: "Boot Postgres, Temporal, API, and worker locally or in your cluster.",
        href: "/docs/getting-started/self-host",
      },
      ...sharedDocsLinks,
    ],
  }),
  page({
    path: "/agent-evals",
    tier: "S",
    keyword: "agent evals",
    intent: "Dev shorthand",
    pageTitle: "Agent Evals for Real Tasks, Replay, and CI Gates - AgentClash",
    metaDescription:
      "Run agent evals on real tasks with same-tools races, replay evidence, scorecards, challenge packs, and regression gates — not just final-answer checks.",
    socialImageAlt: "AgentClash agent evals social preview.",
    eyebrow: "Agent evals",
    h1: "Agent evals that cover the whole trajectory",
    heroDescription:
      "Agent evals should answer whether an agent can finish the job again under the same tools, budget, and constraints. AgentClash records replay evidence and scorecards so evals become release decisions.",
    proofSectionTitle: "What good agent evals capture",
    proofSectionDescription:
      "If your agent eval only checks the final string, you miss the tool misuse, runaway loops, and artifact gaps that show up in production.",
    workflowSectionTitle: "From one eval to a reusable gate",
    docsSectionTitle: "Run your first agent eval",
    docsSectionDescription:
      "Use challenge packs for repeatable workloads, then promote failures into regression cases your team can run in CI.",
    faqSectionTitle: "Agent eval FAQ",
    applicationSubCategory: "Agent evaluation software",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "Agent evals", url: "/agent-evals" },
    ],
    schemaId: "agentclash-agent-evals-schema",
    searchKeywords:
      "agent evals agent evaluation eval harness real task agent benchmark replay scorecards challenge packs CI gates",
    sitemapTitle: "Agent evals",
    sitemapDescription:
      "Real-task agent evals with replay evidence, scorecards, and CI regression gates.",
    faqItems: [
      {
        question: "What is an agent eval?",
        answer:
          "An agent eval is a repeatable test that runs an agent on a task, scores the full trajectory, and compares the result to a baseline or competitor.",
      },
      {
        question: "How is AgentClash different from prompt evals?",
        answer:
          "Prompt evals score one model response. AgentClash evals multi-turn agents that use tools in a sandbox and scores correctness, cost, latency, and evidence quality across the run.",
      },
      {
        question: "Can agent evals run in CI?",
        answer:
          "Yes. AgentClash can compare candidate and baseline scorecards and fail a pull request when the configured release gate regresses.",
      },
    ],
  }),
  page({
    path: "/llm-agent-evaluation",
    tier: "S",
    keyword: "LLM agent evaluation",
    intent: "Broad category",
    pageTitle: "LLM Agent Evaluation on Real Workloads - AgentClash",
    metaDescription:
      "Evaluate LLM agents on real tasks with sandboxed tool use, head-to-head races, replay trails, scorecards, and CI regression gates.",
    socialImageAlt: "AgentClash LLM agent evaluation social preview.",
    eyebrow: "LLM agents",
    h1: "LLM agent evaluation beyond single-turn answers",
    heroDescription:
      "LLM agents plan, call tools, inspect results, and recover from mistakes. AgentClash evaluates that full loop on your workloads so model swaps and prompt changes do not hide regressions.",
    proofSectionTitle: "Evaluate the agent loop, not just the model",
    proofSectionDescription:
      "LLM agent evaluation needs the same task, same tools, and preserved evidence — otherwise you are comparing demos, not systems.",
    workflowSectionTitle: "A practical LLM agent eval workflow",
    docsSectionTitle: "Bring your workload into AgentClash",
    docsSectionDescription:
      "Start with one real failure, encode it as a challenge pack, then scale to model comparisons and CI gates.",
    faqSectionTitle: "LLM agent evaluation FAQ",
    applicationSubCategory: "LLM agent evaluation platform",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "LLM agent evaluation", url: "/llm-agent-evaluation" },
    ],
    schemaId: "agentclash-llm-agent-evaluation-schema",
    searchKeywords:
      "LLM agent evaluation language model agent eval tool-using agents sandboxed workloads replay scorecards challenge packs",
    sitemapTitle: "LLM agent evaluation",
    sitemapDescription:
      "Evaluate tool-using LLM agents on real tasks with replay and scorecards.",
    faqItems: [
      {
        question: "What should LLM agent evaluation measure?",
        answer:
          "At minimum: task success, tool strategy, artifacts produced, cost, latency, and whether the agent stayed inside policy. AgentClash captures all of that in a scorecard.",
      },
      {
        question: "Can we compare multiple LLM agents fairly?",
        answer:
          "Yes. AgentClash races candidates on the same challenge pack with the same tool policy, time budget, and sandbox resources.",
      },
      {
        question: "Does AgentClash work with hosted model providers?",
        answer:
          "Yes. AgentClash routes to major LLM providers and normalizes tool-call shapes so evals stay comparable across models.",
      },
    ],
  }),
  page({
    path: "/agent-evaluation-framework",
    tier: "A",
    keyword: "agent evaluation framework",
    intent: "Devs comparing tools",
    pageTitle: "Agent Evaluation Framework for Real Tasks - AgentClash",
    metaDescription:
      "Compare agent evaluation frameworks on what matters: sandboxed execution, replay evidence, scorecards, challenge packs, and CI regression gates.",
    socialImageAlt: "AgentClash agent evaluation framework social preview.",
    eyebrow: "Framework",
    h1: "An agent evaluation framework built for production tasks",
    heroDescription:
      "Teams comparing agent evaluation frameworks should look past leaderboard scores. AgentClash gives you repeatable workloads, head-to-head races, replay evidence, and release gates you can audit in git.",
    proofSectionTitle: "What a serious framework includes",
    proofSectionDescription:
      "A useful agent evaluation framework packages tasks, enforces fair constraints, scores trajectories, and makes failures reusable.",
    workflowSectionTitle: "Framework workflow",
    docsSectionTitle: "Evaluate before you commit",
    docsSectionDescription:
      "Use AgentClash alongside prompt-eval tools when you need end-to-end agent behavior, not single-call scoring.",
    faqSectionTitle: "Framework comparison FAQ",
    applicationSubCategory: "Agent evaluation framework",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "Agent evaluation framework", url: "/agent-evaluation-framework" },
    ],
    schemaId: "agentclash-agent-evaluation-framework-schema",
    searchKeywords:
      "agent evaluation framework compare agent eval tools agent testing framework replay scorecards challenge packs CI gates",
    sitemapTitle: "Agent evaluation framework",
    sitemapDescription:
      "Framework for real-task agent evaluation with replay, scorecards, and CI gates.",
    faqItems: [
      {
        question: "How is AgentClash different from prompt-evaluation frameworks?",
        answer:
          "Prompt-evaluation frameworks score isolated model outputs. AgentClash is an agent-evaluation framework for multi-turn tool-using runs in a sandbox.",
      },
      {
        question: "Can we compare AgentClash with other tools?",
        answer:
          "Yes. See the compare hub for side-by-side notes with Braintrust, LangSmith, Promptfoo, Langfuse, Arize Phoenix, and OpenAI Evals.",
      },
      {
        question: "Does the framework support custom scoring?",
        answer:
          "Yes. Challenge packs carry scoring rules, validators, and judge configuration so teams can encode domain-specific pass conditions.",
      },
    ],
    relatedLinks: [
      {
        title: "Compare tools",
        text: "See how AgentClash differs from prompt-eval platforms.",
        href: "/compare",
      },
      ...sharedDocsLinks,
    ],
  }),
  page({
    path: "/ai-agent-testing",
    tier: "A",
    keyword: "AI agent testing",
    intent: "Simpler language",
    pageTitle: "AI Agent Testing with Replay and Release Gates - AgentClash",
    metaDescription:
      "Test AI agents on real tasks with sandboxed execution, replay evidence, scorecards, challenge packs, and CI gates that block regressions.",
    socialImageAlt: "AgentClash AI agent testing social preview.",
    eyebrow: "Agent testing",
    h1: "AI agent testing that looks like release engineering",
    heroDescription:
      "AI agent testing should feel closer to software testing than prompt tweaking. AgentClash turns real failures into repeatable challenge packs, compares candidates to baselines, and keeps the evidence reviewers need.",
    proofSectionTitle: "What production-grade agent testing covers",
    proofSectionDescription:
      "Testing agents means checking behavior across the whole run — not approving a single polished answer from a cherry-picked prompt.",
    workflowSectionTitle: "Test loop",
    docsSectionTitle: "Start testing with docs",
    docsSectionDescription:
      "Write a challenge pack, run a race, inspect replay, then wire the same workload into CI.",
    faqSectionTitle: "AI agent testing FAQ",
    applicationSubCategory: "AI agent testing software",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "AI agent testing", url: "/ai-agent-testing" },
    ],
    schemaId: "agentclash-ai-agent-testing-schema",
    searchKeywords:
      "AI agent testing test AI agents agent QA replay scorecards challenge packs regression testing CI gates",
    sitemapTitle: "AI agent testing",
    sitemapDescription:
      "Test AI agents on real tasks with replay evidence and CI regression gates.",
    faqItems: [
      {
        question: "Is AI agent testing different from LLM testing?",
        answer:
          "Yes. LLM testing often means scoring one response. AI agent testing evaluates plans, tool calls, artifacts, recovery behavior, and whether the task actually finished.",
      },
      {
        question: "Can non-ML engineers review agent test failures?",
        answer:
          "Yes. Replay timelines, artifacts, and scorecards are designed for reviewers who need to understand what changed without reading raw model traces alone.",
      },
      {
        question: "How do we keep tests from getting stale?",
        answer:
          "Promote escaped production failures into challenge packs and regression suites so the same mistake stays covered after the next model swap.",
      },
    ],
  }),
  page({
    path: "/agent-trajectory-evaluation",
    tier: "A",
    keyword: "agent trajectory evaluation",
    intent: "Technical buyer",
    pageTitle: "Agent Trajectory Evaluation and Replay Evidence - AgentClash",
    metaDescription:
      "Evaluate agent trajectories with replay trails, tool-call evidence, scorecards, challenge packs, and CI gates for baseline versus candidate decisions.",
    socialImageAlt: "AgentClash agent trajectory evaluation social preview.",
    eyebrow: "Trajectories",
    h1: "Agent trajectory evaluation with reviewable evidence",
    heroDescription:
      "The final answer is not enough. AgentClash evaluates the trajectory — tool choices, observations, retries, artifacts, and stop conditions — then preserves replay evidence for auditors and release owners.",
    proofSectionTitle: "Why trajectories matter",
    proofSectionDescription:
      "Two agents can return the same answer while taking wildly different paths. Trajectory evaluation catches unsafe shortcuts, runaway loops, and brittle tool strategies.",
    workflowSectionTitle: "Trajectory eval workflow",
    docsSectionTitle: "Inspect runs with docs",
    docsSectionDescription:
      "Use replay and scorecards to debug trajectory regressions, then encode the workload as a challenge pack for CI.",
    faqSectionTitle: "Trajectory evaluation FAQ",
    applicationSubCategory: "Agent trajectory evaluation software",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "Agent trajectory evaluation", url: "/agent-trajectory-evaluation" },
    ],
    schemaId: "agentclash-agent-trajectory-evaluation-schema",
    searchKeywords:
      "agent trajectory evaluation trajectory scoring tool call replay agent trace evaluation scorecards challenge packs",
    sitemapTitle: "Agent trajectory evaluation",
    sitemapDescription:
      "Score full agent trajectories with replay evidence and release gates.",
    faqItems: [
      {
        question: "What is agent trajectory evaluation?",
        answer:
          "Trajectory evaluation scores the sequence of actions and observations an agent took to complete a task, not just the final output string.",
      },
      {
        question: "How does AgentClash store trajectory evidence?",
        answer:
          "Each run keeps replay events, tool calls, logs, artifacts, and scorecards so reviewers can reconstruct the path that produced the result.",
      },
      {
        question: "Can trajectory evals gate releases?",
        answer:
          "Yes. Compare candidate and baseline trajectories via scorecards, then fail CI when correctness, cost, latency, or evidence quality regresses.",
      },
    ],
    relatedLinks: [
      {
        title: "Agent replay",
        text: "See how AgentClash preserves replay evidence for reviewers.",
        href: "/features/agent-replay",
      },
      ...sharedDocsLinks,
    ],
  }),
  page({
    path: "/ci-cd-agent-evaluation",
    tier: "A",
    keyword: "agent evaluation CI/CD",
    intent: "Release engineering",
    pageTitle: "CI/CD Agent Evaluation and Regression Gates - AgentClash",
    metaDescription:
      "Wire agent evaluation into CI/CD with baseline comparisons, challenge packs, replay evidence, scorecards, and pull request gates.",
    socialImageAlt: "AgentClash CI/CD agent evaluation social preview.",
    eyebrow: "CI/CD",
    h1: "CI/CD agent evaluation for pull request gates",
    heroDescription:
      "Model, prompt, and tool changes should not ship on vibes. AgentClash runs repeatable agent workloads in CI, compares candidate scorecards to baselines, and blocks merges when behavior regresses.",
    proofSectionTitle: "What CI/CD agent eval needs",
    proofSectionDescription:
      "Release engineering needs deterministic workloads, stable scoring, and enough evidence to debug a failed gate without reproducing the issue manually.",
    workflowSectionTitle: "CI gate workflow",
    docsSectionTitle: "Wire gates with docs",
    docsSectionDescription:
      "Start with the CI/CD agent gates guide, then promote real failures into challenge packs your pipeline can rerun on every change.",
    faqSectionTitle: "CI/CD agent evaluation FAQ",
    applicationSubCategory: "CI/CD agent evaluation software",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "CI/CD agent evaluation", url: "/ci-cd-agent-evaluation" },
    ],
    schemaId: "agentclash-ci-cd-agent-evaluation-schema",
    searchKeywords:
      "CI/CD agent evaluation agent eval CI pull request gates release gates baseline candidate regression testing challenge packs",
    sitemapTitle: "CI/CD agent evaluation",
    sitemapDescription:
      "Run agent evaluation in CI/CD with scorecard gates and replay evidence.",
    faqItems: [
      {
        question: "How do agent eval gates fit into CI/CD?",
        answer:
          "A challenge pack runs on every candidate change, AgentClash compares the scorecard to a baseline, and the pipeline fails when configured thresholds regress.",
      },
      {
        question: "What happens when a gate fails?",
        answer:
          "Reviewers open replay evidence, inspect tool calls and artifacts, and either fix the regression or update the baseline intentionally.",
      },
      {
        question: "Can gates cover cost and latency budgets?",
        answer:
          "Yes. Scorecards include correctness and evidence quality plus cost and latency signals you can enforce in release policy.",
      },
    ],
    relatedLinks: [
      {
        title: "Agent regression testing",
        text: "Product overview for baseline versus candidate agent testing.",
        href: "/platform/agent-regression-testing",
      },
      ...sharedDocsLinks,
    ],
  }),
  page({
    path: "/ai-agent-benchmark",
    tier: "A",
    keyword: "AI agent benchmark",
    intent: "Benchmark traffic",
    pageTitle: "AI Agent Benchmark on Real Tasks - AgentClash",
    metaDescription:
      "Run AI agent benchmarks on real workloads with fair constraints, replay evidence, scorecards, and regression gates — not leaderboard-only snapshots.",
    socialImageAlt: "AgentClash AI agent benchmark social preview.",
    eyebrow: "Benchmarks",
    h1: "AI agent benchmarks grounded in real workloads",
    heroDescription:
      "Leaderboards are a starting point, not a release decision. AgentClash lets teams benchmark agents on the tasks they actually ship — with the same tools, budgets, and evidence requirements.",
    proofSectionTitle: "Better than a one-number benchmark",
    proofSectionDescription:
      "A useful AI agent benchmark reports correctness, cost, latency, tool strategy, and artifact quality on workloads your team owns.",
    workflowSectionTitle: "Benchmark workflow",
    docsSectionTitle: "Build a benchmark you can reuse",
    docsSectionDescription:
      "Encode workloads as challenge packs so benchmark runs become regression gates instead of one-off demos.",
    faqSectionTitle: "AI agent benchmark FAQ",
    applicationSubCategory: "AI agent benchmark software",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "AI agent benchmark", url: "/ai-agent-benchmark" },
    ],
    schemaId: "agentclash-ai-agent-benchmark-schema",
    searchKeywords:
      "AI agent benchmark agent benchmark real task benchmark coding agent benchmark replay scorecards challenge packs",
    sitemapTitle: "AI agent benchmark",
    sitemapDescription:
      "Benchmark AI agents on real tasks with replay and scorecards.",
    faqItems: [
      {
        question: "How is AgentClash different from public leaderboards?",
        answer:
          "Public leaderboards summarize generic tasks. AgentClash benchmarks your agents on your tools, repositories, APIs, and release constraints.",
      },
      {
        question: "Can we benchmark multiple agents head-to-head?",
        answer:
          "Yes. AgentClash races candidates on the same challenge pack with the same sandbox policy and produces comparable scorecards.",
      },
      {
        question: "Can a benchmark become a regression test?",
        answer:
          "Yes. The same challenge pack can power ad-hoc benchmarks and CI gates once your team trusts the scoring rules.",
      },
    ],
  }),
  page({
    path: "/agent-reliability-benchmark",
    tier: "A",
    keyword: "agent reliability benchmark",
    intent: "Better than generic eval",
    pageTitle: "Agent Reliability Benchmark and Regression Gates - AgentClash",
    metaDescription:
      "Measure agent reliability with repeatable workloads, pass-rate tracking, replay evidence, scorecards, and CI gates for baseline versus candidate runs.",
    socialImageAlt: "AgentClash agent reliability benchmark social preview.",
    eyebrow: "Reliability",
    h1: "Agent reliability benchmarks your team can ship on",
    heroDescription:
      "Reliability is repeatability under constraint. AgentClash benchmarks how often agents finish real tasks correctly, how much evidence they produce, and whether new changes make outcomes worse.",
    proofSectionTitle: "Reliability signals that matter",
    proofSectionDescription:
      "Track success rate, cost stability, latency drift, tool misuse, and promoted failures — not just whether one demo looked impressive.",
    workflowSectionTitle: "Reliability workflow",
    docsSectionTitle: "Turn reliability into gates",
    docsSectionDescription:
      "Use challenge packs and regression suites to keep reliability benchmarks current as models and tools change.",
    faqSectionTitle: "Agent reliability FAQ",
    applicationSubCategory: "Agent reliability benchmark software",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "Agent reliability benchmark", url: "/agent-reliability-benchmark" },
    ],
    schemaId: "agentclash-agent-reliability-benchmark-schema",
    searchKeywords:
      "agent reliability benchmark reliability eval pass rate regression testing replay scorecards challenge packs",
    sitemapTitle: "Agent reliability benchmark",
    sitemapDescription:
      "Benchmark agent reliability on real tasks with regression gates.",
    faqItems: [
      {
        question: "What makes an agent reliability benchmark useful?",
        answer:
          "It reruns the same real workloads over time, tracks pass rates and cost/latency drift, and preserves evidence when a run fails.",
      },
      {
        question: "How does AgentClash handle flaky agent behavior?",
        answer:
          "Teams can rerun workloads, inspect replay differences, and encode pass@k-style reliability policies in challenge packs and release gates.",
      },
      {
        question: "Can reliability benchmarks block release?",
        answer:
          "Yes. Compare candidate and baseline scorecards in CI and fail the gate when reliability metrics cross your threshold.",
      },
    ],
    relatedLinks: [
      {
        title: "pass@k vs pass^k",
        text: "Read when strict success and independent retries measure different things.",
        href: "/blog/pass-at-k-vs-pass-power-k",
      },
      ...sharedDocsLinks,
    ],
  }),
  page({
    path: "/use-cases/coding-agent-evaluation",
    tier: "B",
    keyword: "evaluate coding agents",
    intent: "Use-case page",
    pageTitle: "Coding Agent Evaluation on Real Repos - AgentClash",
    metaDescription:
      "Evaluate coding agents on real repositories with sandboxed execution, replay evidence, scorecards, challenge packs, and CI regression gates.",
    socialImageAlt: "AgentClash coding agent evaluation social preview.",
    eyebrow: "Use case",
    h1: "Evaluate coding agents on real repositories",
    heroDescription:
      "Coding agents fail on messy repos, flaky tests, and tool limits — not on polished benchmark prompts. AgentClash evaluates patches, test runs, artifacts, and cost on workloads your team actually ships.",
    proofSectionTitle: "What coding agent eval should check",
    proofSectionDescription:
      "Look for correct fixes, sane tool usage, reproducible artifacts, and stable cost/latency — not just a plausible diff in a demo.",
    workflowSectionTitle: "Coding eval workflow",
    docsSectionTitle: "Start with a real bug",
    docsSectionDescription:
      "Promote a failed coding-agent run into a challenge pack, then race model and harness changes before they reach users.",
    faqSectionTitle: "Coding agent evaluation FAQ",
    applicationSubCategory: "Coding agent evaluation software",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "Use cases", url: "/use-cases" },
      { name: "Coding agent evaluation", url: "/use-cases/coding-agent-evaluation" },
    ],
    schemaId: "agentclash-coding-agent-evaluation-schema",
    searchKeywords:
      "coding agent evaluation evaluate coding agents software engineering agents repo eval replay scorecards challenge packs",
    sitemapTitle: "Coding agent evaluation",
    sitemapDescription:
      "Evaluate coding agents on real repositories with replay and CI gates.",
    faqItems: [
      {
        question: "Can AgentClash evaluate agents that edit code?",
        answer:
          "Yes. Challenge packs can run agents in sandboxes with repository fixtures, test commands, and artifact checks for patches and logs.",
      },
      {
        question: "How do teams compare coding agents fairly?",
        answer:
          "Run every candidate on the same repo state, tool policy, and time budget, then compare scorecards and replay evidence.",
      },
      {
        question: "Can coding agent evals run in CI?",
        answer:
          "Yes. Wire challenge packs into pull request gates so model, prompt, or tool changes cannot merge when correctness regresses.",
      },
    ],
  }),
  page({
    path: "/use-cases/research-agent-evaluation",
    tier: "B",
    keyword: "evaluate research agents",
    intent: "Use-case page",
    pageTitle: "Research Agent Evaluation with Evidence Quality - AgentClash",
    metaDescription:
      "Evaluate research agents on real investigation tasks with replay evidence, artifact checks, scorecards, challenge packs, and regression gates.",
    socialImageAlt: "AgentClash research agent evaluation social preview.",
    eyebrow: "Use case",
    h1: "Evaluate research agents with evidence quality",
    heroDescription:
      "Research agents live or die on sourcing, synthesis, and artifact quality. AgentClash scores whether an agent found the right evidence, cited it correctly, and finished the investigation under budget.",
    proofSectionTitle: "Research eval signals",
    proofSectionDescription:
      "Measure coverage, citation quality, artifact completeness, and whether the agent stopped with a useful deliverable.",
    workflowSectionTitle: "Research eval workflow",
    docsSectionTitle: "Encode investigations as packs",
    docsSectionDescription:
      "Turn recurring research workflows into challenge packs so every model or prompt change reruns the same investigation fairly.",
    faqSectionTitle: "Research agent evaluation FAQ",
    applicationSubCategory: "Research agent evaluation software",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "Use cases", url: "/use-cases" },
      { name: "Research agent evaluation", url: "/use-cases/research-agent-evaluation" },
    ],
    schemaId: "agentclash-research-agent-evaluation-schema",
    searchKeywords:
      "research agent evaluation evaluate research agents investigation agents evidence quality replay scorecards challenge packs",
    sitemapTitle: "Research agent evaluation",
    sitemapDescription:
      "Evaluate research agents on investigation tasks with replay and scorecards.",
    faqItems: [
      {
        question: "What should research agent evaluation score?",
        answer:
          "Task completion, evidence quality, artifact completeness, tool discipline, and whether the final synthesis matches the sources collected.",
      },
      {
        question: "Can evals include web or file tools?",
        answer:
          "Yes. Challenge packs define which tools agents may use and which artifacts must be produced for a pass.",
      },
      {
        question: "How do teams debug a failed research run?",
        answer:
          "Replay shows each search, fetch, note, and synthesis step so reviewers can see where the investigation went off track.",
      },
    ],
  }),
  page({
    path: "/use-cases/support-agent-evaluation",
    tier: "B",
    keyword: "evaluate customer support agents",
    intent: "Use-case page",
    pageTitle: "Customer Support Agent Evaluation - AgentClash",
    metaDescription:
      "Evaluate customer support agents on real resolution workflows with replay evidence, scorecards, challenge packs, and CI regression gates.",
    socialImageAlt: "AgentClash support agent evaluation social preview.",
    eyebrow: "Use case",
    h1: "Evaluate customer support agents on real resolutions",
    heroDescription:
      "Support agents must resolve tickets safely, use the right tools, and leave an audit trail. AgentClash evaluates full support trajectories and keeps replay evidence when tone, policy, or resolution quality regresses.",
    proofSectionTitle: "Support eval signals",
    proofSectionDescription:
      "Score resolution correctness, policy adherence, tool usage, escalation behavior, and customer-facing artifact quality.",
    workflowSectionTitle: "Support eval workflow",
    docsSectionTitle: "Start with escaped tickets",
    docsSectionDescription:
      "Promote real support failures into challenge packs and regression suites so the same mistake cannot return after a model update.",
    faqSectionTitle: "Support agent evaluation FAQ",
    applicationSubCategory: "Customer support agent evaluation software",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "Use cases", url: "/use-cases" },
      { name: "Support agent evaluation", url: "/use-cases/support-agent-evaluation" },
    ],
    schemaId: "agentclash-support-agent-evaluation-schema",
    searchKeywords:
      "customer support agent evaluation support agent eval ticket resolution replay scorecards challenge packs regression testing",
    sitemapTitle: "Support agent evaluation",
    sitemapDescription:
      "Evaluate support agents on resolution workflows with replay and gates.",
    faqItems: [
      {
        question: "Can AgentClash evaluate multi-turn support flows?",
        answer:
          "Yes. Multi-turn challenge packs support scripted, simulated, and human phases for realistic support conversations.",
      },
      {
        question: "How do teams measure policy adherence?",
        answer:
          "Challenge packs encode required actions, forbidden tool use, and validator checks so scorecards reflect policy—not just friendly language.",
      },
      {
        question: "Can support evals gate releases?",
        answer:
          "Yes. Compare candidate and baseline scorecards in CI before deploying a new support agent or model route.",
      },
    ],
  }),
  page({
    path: "/features/agent-scorecards",
    tier: "B",
    keyword: "agent scorecard",
    intent: "Feature keyword",
    pageTitle: "Agent Scorecards for Correctness, Cost, and Evidence - AgentClash",
    metaDescription:
      "Agent scorecards summarize correctness, latency, cost, tool strategy, and evidence quality so teams can compare runs and gate releases.",
    socialImageAlt: "AgentClash agent scorecards social preview.",
    eyebrow: "Feature",
    h1: "Agent scorecards that justify a release decision",
    heroDescription:
      "Scorecards turn long agent runs into a reviewable verdict. AgentClash aggregates correctness, cost, latency, tool strategy, and validator evidence so baselines and candidates are easy to compare.",
    proofSectionTitle: "What scorecards include",
    proofSectionDescription:
      "A scorecard should be more than a single number — it should explain why a run passed or failed.",
    workflowSectionTitle: "From run to scorecard",
    docsSectionTitle: "Interpret results",
    docsSectionDescription:
      "Read the interpret-results guide, then wire scorecard thresholds into CI gates and release policy.",
    faqSectionTitle: "Agent scorecard FAQ",
    applicationSubCategory: "Agent scorecard software",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "Features", url: "/features" },
      { name: "Agent scorecards", url: "/features/agent-scorecards" },
    ],
    schemaId: "agentclash-agent-scorecards-schema",
    searchKeywords:
      "agent scorecard scorecards agent eval results correctness cost latency evidence quality release gates",
    sitemapTitle: "Agent scorecards",
    sitemapDescription:
      "Scorecards for correctness, cost, latency, and evidence quality.",
    faqItems: [
      {
        question: "What appears on an AgentClash scorecard?",
        answer:
          "Correctness signals, validator evidence, cost, latency, tool usage summaries, and dimension scores your challenge pack defines.",
      },
      {
        question: "Can scorecards compare two runs?",
        answer:
          "Yes. Baseline versus candidate comparisons are first-class for regression testing and CI gates.",
      },
      {
        question: "Can scorecards be exported or shared?",
        answer:
          "Yes. Runs keep scorecard views in the product and support shareable evidence for reviewers.",
      },
    ],
    relatedLinks: [
      {
        title: "Interpret results",
        text: "Learn how to read scorecards and replay evidence after a run.",
        href: "/docs/guides/interpret-results",
      },
      ...sharedDocsLinks,
    ],
  }),
  page({
    path: "/features/agent-replay",
    tier: "B",
    keyword: "agent replay",
    intent: "Feature keyword",
    pageTitle: "Agent Replay for Debugging and Review - AgentClash",
    metaDescription:
      "Agent replay preserves tool calls, observations, artifacts, and scorecard evidence so teams can debug agent failures without reproducing them manually.",
    socialImageAlt: "AgentClash agent replay social preview.",
    eyebrow: "Feature",
    h1: "Agent replay that makes failures reviewable",
    heroDescription:
      "When an agent gate fails, reviewers need the path — not a summary. AgentClash replay timelines show tool calls, observations, artifacts, and scorecard context in one place.",
    proofSectionTitle: "Why replay matters",
    proofSectionDescription:
      "Replay turns agent eval from a score into an auditable story of what the agent tried and what the environment returned.",
    workflowSectionTitle: "Replay workflow",
    docsSectionTitle: "Debug with docs",
    docsSectionDescription:
      "Pair replay with interpret-results guidance and challenge packs so every debugged failure becomes a reusable test.",
    faqSectionTitle: "Agent replay FAQ",
    applicationSubCategory: "Agent replay software",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "Features", url: "/features" },
      { name: "Agent replay", url: "/features/agent-replay" },
    ],
    schemaId: "agentclash-agent-replay-schema",
    searchKeywords:
      "agent replay replay evidence tool call timeline agent trace debugging scorecards challenge packs",
    sitemapTitle: "Agent replay",
    sitemapDescription:
      "Replay tool calls, artifacts, and evidence for agent debugging.",
    faqItems: [
      {
        question: "What does AgentClash replay include?",
        answer:
          "Tool calls, observations, logs, artifacts, timing, and links to the scorecard dimensions that passed or failed.",
      },
      {
        question: "Can replay be shared with reviewers?",
        answer:
          "Yes. Runs support shareable views so PMs, support leads, and engineers can review the same evidence.",
      },
      {
        question: "How does replay help CI failures?",
        answer:
          "When a gate fails, replay shows exactly which step diverged from baseline so you do not guess from a single error string.",
      },
    ],
    relatedLinks: [
      {
        title: "Trajectory evaluation",
        text: "See how replay supports full-path agent scoring.",
        href: "/agent-trajectory-evaluation",
      },
      ...sharedDocsLinks,
    ],
  }),
  page({
    path: "/features/challenge-packs",
    tier: "B",
    keyword: "challenge packs agent evaluation",
    intent: "Your unique term",
    pageTitle: "Challenge Packs for Agent Evaluation - AgentClash",
    metaDescription:
      "Challenge packs turn real agent tasks into repeatable evaluations with tools, scoring rules, artifacts, and CI-ready regression gates.",
    socialImageAlt: "AgentClash challenge packs social preview.",
    eyebrow: "Feature",
    h1: "Challenge packs for repeatable agent evaluation",
    heroDescription:
      "Challenge packs are AgentClash's unit of agent evaluation: a real task, tool policy, scoring rules, and artifacts encoded once so every model or harness change reruns the same workload.",
    proofSectionTitle: "What challenge packs encode",
    proofSectionDescription:
      "Inputs, sandbox resources, allowed tools, validators, judges, and pass conditions — everything needed for a fair, repeatable agent eval.",
    workflowSectionTitle: "Pack lifecycle",
    docsSectionTitle: "Author packs with docs",
    docsSectionDescription:
      "Read the challenge pack docs and authoring guide, then promote escaped failures into packs your whole team can run.",
    faqSectionTitle: "Challenge packs FAQ",
    applicationSubCategory: "Challenge pack agent evaluation software",
    breadcrumbs: [
      { name: "Home", url: "/" },
      { name: "Features", url: "/features" },
      { name: "Challenge packs", url: "/features/challenge-packs" },
    ],
    schemaId: "agentclash-challenge-packs-schema",
    searchKeywords:
      "challenge packs agent evaluation repeatable agent tasks scoring rules validators CI regression packs",
    sitemapTitle: "Challenge packs",
    sitemapDescription:
      "Repeatable agent evaluation workloads with scoring and CI gates.",
    faqItems: [
      {
        question: "What is a challenge pack?",
        answer:
          "A challenge pack is a versioned agent evaluation workload with inputs, tool policy, scoring rules, and expected artifacts.",
      },
      {
        question: "Can challenge packs run locally and in CI?",
        answer:
          "Yes. The same pack can power exploratory races, hosted runs, and pull request gates.",
      },
      {
        question: "How do teams create challenge packs?",
        answer:
          "Start from a real failure or release risk, encode it as YAML, and iterate with replay evidence until the scoring rules match what reviewers expect.",
      },
    ],
    relatedLinks: [
      {
        title: "Challenge packs docs",
        text: "Overview of pack structure, scoring, and execution modes.",
        href: "/docs/challenge-packs",
      },
      {
        title: "Write a challenge pack",
        text: "Step-by-step authoring guide for your first pack.",
        href: "/docs/guides/write-a-challenge-pack",
      },
      ...sharedDocsLinks.slice(0, 2),
    ],
  }),
];

const SEO_PAGE_BY_PATH = new Map(
  SEO_PAGE_REGISTRY.map((entry) => [entry.path, entry]),
);

export function getSeoPageByPath(path: string): SeoPageConfig | undefined {
  return SEO_PAGE_BY_PATH.get(path);
}

export function getSeoPagesByPrefix(prefix: string): SeoPageConfig[] {
  return SEO_PAGE_REGISTRY.filter((entry) => entry.path.startsWith(`${prefix}/`));
}

export function getAllSeoPagePaths(): string[] {
  return SEO_PAGE_REGISTRY.map((entry) => entry.path);
}
