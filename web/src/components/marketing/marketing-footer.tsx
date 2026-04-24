import Link from "next/link";

const COLUMNS: Array<{
  heading: string;
  links: Array<{ href: string; label: string; external?: boolean }>;
}> = [
  {
    heading: "Product",
    links: [
      { href: "/v2/platform/agent-evaluation", label: "Agent evaluation" },
      { href: "/v2/platform/regression-testing", label: "Regression testing" },
      { href: "/v2/platform/ci-cd-gating", label: "CI/CD gating" },
      { href: "/v2/platform/multi-turn-evaluation", label: "Multi-turn evals" },
      { href: "/v2/platform/rag-evaluation", label: "RAG evaluation" },
      { href: "/v2/platform/self-hosted", label: "Self-hosted" },
    ],
  },
  {
    heading: "Use cases",
    links: [
      { href: "/v2/use-cases/coding-agents", label: "Coding agents" },
      { href: "/v2/use-cases/deep-research", label: "Deep research" },
      { href: "/v2/use-cases/sql-data-agents", label: "SQL & data" },
      { href: "/v2/use-cases/support-agents", label: "Support agents" },
      { href: "/v2/use-cases/sre-agents", label: "SRE agents" },
    ],
  },
  {
    heading: "Compare",
    links: [
      { href: "/v2/vs/langsmith", label: "vs LangSmith" },
      { href: "/v2/vs/braintrust", label: "vs Braintrust" },
      { href: "/v2/vs/promptfoo", label: "vs Promptfoo" },
      { href: "/v2/vs/langfuse", label: "vs Langfuse" },
      { href: "/v2/vs/openai-evals", label: "vs OpenAI Evals" },
    ],
  },
  {
    heading: "Company",
    links: [
      { href: "/v2/cloud", label: "Managed cloud" },
      { href: "/v2/oss", label: "Open source" },
      { href: "/v2/security", label: "Security" },
      { href: "/v2/methodology", label: "Methodology" },
      { href: "/v2/design-partners", label: "Design partners" },
      { href: "/blog", label: "Blog" },
      { href: "/docs", label: "Docs" },
      { href: "/team", label: "Team" },
      { href: "https://github.com/agentclash/agentclash", label: "GitHub", external: true },
    ],
  },
];

export function MarketingFooter() {
  return (
    <footer className="mt-auto border-t border-white/[0.06] px-8 sm:px-12 pt-20 pb-10">
      <div className="mx-auto max-w-[1440px]">
        <div className="grid gap-12 sm:grid-cols-2 lg:grid-cols-4">
          {COLUMNS.map((col) => (
            <div key={col.heading}>
              <p className="text-[11px] font-[family-name:var(--font-mono)] uppercase tracking-[0.2em] text-white/35">
                {col.heading}
              </p>
              <ul className="mt-5 space-y-3 text-[13px] text-white/55">
                {col.links.map((link) =>
                  link.external ? (
                    <li key={link.href}>
                      <a
                        href={link.href}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="hover:text-white/90 transition-colors"
                      >
                        {link.label}
                      </a>
                    </li>
                  ) : (
                    <li key={link.href}>
                      <Link
                        href={link.href}
                        className="hover:text-white/90 transition-colors"
                      >
                        {link.label}
                      </Link>
                    </li>
                  ),
                )}
              </ul>
            </div>
          ))}
        </div>

        <div className="mt-16 flex flex-wrap items-center justify-between gap-4 border-t border-white/[0.06] pt-8 text-[11px] font-[family-name:var(--font-mono)] text-white/35">
          <div className="flex items-center gap-6">
            <span className="font-medium text-white/55">AgentClash</span>
            <span className="text-white/40">Beta · FSL-1.1-MIT</span>
          </div>
          <div className="flex items-center gap-5">
            <Link href="/docs" className="hover:text-white/70 transition-colors">
              Docs
            </Link>
            <a
              href="/llms.txt"
              className="hover:text-white/70 transition-colors"
            >
              llms.txt
            </a>
            <a
              href="/llms-full.txt"
              className="hover:text-white/70 transition-colors"
            >
              llms-full.txt
            </a>
            <a
              href="https://github.com/agentclash/agentclash"
              target="_blank"
              rel="noopener noreferrer"
              className="hover:text-white/70 transition-colors"
            >
              GitHub
            </a>
          </div>
        </div>
      </div>
    </footer>
  );
}
