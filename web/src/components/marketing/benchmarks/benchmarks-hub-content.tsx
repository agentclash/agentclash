import type { ReactNode } from "react";
import Link from "next/link";
import { ArrowRight, ArrowUpRight } from "lucide-react";
import { BenchmarkScoreboard } from "@/components/marketing/benchmark-scoreboard";
import { BenchmarkRunCTA } from "@/components/marketing/research-audience-cta";
import type { BenchmarkReport } from "@/lib/benchmarks";
import {
  BENCHMARKS_CHILD_PAGES,
  BENCHMARKS_METHODOLOGY,
  BENCHMARKS_MONTHLY_BLOG,
  BENCHMARKS_MONTHLY_PROCESS,
  BENCHMARKS_PACK_LINKS,
  BENCHMARKS_READING,
  BENCHMARKS_RUNBOOK_HREF,
} from "@/lib/benchmarks-hub";

type Props = {
  published: BenchmarkReport[];
};

function SectionEyebrow({ children }: { children: ReactNode }) {
  return (
    <p className="font-[family-name:var(--font-mono)] text-2xs uppercase tracking-[0.14em] text-white/35">
      {children}
    </p>
  );
}

function SectionTitle({ children }: { children: ReactNode }) {
  return (
    <h2 className="mt-3 text-xl font-semibold tracking-tight text-white sm:text-2xl">
      {children}
    </h2>
  );
}

function LinkCard({
  href,
  title,
  text,
  external,
}: {
  href: string;
  title: string;
  text: string;
  external?: boolean;
}) {
  const className =
    "group flex flex-col gap-1.5 rounded-lg border border-white/[0.08] bg-white/[0.03] px-5 py-4 transition-colors hover:border-white/15";

  const content = (
    <>
      <span className="inline-flex items-center gap-1 text-sm font-medium text-white group-hover:text-white/90">
        {title}
        {external ? (
          <ArrowUpRight className="size-3.5 text-white/35" />
        ) : (
          <ArrowRight className="size-3.5 text-white/35 transition-transform group-hover:translate-x-0.5" />
        )}
      </span>
      <span className="text-xs leading-relaxed text-white/40">{text}</span>
    </>
  );

  if (external) {
    return (
      <a
        href={href}
        target="_blank"
        rel="noopener noreferrer"
        className={className}
      >
        {content}
      </a>
    );
  }

  return (
    <Link href={href} className={className}>
      {content}
    </Link>
  );
}

