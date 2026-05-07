import Link from "next/link";

const COLUMNS: Array<{
  heading: string;
  links: Array<{ href: string; label: string; external?: boolean }>;
}> = [
  {
    heading: "Product",
    links: [
      { href: "/#features", label: "Features" },
      { href: "/docs", label: "Docs" },
      { href: "/docs/getting-started/quickstart", label: "Quickstart" },
      { href: "/docs/getting-started/self-host", label: "Self-host" },
      { href: "/docs/challenge-packs", label: "Challenge packs" },
      { href: "/docs/guides/ci-cd-agent-gates", label: "CI/CD gates" },
    ],
  },
  {
    heading: "Guides",
    links: [
      { href: "/docs/guides/write-a-challenge-pack", label: "Write a challenge pack" },
      { href: "/docs/guides/configure-runtime-resources", label: "Configure resources" },
      { href: "/docs/guides/interpret-results", label: "Interpret results" },
      { href: "/docs/guides/ci-cd-workload-recipes", label: "CI/CD recipes" },
      { href: "/docs/guides/use-with-ai-tools", label: "Use with AI tools" },
    ],
  },
  {
    heading: "Company",
    links: [
      { href: "/blog", label: "Blog" },
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
