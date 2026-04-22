import type { ReactNode } from "react";
import Link from "next/link";
import { ArrowUpRight, BookOpenText } from "lucide-react";
import type { DocHeading, DocNavSection, DocSearchItem } from "@/lib/docs";
import { DocsSidebar } from "@/components/docs/docs-sidebar";
import { DocsToc } from "@/components/docs/docs-toc";

export function DocsShell({
  currentHref,
  title,
  description,
  sectionTitle,
  sections,
  searchItems,
  headings,
  children,
}: {
  currentHref: string;
  title: string;
  description: string;
  sectionTitle?: string;
  sections: DocNavSection[];
  searchItems: DocSearchItem[];
  headings: DocHeading[];
  children: ReactNode;
}) {
  return (
    <main className="min-h-screen bg-[radial-gradient(circle_at_top_left,rgba(212,255,79,0.09),transparent_24%),radial-gradient(circle_at_top_right,rgba(255,255,255,0.06),transparent_18%),#050505] text-white">
      <header className="border-b border-white/[0.08] bg-black/20 backdrop-blur">
        <div className="mx-auto flex w-full max-w-[1440px] items-center justify-between gap-4 px-6 py-4 sm:px-8">
          <div className="flex items-center gap-3">
            <div className="flex size-10 items-center justify-center rounded-2xl border border-white/[0.08] bg-white/[0.03]">
              <BookOpenText className="size-4 text-lime-200" />
            </div>
            <div>
              <Link
                href="/docs"
                className="font-[family-name:var(--font-display)] text-xl tracking-[-0.02em] text-white/95"
              >
                AgentClash Docs
              </Link>
              <p className="text-xs text-white/35">
                Product, reference, and contributor docs in one place.
              </p>
            </div>
          </div>

          <div className="flex items-center gap-2 text-xs">
            <Link
              href="/"
              className="rounded-full border border-white/[0.08] bg-white/[0.03] px-3 py-2 text-white/65 transition-colors hover:border-white/15 hover:text-white/90"
            >
              Product
            </Link>
            <a
              href="https://github.com/agentclash/agentclash"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-1.5 rounded-full border border-white/[0.08] bg-white/[0.03] px-3 py-2 text-white/65 transition-colors hover:border-white/15 hover:text-white/90"
            >
              GitHub
              <ArrowUpRight className="size-3" />
            </a>
          </div>
        </div>
      </header>

      <div className="mx-auto grid w-full max-w-[1440px] gap-12 px-6 py-10 sm:px-8 lg:grid-cols-[280px_minmax(0,1fr)] lg:gap-16 lg:py-14 xl:grid-cols-[280px_minmax(0,1fr)_220px] xl:items-start">
        <aside className="lg:sticky lg:top-8 lg:h-fit">
          <DocsSidebar
            sections={sections}
            currentHref={currentHref}
            searchItems={searchItems}
          />
        </aside>

        <section className="min-w-0">
          <div className="rounded-[32px] border border-white/[0.08] bg-white/[0.03] px-6 py-8 shadow-[0_24px_80px_rgba(0,0,0,0.32)] sm:px-10 sm:py-10">
            <div className="max-w-3xl">
              <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.18em] text-lime-200/70">
                {sectionTitle ?? "Documentation"}
              </p>
              <h1 className="mt-4 font-[family-name:var(--font-display)] text-4xl tracking-[-0.04em] text-white sm:text-5xl">
                {title}
              </h1>
              <p className="mt-4 max-w-2xl text-base leading-7 text-white/55 sm:text-lg">
                {description}
              </p>
            </div>

            <div className="mt-10 border-t border-white/[0.08] pt-8">
              {children}
            </div>
          </div>
        </section>

        <DocsToc headings={headings} />
      </div>
    </main>
  );
}
