import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { MDXRemote } from "next-mdx-remote/rsc";
import { DocsShell } from "@/components/docs/docs-shell";
import { docsMDXComponents } from "@/components/docs/mdx-components";
import {
  DOCS_NAV,
  getAllDocSlugs,
  getDocBySlug,
  getDocNeighbors,
  getDocsSearchIndex,
} from "@/lib/docs";

type Props = {
  params: Promise<{ slug?: string[] }>;
};

export function generateStaticParams() {
  return getAllDocSlugs().map((slug) => ({ slug }));
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { slug = [] } = await params;
  const doc = getDocBySlug(slug);
  if (!doc) return {};

  return {
    title: `${doc.title} — AgentClash Docs`,
    description: doc.description,
  };
}

export default async function DocsPage({ params }: Props) {
  const { slug = [] } = await params;
  const doc = getDocBySlug(slug);
  if (!doc) notFound();

  const isHome = doc.href === "/docs";
  const searchItems = getDocsSearchIndex();
  const neighbors = getDocNeighbors(doc.href);

  return (
    <DocsShell
      currentHref={doc.href}
      title={doc.title}
      description={doc.description}
      sectionTitle={doc.sectionTitle}
      sections={DOCS_NAV}
      searchItems={searchItems}
      headings={doc.headings}
    >
      <div className="prose-agentclash-docs">
        <MDXRemote source={doc.content} components={docsMDXComponents} />
      </div>

      {isHome && (
        <div className="mt-12 border-t border-white/[0.08] pt-8">
          <div className="grid gap-4 xl:grid-cols-2">
            {DOCS_NAV.map((section) => (
              <section
                key={section.title}
                className="rounded-[24px] border border-white/[0.08] bg-black/20 p-5"
              >
                <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.18em] text-white/30">
                  {section.title}
                </p>
                <p className="mt-3 text-sm leading-6 text-white/45">
                  {section.description}
                </p>
                <div className="mt-5 space-y-3">
                  {section.items.map((item) => (
                    <Link
                      key={item.href}
                      href={item.href}
                      className="block rounded-2xl border border-white/[0.08] bg-white/[0.03] px-4 py-3 transition-colors hover:border-white/15 hover:bg-white/[0.04]"
                    >
                      <span className="block text-sm font-medium text-white/90">
                        {item.title}
                      </span>
                      <span className="mt-1 block text-xs leading-5 text-white/45">
                        {item.description}
                      </span>
                    </Link>
                  ))}
                </div>
              </section>
            ))}
          </div>
        </div>
      )}

      {!isHome && (neighbors.previous || neighbors.next) && (
        <div className="mt-12 grid gap-4 border-t border-white/[0.08] pt-8 sm:grid-cols-2">
          {neighbors.previous ? (
            <Link
              href={neighbors.previous.href}
              className="rounded-[24px] border border-white/[0.08] bg-black/20 px-5 py-4 transition-colors hover:border-white/15"
            >
              <span className="block text-[11px] uppercase tracking-[0.18em] text-white/30">
                Previous
              </span>
              <span className="mt-2 block text-sm font-medium text-white/88">
                {neighbors.previous.title}
              </span>
            </Link>
          ) : (
            <div />
          )}
          {neighbors.next && (
            <Link
              href={neighbors.next.href}
              className="rounded-[24px] border border-white/[0.08] bg-black/20 px-5 py-4 text-left transition-colors hover:border-white/15"
            >
              <span className="block text-[11px] uppercase tracking-[0.18em] text-white/30">
                Next
              </span>
              <span className="mt-2 block text-sm font-medium text-white/88">
                {neighbors.next.title}
              </span>
            </Link>
          )}
        </div>
      )}
    </DocsShell>
  );
}
