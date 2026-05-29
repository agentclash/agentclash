import type { Metadata } from "next";
import Link from "next/link";
import { Star } from "lucide-react";
import { GridBackdrop, Wordmark } from "@/components/try-cli/chrome";

export const metadata: Metadata = {
  title: "Try CLI — Interactive terminal demos | AgentClash",
  description:
    "Run any CLI in a disposable cloud terminal before you install it. AI coding agents and dev tools, zero setup, powered by E2B sandboxes.",
  openGraph: {
    title: "AgentClash Try CLI",
    description: "Run any CLI in a disposable cloud terminal — zero install.",
    url: "https://www.agentclash.dev/try",
  },
};

export default function TryCliLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="relative min-h-screen bg-[#060606] text-white antialiased">
      <GridBackdrop />
      <header className="sticky top-0 z-40 h-16 border-b border-white/[0.06] bg-[#060606]/70 backdrop-blur-md">
        <div className="mx-auto flex h-full max-w-[1400px] items-center justify-between px-5 sm:px-8">
          <div className="flex items-center gap-3">
            <Wordmark />
            <span className="hidden select-none text-white/20 sm:inline">/</span>
            <Link
              href="/try"
              className="hidden text-sm text-white/55 transition-colors hover:text-white/90 sm:inline"
            >
              Try
            </Link>
          </div>
          <nav className="flex items-center gap-1.5 text-xs">
            <Link
              href="/docs/concepts/try-cli"
              className="px-3 py-1.5 text-white/55 transition-colors hover:text-white/85"
            >
              Docs
            </Link>
            <a
              href="https://github.com/agentclash/agentclash"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-1.5 rounded-md border border-white/[0.08] bg-white/[0.03] px-3 py-1.5 text-white/60 transition-colors hover:border-white/15 hover:text-white/85"
            >
              <Star className="size-3.5" />
              <span className="hidden sm:inline">GitHub</span>
            </a>
          </nav>
        </div>
      </header>
      {children}
    </div>
  );
}
