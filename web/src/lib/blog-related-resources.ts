export type BlogRelatedResourceLink = {
  href: string;
  label: string;
  description: string;
};

const LINKS = {
  agentEvals: {
    href: "/agent-evals",
    label: "Agent evals",
    description: "Real-task agent evals with replay evidence and CI gates.",
  },
  llmAgentEvaluation: {
    href: "/llm-agent-evaluation",
    label: "LLM agent evaluation",
    description: "Evaluate LLM agents on full trajectories, not one-shot answers.",
  },
  compare: {
    href: "/compare",
    label: "Compare tools",
    description: "See how AgentClash differs from prompt-eval platforms.",
  },
  ciCdAgentEvaluation: {
    href: "/ci-cd-agent-evaluation",
    label: "CI/CD agent evaluation",
    description: "Wire agent evals into pull request gates.",
  },
  releaseGate: {
    href: "/glossary/release-gate",
    label: "Release gate",
    description: "Define pass conditions before a candidate ships.",
  },
  agentReliabilityBenchmark: {
    href: "/agent-reliability-benchmark",
    label: "Agent reliability benchmark",
    description: "Measure pass@k and pass^k on repeatable workloads.",
  },
  codingAgentEvaluation: {
    href: "/use-cases/coding-agent-evaluation",
    label: "Coding agent evaluation",
    description: "Benchmark coding agents on private repos and real tasks.",
  },
  supportAgentEvaluation: {
    href: "/use-cases/support-agent-evaluation",
    label: "Support agent evaluation",
    description: "Test bilingual and escalation-heavy support flows.",
  },
  agentEvaluationFramework: {
    href: "/agent-evaluation-framework",
    label: "Agent evaluation framework",
    description: "Framework for real-task races, replay, and scorecards.",
  },
  agentTrajectoryEvaluation: {
    href: "/agent-trajectory-evaluation",
    label: "Agent trajectory evaluation",
    description: "Score tool choices, retries, and stop conditions.",
  },
  openSourceEvaluation: {
    href: "/open-source-ai-agent-evaluation",
    label: "Open source agent evaluation",
    description: "MIT-licensed, self-hostable agent eval platform.",
  },
  changelog: {
    href: "/changelog",
    label: "Changelog",
    description: "Product updates grouped into ten-day release periods.",
  },
  benchmarks: {
    href: "/benchmarks",
    label: "Benchmarks",
    description: "Measured field reports with frozen eval packs.",
  },
  passAtKPost: {
    href: "/blog/pass-at-k-vs-pass-power-k",
    label: "pass@k vs pass^k",
    description: "When independent retries and strict success-over-trials differ.",
  },
} as const satisfies Record<string, BlogRelatedResourceLink>;

const BLOG_RELATED_RESOURCES: Record<string, BlogRelatedResourceLink[]> = {
  "agent-evaluation-vs-prompt-evaluation-braintrust": [
    LINKS.compare,
    LINKS.llmAgentEvaluation,
    LINKS.agentEvaluationFramework,
  ],
  "agentclash-vs-langsmith-braintrust-production": [
    LINKS.compare,
    LINKS.agentEvals,
    LINKS.ciCdAgentEvaluation,
  ],
  "ai-agent-approval-security-compliance": [
    LINKS.ciCdAgentEvaluation,
    LINKS.releaseGate,
    LINKS.compare,
  ],
  "ai-agent-evaluation-regression-testing": [
    LINKS.agentEvals,
    LINKS.ciCdAgentEvaluation,
    LINKS.compare,
  ],
  "ai-agent-governance-middle-east-enterprises": [
    LINKS.agentEvals,
    LINKS.ciCdAgentEvaluation,
    LINKS.compare,
  ],
  "ai-platform-lead-agent-release-gates": [
    LINKS.ciCdAgentEvaluation,
    LINKS.releaseGate,
    LINKS.agentEvals,
  ],
  "benchmark-ai-agents-on-your-own-data": [
    LINKS.agentReliabilityBenchmark,
    LINKS.benchmarks,
    LINKS.compare,
  ],
  "building-agent-eval-program-regulated-enterprise": [
    LINKS.ciCdAgentEvaluation,
    LINKS.agentEvals,
    LINKS.compare,
  ],
  "coding-agent-benchmark-june-2026": [
    LINKS.benchmarks,
    LINKS.codingAgentEvaluation,
    LINKS.changelog,
  ],
  "do-ai-models-cheat-by-brand": [
    LINKS.benchmarks,
    LINKS.agentTrajectoryEvaluation,
    LINKS.compare,
  ],
  "evaluating-bilingual-customer-support-agents": [
    LINKS.supportAgentEvaluation,
    LINKS.agentEvals,
    LINKS.compare,
  ],
  "evaluating-coding-agents-private-repos-checklist": [
    LINKS.codingAgentEvaluation,
    LINKS.agentEvals,
    LINKS.compare,
  ],
  "how-agentclash-scores-agent-trajectories": [
    LINKS.agentTrajectoryEvaluation,
    LINKS.agentEvals,
    LINKS.compare,
  ],
  "pass-at-k-vs-pass-power-k": [
    LINKS.agentReliabilityBenchmark,
    LINKS.releaseGate,
    LINKS.agentEvals,
  ],
  "pass-k-reliability-enterprise-teams": [
    LINKS.passAtKPost,
    LINKS.releaseGate,
    LINKS.agentEvals,
  ],
  "product-updates-june-2026": [
    LINKS.changelog,
    LINKS.openSourceEvaluation,
    LINKS.compare,
  ],
  "why-agentclash-races-agents-head-to-head": [
    LINKS.agentEvals,
    LINKS.llmAgentEvaluation,
    LINKS.compare,
  ],
  "why-ai-pilot-failed-agent-eval-second-attempt": [
    LINKS.agentEvals,
    LINKS.ciCdAgentEvaluation,
    LINKS.compare,
  ],
  "why-we-built-agentclash": [
    LINKS.openSourceEvaluation,
    LINKS.agentEvals,
    LINKS.compare,
  ],
};

export function getBlogRelatedResources(
  slug: string,
): BlogRelatedResourceLink[] {
  return BLOG_RELATED_RESOURCES[slug] ?? [];
}

export function getMappedBlogRelatedResourceSlugs(): string[] {
  return Object.keys(BLOG_RELATED_RESOURCES).sort();
}
