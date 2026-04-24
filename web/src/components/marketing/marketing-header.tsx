import Link from "next/link";
import { ArrowRight, LogIn, Star } from "lucide-react";
import { withAuth } from "@workos-inc/authkit-nextjs";
import { ClashMark } from "./clash-mark";

type NavLink = { href: string; label: string; external?: boolean };

const DEFAULT_NAV: NavLink[] = [
  { href: "/v2/platform/agent-evaluation", label: "Platform" },
  { href: "/v2/use-cases/coding-agents", label: "Use cases" },
  { href: "/v2/oss", label: "Open source" },
  { href: "/docs", label: "Docs" },
  { href: "/blog", label: "Blog" },
];

type Props = {
  nav?: NavLink[];
};

export async function MarketingHeader({ nav = DEFAULT_NAV }: Props) {
  const { user } = await withAuth();

  return (
    <header className="px-5 sm:px-12 py-5 sm:py-6 border-b border-white/[0.06]">
      <div className="mx-auto flex max-w-[1440px] items-center justify-between">
        <Link
          href="/v2"
          className="inline-flex items-center gap-2.5 text-white/90"
        >
          <ClashMark className="size-6" />
          <span className="font-[family-name:var(--font-display)] text-xl tracking-[-0.01em]">
            AgentClash
          </span>
        </Link>
        <nav className="flex items-center gap-0.5 sm:gap-2 text-xs">
          {nav.map((item) =>
            item.external ? (
              <a
                key={item.href}
                href={item.href}
                target="_blank"
                rel="noopener noreferrer"
                className="hidden sm:inline-flex px-3 py-1.5 text-white/55 hover:text-white/85 transition-colors"
              >
                {item.label}
              </a>
            ) : (
              <Link
                key={item.href}
                href={item.href}
                className="hidden md:inline-flex px-3 py-1.5 text-white/55 hover:text-white/85 transition-colors"
              >
                {item.label}
              </Link>
            ),
          )}
          <a
            href="https://github.com/agentclash/agentclash"
            target="_blank"
            rel="noopener noreferrer"
            aria-label="GitHub"
            className="inline-flex items-center gap-1.5 rounded-md border border-white/[0.08] bg-white/[0.03] px-2 sm:px-3 py-1.5 text-white/60 hover:text-white/85 hover:border-white/15 transition-colors"
          >
            <Star className="size-3.5" />
            <span className="hidden sm:inline">GitHub</span>
          </a>
          {user ? (
            <Link
              href="/dashboard"
              aria-label="Dashboard"
              className="inline-flex items-center gap-1.5 rounded-md bg-white px-2 sm:px-3 py-1.5 font-medium text-[#060606] hover:bg-white/90 transition-colors"
            >
              <span className="hidden sm:inline">Dashboard</span>
              <ArrowRight className="size-3" />
            </Link>
          ) : (
            <Link
              href="/auth/login"
              aria-label="Sign in"
              className="inline-flex items-center gap-1.5 rounded-md border border-white/15 bg-white/[0.04] px-2 sm:px-3 py-1.5 text-white/75 hover:text-white hover:border-white/25 transition-colors"
            >
              <LogIn className="size-3.5" />
              <span className="hidden sm:inline">Sign in</span>
            </Link>
          )}
        </nav>
      </div>
    </header>
  );
}
