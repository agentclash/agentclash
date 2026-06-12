import type { ReactNode } from "react";
import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { ClashMark } from "@/components/marketing/clash-mark";

// Shared header + footer chrome for the /compare hub and per-competitor pages,
// mirroring the platform marketing pages. Server component (no client hooks).
export function CompareShell({ children }: { children: ReactNode }) {
  return (
    <>
      <header className="border-b border-white/[0.06] px-5 py-5 sm:px-12 sm:py-6 lg:py-7">
        <div className="mx-auto flex max-w-[1440px] items-center justify-between gap-4">
          <Link
            href="/"
            className="inline-flex items-center gap-2.5 lg:gap-3 text-white/90"
          >
            <ClashMark className="size-6 lg:size-7 2xl:size-8" />
            <span className="font-[family-name:var(--font-display)] text-xl lg:text-2xl 2xl:text-[1.75rem] tracking-normal">
              AgentClash
            </span>
          </Link>
          <nav className="flex items-center gap-2 text-xs lg:text-sm">
            <Link
              href="/compare"
              className="hidden px-3 py-1.5 lg:px-3.5 lg:py-2 text-white/55 transition-colors hover:text-white/85 sm:inline-flex"
            >
              Compare
            </Link>
            <Link
              href="/docs"
              className="hidden px-3 py-1.5 lg:px-3.5 lg:py-2 text-white/55 transition-colors hover:text-white/85 sm:inline-flex"
            >
              Docs
            </Link>
            <a
              href="https://github.com/agentclash/agentclash"
              target="_blank"
              rel="noopener noreferrer"
              className="hidden px-3 py-1.5 lg:px-3.5 lg:py-2 text-white/55 transition-colors hover:text-white/85 sm:inline-flex"
            >
              GitHub
            </a>
            <Link
              href="/auth/login"
              className="inline-flex items-center gap-1.5 rounded-md bg-white px-3 py-1.5 lg:px-3.5 lg:py-2 font-medium text-[#060606] transition-colors hover:bg-white/90"
            >
              Start
              <ArrowRight className="size-3.5 lg:size-4" />
            </Link>
          </nav>
        </div>
      </header>
      <main className="min-h-screen bg-[#060606] text-white">{children}</main>
      <footer className="border-t border-white/[0.06] bg-[#060606] px-6 py-10 sm:px-12">
        <div className="mx-auto flex max-w-[1440px] flex-col gap-5 text-sm text-white/45 sm:flex-row sm:items-center sm:justify-between">
          <Link href="/" className="font-medium text-white/65">
            AgentClash
          </Link>
          <nav className="flex flex-wrap gap-5">
            <Link href="/pricing" className="transition-colors hover:text-white/75">
              Pricing
            </Link>
            <Link href="/compare" className="transition-colors hover:text-white/75">
              Compare
            </Link>
            <Link href="/docs" className="transition-colors hover:text-white/75">
              Docs
            </Link>
            <Link href="/blog" className="transition-colors hover:text-white/75">
              Blog
            </Link>
            <a
              href="https://github.com/agentclash/agentclash"
              target="_blank"
              rel="noopener noreferrer"
              className="transition-colors hover:text-white/75"
            >
              GitHub
            </a>
          </nav>
        </div>
      </footer>
    </>
  );
}
