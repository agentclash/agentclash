"use client";

import type { ReactNode } from "react";
import Link from "next/link";
import { Copy, Sparkles } from "lucide-react";
import type { DocHeading, DocNavSection } from "@/lib/docs";
import { DocsSidebar } from "@/components/docs/docs-sidebar";
import { DocsToc } from "@/components/docs/docs-toc";
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
      <header className="sticky top-0 z-40 border-b border-zinc-800/60 bg-[#060606]/90 backdrop-blur">
        <div className="flex h-16 w-full items-center justify-between gap-4 px-6">
          <div className="flex items-center gap-6">
            <Link href="/" className="flex items-center gap-2 text-zinc-100">
              <Sparkles className="size-5" />
              <span className="font-semibold tracking-tight">AgentClash</span>
            </Link>
            <nav className="hidden items-center gap-6 text-sm font-medium text-zinc-400 md:flex">
              <Link href="/docs" className="text-zinc-100 relative">
                Docs
                <span className="absolute -bottom-[22px] left-0 right-0 h-0.5 bg-emerald-500" />
              </Link>
              <span className="cursor-not-allowed text-zinc-600">SDK & Examples</span>
              <span className="cursor-not-allowed text-zinc-600">API Reference</span>
            </nav>
          </div>

          <div className="flex flex-1 items-center justify-end gap-4 md:flex-none">
            <DocsSearch />
            <a
              href="https://cal.com/agentclash/demo"
              target="_blank"
              rel="noopener noreferrer"
              className="hidden rounded-full bg-emerald-600 px-4 py-1.5 text-sm font-medium text-white transition-colors hover:bg-emerald-700 sm:block"
            >
              Get Started &rarr;
            </a>
          </div>
        </div>
      </header>

      <div className="mx-auto flex w-full max-w-[1536px] items-start px-6 lg:px-8">
        <aside className="sticky top-16 hidden h-[calc(100vh-4rem)] w-64 shrink-0 overflow-y-auto border-r border-zinc-800/60 pt-8 lg:block">
          <DocsSidebar sections={sections} currentHref={currentHref} />
        </aside>

        <section className="flex-1 min-w-0 pb-20 pt-8 lg:px-12 lg:pt-12">
          <div className="mx-auto max-w-[800px]">
            <div className="mb-8">
              <p className="mb-2 text-sm font-medium text-emerald-500">
                {sectionTitle ?? "Documentation"}
              </p>
              <div className="flex items-start justify-between gap-4">
                <h1 className="text-3xl font-bold tracking-tight text-zinc-100 sm:text-4xl">
                  {title}
                </h1>
                <button 
                  onClick={() => navigator.clipboard.writeText(window.location.href)}
                  className="hidden items-center gap-2 rounded-md border border-zinc-800 px-3 py-1.5 text-xs font-medium text-zinc-400 transition-colors hover:bg-zinc-800 hover:text-zinc-100 sm:flex"
                >
                  <Copy className="size-3.5" />
                  Copy page
                </button>
              </div>
              <p className="mt-4 text-lg text-zinc-400">
                {description}
              </p>
            </div>

            <div className="prose prose-invert prose-zinc max-w-none prose-headings:text-zinc-100 prose-a:text-emerald-500 prose-a:no-underline hover:prose-a:underline prose-code:text-zinc-200 prose-pre:bg-zinc-900 prose-pre:border prose-pre:border-zinc-800">
              {children}
            </div>
          </div>
        </section>

        <div className="hidden w-64 shrink-0 xl:block">
          <DocsToc headings={headings} />
        </div>
      </div>
    </main>
  );
}
