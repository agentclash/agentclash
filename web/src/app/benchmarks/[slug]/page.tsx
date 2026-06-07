import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { MDXRemote } from "next-mdx-remote/rsc";
import { ArrowUpRight } from "lucide-react";
import { MarketingShell } from "@/components/marketing/marketing-shell";
import { BenchmarkScoreboard } from "@/components/marketing/benchmark-scoreboard";
import {
  JsonLd,
  benchmarkReportSchema,
} from "@/components/marketing/json-ld";
import { getAllSlugs, getReportBySlug } from "@/lib/benchmarks";
import { mdxRemoteOptions } from "@/lib/mdx-options";
import { benchmarkRssAlternate, ogImageUrl } from "@/lib/seo";

type Props = {
  params: Promise<{ slug: string }>;
};

export function generateStaticParams() {
  return getAllSlugs().map((slug) => ({ slug }));
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { slug } = await params;
  const report = getReportBySlug(slug);
  if (!report) return {};
  const title = `${report.title} — AgentClash`;
  const ogImage = ogImageUrl({
    title: report.title,
    subtitle: report.verdict,
    kind: "Benchmark",
  });

  return {
    title,
    description: report.description,
    // Sample reports use illustrative numbers — keep them reachable for humans
    // but out of the search index so engines never treat them as real data.
    robots: report.sample ? { index: false, follow: true } : undefined,
    alternates: {
      canonical: `/benchmarks/${report.slug}`,
      types: benchmarkRssAlternate,
    },
    openGraph: {
      type: "article",
      title,
      description: report.description,
      url: `/benchmarks/${report.slug}`,
      locale: "en_US",
      siteName: "AgentClash",
      publishedTime: report.date,
      authors: [report.author],
      images: [{ url: ogImage, width: 1200, height: 630, alt: report.title }],
    },
    twitter: {
      card: "summary_large_image",
      title,
      description: report.description,
      images: [{ url: ogImage, alt: report.title }],
    },
  };
}

export default async function BenchmarkReportPage({ params }: Props) {
  const { slug } = await params;
  const report = getReportBySlug(slug);
  if (!report) notFound();

  return (
    <MarketingShell>
      {/* Sample reports carry illustrative numbers — don't emit a Dataset/Article
          that would present them to answer engines as real, measured data. */}
      {!report.sample && (
        <JsonLd
          id={`agentclash-benchmark-${report.slug}-schema`}
          data={benchmarkReportSchema(report)}
        />
      )}
      <article className="mx-auto w-full max-w-3xl px-6 py-16">
        <Link
          href="/benchmarks"
          className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.14em] text-white/35 transition-colors hover:text-white/55"
        >
          &larr; Benchmarks
        </Link>

        <header className="mt-6">
          <p className="font-[family-name:var(--font-mono)] text-[11px] text-white/40">
            {report.date} &middot; {report.featuredModel}
            {report.challengePack ? ` · ${report.challengePack}` : ""}
          </p>
          <h1 className="mt-3 font-[family-name:var(--font-display)] text-3xl tracking-[-0.02em] leading-[1.15] sm:text-4xl">
            {report.title}
          </h1>
          <p className="mt-4 text-base leading-relaxed text-white/60">
            {report.verdict}
          </p>
        </header>

        {report.sample && (
          <div className="mt-6 rounded-lg border border-amber-400/25 bg-amber-400/[0.06] px-4 py-3 text-xs leading-relaxed text-amber-200/80">
            <span className="font-medium text-amber-200">Sample data.</span>{" "}
            This report uses representative numbers to illustrate the format. Run
            the race to publish a report with real results.
          </div>
        )}

        <section className="mt-8">
          <h2 className="mb-3 font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.14em] text-white/35">
            Scoreboard
          </h2>
          <BenchmarkScoreboard
            results={report.results}
            featuredModel={report.featuredModel}
          />
          {report.runShareUrl && (
            <a
              href={report.runShareUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="mt-3 inline-flex items-center gap-1 text-xs text-white/45 transition-colors hover:text-white/75"
            >
              View the live race scorecard
              <ArrowUpRight className="size-3" />
            </a>
          )}
        </section>

        {report.tasks.length > 0 && (
          <section className="mt-10">
            <h2 className="mb-3 font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.14em] text-white/35">
              The tasks
            </h2>
            <ol className="flex flex-col gap-2">
              {report.tasks.map((task, index) => (
                <li
                  key={task.id}
                  className="rounded-lg border border-white/[0.06] bg-white/[0.02] px-4 py-3"
                >
                  <div className="flex items-baseline gap-2">
                    <span className="font-[family-name:var(--font-mono)] text-[11px] text-white/30">
                      {String(index + 1).padStart(2, "0")}
                    </span>
                    <span className="text-sm font-medium text-white/85">
                      {task.name}
                    </span>
                  </div>
                  {task.summary && (
                    <p className="mt-1 pl-6 text-xs leading-relaxed text-white/40">
                      {task.summary}
                    </p>
                  )}
                </li>
              ))}
            </ol>
          </section>
        )}

        <div className="prose-agentclash mt-12">
          <MDXRemote source={report.content} options={mdxRemoteOptions} />
        </div>
      </article>
    </MarketingShell>
  );
}