export function BenchmarksHubContent({ published }: Props) {
  const latest = published[0];

  return (
    <div className="mx-auto w-full max-w-3xl px-6 py-16">
      <SectionEyebrow>Benchmarks</SectionEyebrow>
      <h1 className="mt-3 text-3xl font-semibold tracking-tight leading-[1.15] text-white sm:text-4xl">
        Head-to-head AI agent benchmarks you can reproduce
      </h1>
      <p className="mt-4 max-w-2xl text-sm leading-relaxed text-white/50 sm:text-base">
        Public eval runs on frozen eval packs: same tools, same constraints,
        full trajectory scoring, and replay evidence. Not a vibes leaderboard.
        Run the same benchmark on your agents when you are ready to gate releases.
      </p>

      <BenchmarkRunCTA className="mt-8" />

      {latest ? (
        <section className="mt-14" aria-labelledby="latest-report-heading">
          <SectionEyebrow>Latest report</SectionEyebrow>
          <SectionTitle>
            <span id="latest-report-heading">{latest.title}</span>
          </SectionTitle>
          <p className="mt-3 text-sm leading-relaxed text-white/50">
            {latest.verdict}
          </p>
          <p className="mt-2 font-[family-name:var(--font-mono)] text-2xs text-white/35">
            {latest.date}
            {latest.evalPack ? ` · ${latest.evalPack}` : ""}
            {latest.featuredModel ? ` · ${latest.featuredModel}` : ""}
          </p>

          {latest.results.length > 0 && (
            <div className="mt-6">
              <BenchmarkScoreboard
                results={latest.results}
                featuredModel={latest.featuredModel}
              />
            </div>
          )}

          <div className="mt-4 flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-4">
            <Link
              href={`/benchmarks/${latest.slug}`}
              className="inline-flex items-center gap-1.5 text-sm text-white/70 transition-colors hover:text-white"
            >
              Read the full report
              <ArrowRight className="size-3.5" />
            </Link>
            <Link
              href={BENCHMARKS_MONTHLY_BLOG.href}
              className="inline-flex items-center gap-1.5 text-sm text-white/55 transition-colors hover:text-white/80"
            >
              {BENCHMARKS_MONTHLY_BLOG.title}
              <ArrowRight className="size-3.5" />
            </Link>
          </div>
        </section>
      ) : (
        <section className="mt-14" aria-labelledby="monthly-process-heading">
          <SectionEyebrow>Monthly reports</SectionEyebrow>
          <SectionTitle>
            <span id="monthly-process-heading">How we publish benchmark reports</span>
          </SectionTitle>
          <p className="mt-3 max-w-2xl text-sm leading-relaxed text-white/50">
            The first measured same-task is in progress. When it ships, this
            hub will summarize winners, scorecards, and replay links. Until then,
            here is the process we follow for every public eval and monthly
            reliability report.
          </p>
          <ol className="mt-6 flex flex-col gap-3">
            {BENCHMARKS_MONTHLY_PROCESS.map((step, index) => (
              <li
                key={step}
                className="flex gap-3 rounded-lg border border-white/[0.06] bg-white/[0.02] px-4 py-3"
              >
                <span className="font-[family-name:var(--font-mono)] text-2xs text-white/30">
                  {String(index + 1).padStart(2, "0")}
                </span>
                <span className="text-sm leading-relaxed text-white/55">
                  {step}
                </span>
              </li>
            ))}
          </ol>
          <p className="mt-4 text-xs text-white/35">
            Subscribe via{" "}
            <Link
              href="/benchmarks/feed.xml"
              className="text-white/55 underline-offset-2 hover:text-white/75 hover:underline"
            >
              benchmarks RSS
            </Link>{" "}
            for new reports.
          </p>
        </section>
      )}

      <section className="mt-14" aria-labelledby="methodology-heading">
        <SectionEyebrow>Methodology</SectionEyebrow>
        <SectionTitle>
          <span id="methodology-heading">Races, not leaderboard snapshots</span>
        </SectionTitle>
        <p className="mt-3 max-w-2xl text-sm leading-relaxed text-white/50">
          Static leaderboards hide setup drift. AgentClash benchmarks are
          same-task eval runs on pinned packs so you can compare models, replay
          evidence, and promote the same workload to a CI gate.
        </p>
        <div className="mt-6 grid gap-3 sm:grid-cols-2">
          {BENCHMARKS_METHODOLOGY.map((item) => (
            <div
              key={item.title}
              className="rounded-lg border border-white/[0.06] bg-white/[0.02] px-4 py-4"
            >
              <h3 className="text-sm font-medium text-white/90">{item.title}</h3>
              <p className="mt-2 text-xs leading-relaxed text-white/45">
                {item.text}
              </p>
            </div>
          ))}
        </div>
      </section>

      {latest && (
        <section className="mt-14" aria-labelledby="monthly-cadence-heading">
          <SectionEyebrow>Monthly cadence</SectionEyebrow>
          <SectionTitle>
            <span id="monthly-cadence-heading">How reports stay reproducible</span>
          </SectionTitle>
          <ol className="mt-6 flex flex-col gap-3">
            {BENCHMARKS_MONTHLY_PROCESS.map((step, index) => (
              <li
                key={step}
                className="flex gap-3 rounded-lg border border-white/[0.06] bg-white/[0.02] px-4 py-3"
              >
                <span className="font-[family-name:var(--font-mono)] text-2xs text-white/30">
                  {String(index + 1).padStart(2, "0")}
                </span>
                <span className="text-sm leading-relaxed text-white/55">
                  {step}
                </span>
              </li>
            ))}
          </ol>
          <p className="mt-4 text-xs text-white/35">
            Owner checklist and scorecard export format:{" "}
            <a
              href={BENCHMARKS_RUNBOOK_HREF}
              target="_blank"
              rel="noopener noreferrer"
              className="text-white/55 underline-offset-2 hover:text-white/75 hover:underline"
            >
              monthly benchmark runbook
            </a>
            .
          </p>
        </section>
      )}

      <section className="mt-14" aria-labelledby="child-benchmarks-heading">
        <SectionEyebrow>Go deeper</SectionEyebrow>
        <SectionTitle>
          <span id="child-benchmarks-heading">Benchmark pages for your team</span>
        </SectionTitle>
        <div className="mt-6 flex flex-col gap-3">
          {BENCHMARKS_CHILD_PAGES.map((page) => (
            <LinkCard key={page.href} {...page} />
          ))}
        </div>
      </section>

      <section className="mt-14" aria-labelledby="packs-heading">
        <SectionEyebrow>Eval packs</SectionEyebrow>
        <SectionTitle>
          <span id="packs-heading">Reproduce the workload</span>
        </SectionTitle>
        <div className="mt-6 flex flex-col gap-3">
          {BENCHMARKS_PACK_LINKS.map((link) => (
            <LinkCard key={link.href} {...link} />
          ))}
        </div>
      </section>

      <section className="mt-14" aria-labelledby="reading-heading">
        <SectionEyebrow>Further reading</SectionEyebrow>
        <SectionTitle>
          <span id="reading-heading">Blog posts on benchmarking agents</span>
        </SectionTitle>
        <ul className="mt-6 flex flex-col gap-2">
          {BENCHMARKS_READING.map((post) => (
            <li key={post.href}>
              <Link
                href={post.href}
                className="inline-flex items-center gap-1.5 text-sm text-white/60 transition-colors hover:text-white/85"
              >
                {post.title}
                <ArrowRight className="size-3.5 text-white/30" />
              </Link>
            </li>
          ))}
        </ul>
      </section>

      {published.length > 0 && (
        <section className="mt-14" aria-labelledby="all-reports-heading">
          <SectionEyebrow>All reports</SectionEyebrow>
          <SectionTitle>
            <span id="all-reports-heading">Every same-task eval we have published</span>
          </SectionTitle>
          <div className="mt-6 flex flex-col gap-3">
            {published.map((report) => (
              <Link
                key={report.slug}
                href={`/benchmarks/${report.slug}`}
                className="group flex flex-col gap-1.5 rounded-lg border border-white/[0.08] bg-white/[0.03] px-5 py-4 transition-colors hover:border-white/15"
              >
                <span className="font-[family-name:var(--font-mono)] text-2xs text-white/40">
                  {report.date} · {report.featuredModel}
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
      )}

      <BenchmarkRunCTA className="mt-14" />
    </div>
  );
}
