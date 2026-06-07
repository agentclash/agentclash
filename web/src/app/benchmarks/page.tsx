import type { Metadata } from "next";
import Link from "next/link";
import { MarketingShell } from "@/components/marketing/marketing-shell";
import { JsonLd, benchmarkIndexSchema } from "@/components/marketing/json-ld";
import { getAllReports, hasPublishedBenchmarks } from "@/lib/benchmarks";
import { benchmarkRssAlternate, ogImageUrl } from "@/lib/seo";

const PAGE_TITLE = "AI Agent Benchmarks - Models Raced Head-to-Head | AgentClash";
const PAGE_DESCRIPTION =
  "Head-to-head AI agent benchmarks: every major model launch raced against the field on real agentic tasks, scored on correctness, reliability, latency, and cost.";
const SOCIAL_IMAGE = ogImageUrl({
  title: "AI Agent Benchmarks",
  subtitle: "New models raced against the field on real agentic tasks",
  kind: "Benchmark",
});

export function generateMetadata(): Metadata {
  const metadata: Metadata = {
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

  // Until a real race ships, the page shows a "coming soon" state — keep it
  // crawlable (so engines actually read the noindex) but out of the index, the
  // same way individual `sample` reports are handled. Indexability returns
  // automatically when the first measured benchmark is published.
  if (!hasPublishedBenchmarks()) {
    metadata.robots = { index: false, follow: true };
  }

  return metadata;
}

export default function BenchmarksPage() {
  const reports = getAllReports();
  // `sample` reports are illustrative only — never list them publicly. With no
  // real report yet, the section shows a "coming soon" state instead.
  const published = reports.filter((report) => !report.sample);

  if (published.length === 0) {
    return (
      <MarketingShell>
        <section className="mx-auto w-full max-w-3xl px-6 py-16">
          <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.14em] text-white/35">
            Benchmarks
          </p>
          <h1 className="mt-3 font-[family-name:var(--font-display)] text-3xl tracking-[-0.02em] leading-[1.15] sm:text-4xl">
            Models, raced head-to-head
          </h1>
          <p className="mt-3 max-w-xl text-sm leading-relaxed text-white/45">
            When a new model ships, we race it against the field on real agentic
            tasks — same challenge, same tools — and score the whole trajectory
            on correctness, reliability, latency, and cost.
          </p>

          <div className="mt-10 rounded-lg border border-white/[0.08] bg-white/[0.03] px-6 py-8">
            <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.14em] text-white/40">
              Coming soon
            </p>
            <p className="mt-3 max-w-xl text-sm leading-relaxed text-white/55">
              We&apos;re lining up the first head-to-head. The opening race drops
              soon — real models on real agentic tasks, with the full scorecard
              and replay. Check back shortly.
            </p>
          </div>
        </section>
      </MarketingShell>
    );
  }

  return (
    <MarketingShell>
      <JsonLd
        id="agentclash-benchmarks-index-schema"
        data={benchmarkIndexSchema(published)}
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
                {report.sample && (
                  <span className="ml-2 rounded-full border border-amber-400/25 bg-amber-400/[0.06] px-1.5 py-0.5 text-[9px] uppercase tracking-[0.12em] text-amber-200/80">
                    Sample
                  </span>
                )}
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
      </section>
    </MarketingShell>
  );
}
