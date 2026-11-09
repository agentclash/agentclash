export const FACTUAL_PAGES = {
  security: {
    path: "/security",
    title: "Security reporting | AgentClash",
    description: "How to report a vulnerability privately and where to read about sandbox and secrets configuration.",
    sections: [
      { heading: "Report a vulnerability", body: "Use GitHub private vulnerability reporting from the repository Security tab. Do not report vulnerabilities in public issues, discussions, or pull requests. Include the issue and impact, reproduction steps, affected component and version or commit, and relevant configuration.", links: [{ label: "Repository security policy", href: "https://github.com/agentclash/agentclash/security/policy" }] },
      { heading: "Response and supported versions", body: "The project aims to acknowledge reports within a few business days. Please allow a reasonable window for a fix before public disclosure and mention if you would like credit. Fixes target the latest main and the most recent release." },
      { heading: "Explore the security documentation", body: "The security policy identifies sandbox escape, credential exposure, tenant isolation, authorization bypass, and injection as areas of interest. Consult the configuration documentation for the boundaries of your deployment.", links: [{ label: "Sandbox and E2B", href: "/docs/challenge-packs/sandbox-and-e2b" }, { label: "Tools, network and secrets", href: "/docs/concepts/tools-network-and-secrets" }, { label: "Security evaluation", href: "/docs/guides/security-evaluation" }] },
    ],
  },
  methodology: {
    path: "/benchmarks/methodology",
    title: "Benchmark methodology | AgentClash",
    description: "How to interpret public agent benchmark reports and prepare a controlled comparison.",
    sections: [
      { heading: "Design a comparison", body: "Freeze the pack version and commit, task inputs, models, execution environment, tool and network policy, budgets, and scoring configuration. Run candidates under the same constraints and preserve the inputs and evidence needed to inspect the results." },
      { heading: "Scoring and repetitions", body: "Deterministic checks and judge dimensions answer different questions. Repeat each configuration and report sample sizes and variation before estimating reliability. A single run cannot establish a pass rate. Normalized token efficiency is not money spent; dollar cost requires prices and usage." },
      { heading: "What the expression report records", body: "The current report describes one expression evaluator task and one run per model, with shell and filesystem tools, no network, and ten iterations. It reports hidden deterministic tiers, an output contract, and judge dimensions. Its fixture link points to mutable main; no immutable inputs or raw replay artifacts are attached, so the published numbers are not an exact reproduction recipe.", links: [{ label: "Expression evaluator report", href: "/benchmarks/gpt-generations-expression-evaluator" }, { label: "Example fixture", href: "https://github.com/agentclash/agentclash/blob/main/examples/challenge-packs/expr-eval-arena.yaml" }] },
    ],
  },
} as const;
