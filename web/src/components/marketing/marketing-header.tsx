import Link from "next/link";
import { ArrowRight, Star } from "lucide-react";
import { withAuth } from "@workos-inc/authkit-nextjs";
import { isReturningVisitor } from "@/lib/auth/returning";
import { DEFAULT_MARKETING_NAV } from "@/lib/marketing-nav";
import { AuthCtaLink } from "./auth-cta-link";
import { ClashMark } from "./clash-mark";

type NavLink = { href: string; label: string; external?: boolean };

const DEFAULT_NAV: NavLink[] = DEFAULT_MARKETING_NAV;

type Props = {
  nav?: NavLink[];
};

export async function MarketingHeader({ nav = DEFAULT_NAV }: Props) {
  const { user } = await withAuth();
  const returning = await isReturningVisitor();

  return (
    <header className="px-5 sm:px-12 py-5 sm:py-6 lg:py-7 border-b border-white/[0.06]">
      <div className="mx-auto flex max-w-[1440px] items-center justify-between">
        <Link
          href="/"
          className="inline-flex items-center gap-2.5 lg:gap-3 text-white/90"
        >
          <ClashMark className="size-6 lg:size-7 2xl:size-8" />
          <span className="font-[family-name:var(--font-display)] text-xl lg:text-2xl 2xl:text-[1.75rem] tracking-[-0.01em]">
            AgentClash
          </span>
        </Link>
        <nav className="flex items-center gap-0.5 sm:gap-2 text-xs lg:text-sm">
          {nav.map((item) =>
            item.external ? (
              <a
                key={item.href}
                href={item.href}
                target="_blank"
                rel="noopener noreferrer"
                className="hidden sm:inline-flex px-3 py-1.5 lg:px-3.5 lg:py-2 text-white/55 hover:text-white/85 transition-colors"
              >
                {item.label}
              </a>
            ) : (
              <Link
                key={item.href}
                href={item.href}
                className="hidden md:inline-flex px-3 py-1.5 lg:px-3.5 lg:py-2 text-white/55 hover:text-white/85 transition-colors"
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
            className="inline-flex items-center gap-1.5 rounded-md border border-white/[0.08] bg-white/[0.03] px-2 sm:px-3 py-1.5 lg:px-3.5 lg:py-2 text-white/60 hover:text-white/85 hover:border-white/15 transition-colors"
          >
            <Star className="size-3.5 lg:size-4" />
            <span className="hidden sm:inline">GitHub</span>
          </a>
          {user ? (
            <Link
              href="/dashboard"
              aria-label="Dashboard"
              className="inline-flex items-center gap-1.5 rounded-md bg-white px-2 sm:px-3 py-1.5 lg:px-3.5 lg:py-2 font-medium text-[#060606] hover:bg-white/90 transition-colors"
            >
              <span className="hidden sm:inline">Dashboard</span>
              <ArrowRight className="size-3 lg:size-3.5" />
            </Link>
          ) : (
            <AuthCtaLink returning={returning} />
          )}
        </nav>
      </div>
    </header>
  );
}
