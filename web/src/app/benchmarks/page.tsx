import type { Metadata } from "next";
import Link from "next/link";
import { MarketingShell } from "@/components/marketing/marketing-shell";
import { JsonLd, benchmarkIndexSchema } from "@/components/marketing/json-ld";
import { getAllReports } from "@/lib/benchmarks";
import { benchmarkRssAlternate, ogImageUrl } from "@/lib/seo";

const PAGE_TITLE = "AI Agent Benchmarks - Models Raced Head-to-Head | AgentClash";
const PAGE_DESCRIPTION =
  "Head-to-head AI agent benchmarks: every major model launch raced against the field on real agentic tasks, scored on correctness, reliability, latency, and cost.";
const SOCIAL_IMAGE = ogImageUrl({
  title: "AI Agent Benchmarks",
  subtitle: "New models raced against the field on real agentic tasks",
  kind: "Benchmark",
});

export const metadata: Metadata = {
  title: PAGE_TITLE,
  description: PAGE_DESCRIPTION,
  alternates: {
    canonical: "/benchmarks",
    types: benchmarkRssAlternate,
  },
  openGraph: {
    title: PAGE_TITLE,
    description: PAGE_DESCRIPTION,
    url: "/benchmarks",
    type: "website",
    locale: "en_US",
    siteName: "AgentClash",
    images: [{ url: SOCIAL_IMAGE, width: 1200, height: 630, alt: PAGE_TITLE }],
  },
  twitter: {
    card: "summary_large_image",
    title: PAGE_TITLE,
    description: PAGE_DESCRIPTION,
    images: [{ url: SOCIAL_IMAGE, alt: PAGE_TITLE }],
  },
};

export default function BenchmarksPage() {
  const reports = getAllReports();

  return (
    <MarketingShell>
      <JsonLd
        id="agentclash-benchmarks-index-schema"
        data={benchmarkIndexSchema(reports)}
      />
      <section className="mx-auto w-full max-w-3xl px-6 py-16">
        <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.14em] text-white/35">
          Benchmarks
        </p>
        <h1 className="mt-3 font-[family-name:var(--font-display)] text-3xl tracking-[-0.02em] leading-[1.15] sm:text-4xl">
          Models, raced head-to-head
        </h1>
        <p className="mt-3 max-w-xl text-sm leading-relaxed text-white/45">
          When a new model ships, we race it against the field on real agentic
          tasks — same challenge, same tools — and score the whole trajectory on
          correctness, reliability, latency, and cost. Here is who won.
        </p>

        <div className="mt-10 flex flex-col gap-3">
          {reports.map((report) => (
            <Link
              key={report.slug}
              href={`/benchmarks/${report.slug}`}
              className="group flex flex-col gap-1.5 rounded-lg border border-white/[0.08] bg-white/[0.03] px-5 py-4 transition-colors hover:border-white/15"
            >
              <span className="font-[family-name:var(--font-mono)] text-[11px] text-white/40">
                {report.date} &middot; {report.featuredModel}
              </span>
              <span className="text-base font-medium text-white group-hover:text-white/90">
                {report.title}
              </span>
              <span className="text-xs leading-relaxed text-white/40">
                {report.verdict}
              </span>
            </Link>
          ))}
        </div>

        {reports.length === 0 && (
          <p className="mt-10 text-xs text-white/20">No benchmarks yet.</p>
        )}
      </section>
    </MarketingShell>
  );
}
