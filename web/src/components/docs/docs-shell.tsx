"use client";

import type { ReactNode } from "react";
import Link from "next/link";
import { Copy, Sparkles } from "lucide-react";
import type { DocHeading, DocNavSection } from "@/lib/docs";
import { DocsSidebar } from "@/components/docs/docs-sidebar";
import { DocsToc } from "@/components/docs/docs-toc";
import { DocsMobileNav } from "@/components/docs/docs-mobile-nav";
import { DocsSearch } from "@/components/docs/docs-search";

export function DocsShell({
  currentHref,
  title,
  description,
  sectionTitle,
  sections,
  headings,
  children,
}: {
  currentHref: string;
  title: string;
  description: string;
  sectionTitle?: string;
  sections: DocNavSection[];
  headings: DocHeading[];
  children: ReactNode;
}) {
  return (
    <main className="min-h-screen bg-[#060606] text-zinc-300">
      <header className="sticky top-0 z-40 border-b border-white/[0.08] bg-[#060606]/90 backdrop-blur">
        <div className="flex h-16 w-full items-center justify-between gap-4 px-6">
          <div className="flex items-center gap-6">
            <Link href="/" className="flex items-center gap-2 text-zinc-100">
              <Sparkles className="size-5" />
              <span className="font-semibold tracking-tight">AgentClash</span>
            </Link>
            <nav className="hidden items-center gap-6 text-sm font-medium text-zinc-400 md:flex">
              <Link href="/docs" className="relative text-zinc-100">
                Docs
                <span className="absolute -bottom-[22px] left-0 right-0 h-0.5 bg-white/70" />
              </Link>
              <Link
                href="/changelog"
                className="transition-colors hover:text-zinc-100"
              >
                Changelog
              </Link>
              <Link
                href="/blog"
                className="transition-colors hover:text-zinc-100"
              >
                Blog
              </Link>
            </nav>
          </div>

          <div className="flex flex-1 items-center justify-end gap-3 md:flex-none">
            <DocsMobileNav sections={sections} currentHref={currentHref} />
            <DocsSearch />
            <a
              href="https://cal.com/agentclash/demo"
              target="_blank"
              rel="noopener noreferrer"
              className="hidden shrink-0 whitespace-nowrap rounded-full border border-white/15 bg-white/[0.06] px-4 py-1.5 text-sm font-medium text-white/90 transition-colors hover:border-white/25 hover:bg-white/[0.09] sm:block"
            >
              Get Started &rarr;
            </a>
          </div>
        </div>
      </header>

      <div className="mx-auto flex w-full max-w-[1536px] items-start px-6 lg:px-8">
        <aside className="sticky top-16 hidden h-[calc(100vh-4rem)] w-64 shrink-0 overflow-y-auto border-r border-white/[0.08] pt-8 lg:block">
          <DocsSidebar sections={sections} currentHref={currentHref} />
        </aside>

        <section className="min-w-0 flex-1 pb-20 pt-8 lg:px-12 lg:pt-12">
          <div className="mx-auto max-w-[720px]">
            <header className="mb-10 border-b border-white/[0.08] pb-8">
              {sectionTitle ? (
                <p className="text-2xs font-semibold uppercase tracking-[0.18em] text-white/35">
                  {sectionTitle}
                </p>
              ) : (
                <p className="text-2xs font-semibold uppercase tracking-[0.18em] text-white/35">
                  Documentation
                </p>
              )}
              <div className="mt-3 flex items-start justify-between gap-4">
                <h1 className="font-sans text-3xl font-semibold tracking-tight text-white/92 antialiased sm:text-[2rem] sm:leading-tight">
                  {title}
                </h1>
                <button
                  type="button"
                  onClick={() =>
                    navigator.clipboard.writeText(window.location.href)
                  }
                  className="hidden items-center gap-2 rounded-xl border border-white/[0.08] px-3 py-1.5 text-2xs font-medium uppercase tracking-wider text-white/45 transition-colors hover:border-white/15 hover:text-white/70 sm:flex"
                >
                  <Copy className="size-3.5" />
                  Copy page
                </button>
              </div>
              <p className="mt-4 text-base leading-7 text-white/45">
                {description}
              </p>
            </header>

            {children}
          </div>
        </section>

        <div className="hidden w-56 shrink-0 xl:block">
          <DocsToc headings={headings} />
        </div>
      </div>
    </main>
  );
}
