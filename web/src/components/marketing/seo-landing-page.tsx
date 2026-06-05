import Link from "next/link";
import {
  ArrowRight,
  CheckCircle2,
  Code2,
  GitBranch,
  Play,
  ShieldCheck,
  Sparkles,
  Terminal,
} from "lucide-react";
import { ClashMark } from "@/components/marketing/clash-mark";
import {
  JsonLd,
  breadcrumbSchema,
  faqSchema,
  productSchema,
} from "@/components/marketing/json-ld";
import type { SeoPageConfig } from "@/lib/seo-pages/types";

const workflowIcons = [Code2, Play, Sparkles, GitBranch] as const;

function StaticHeader() {
  return (
    <header className="border-b border-white/[0.06] px-5 py-5 sm:px-12 sm:py-6">
      <div className="mx-auto flex max-w-[1440px] items-center justify-between gap-4">
        <Link
          href="/"
          className="inline-flex items-center gap-2.5 text-white/90"
        >
          <ClashMark className="size-6" />
          <span className="font-[family-name:var(--font-display)] text-xl tracking-normal">
            AgentClash
          </span>
        </Link>
        <nav className="flex items-center gap-2 text-xs">
          <Link
            href="/docs"
            className="hidden px-3 py-1.5 text-white/55 transition-colors hover:text-white/85 sm:inline-flex"
          >
            Docs
          </Link>
          <a
            href="https://github.com/agentclash/agentclash"
            target="_blank"
            rel="noopener noreferrer"
            className="hidden px-3 py-1.5 text-white/55 transition-colors hover:text-white/85 sm:inline-flex"
          >
            GitHub
          </a>
          <Link
            href="/auth/login"
            className="inline-flex items-center gap-1.5 rounded-md bg-white px-3 py-1.5 font-medium text-[#060606] transition-colors hover:bg-white/90"
          >
            Start
            <ArrowRight className="size-3.5" />
          </Link>
        </nav>
      </div>
    </header>
  );
}

function StaticFooter() {
  return (
    <footer className="border-t border-white/[0.06] bg-[#060606] px-6 py-10 sm:px-12">
      <div className="mx-auto flex max-w-[1440px] flex-col gap-5 text-sm text-white/45 sm:flex-row sm:items-center sm:justify-between">
        <Link href="/" className="font-medium text-white/65">
          AgentClash
        </Link>
        <nav className="flex flex-wrap gap-5">
          <Link href="/docs" className="transition-colors hover:text-white/75">
            Docs
          </Link>
          <Link href="/blog" className="transition-colors hover:text-white/75">
            Blog
          </Link>
          <Link href="/team" className="transition-colors hover:text-white/75">
            Team
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
  );
}

function ProductVisual() {
  return (
    <div className="relative overflow-hidden rounded-md border border-white/[0.08] bg-[#090909] shadow-2xl shadow-cyan-950/20">
      <div className="border-b border-white/[0.08] px-4 py-3">
        <div className="flex items-center justify-between gap-4">
          <div className="flex items-center gap-2">
            <span className="size-2 rounded-full bg-emerald-300" />
            <span className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-normal text-white/45">
              live race
            </span>
          </div>
          <span className="rounded-md border border-white/[0.08] px-2 py-1 font-[family-name:var(--font-mono)] text-[11px] text-white/45">
            gate: pass
          </span>
        </div>
      </div>
      <div className="grid gap-px bg-white/[0.06] sm:grid-cols-3">
        {[
          ["Candidate", "92", "correct patch, low cost"],
          ["Baseline", "88", "stable reference run"],
          ["Control", "73", "missed edge case"],
        ].map(([name, score, note]) => (
          <div key={name} className="bg-[#090909] p-4">
            <p className="text-sm font-medium text-white">{name}</p>
            <div className="mt-5 flex items-end justify-between gap-3">
              <span className="font-[family-name:var(--font-display)] text-5xl text-white">
                {score}
              </span>
              <span className="mb-1 text-right text-xs leading-5 text-white/45">
                {note}
              </span>
            </div>
          </div>
        ))}
      </div>
      <div className="grid gap-px bg-white/[0.06] md:grid-cols-[1.1fr_0.9fr]">
        <div className="bg-[#090909] p-5">
          <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-normal text-white/35">
            replay timeline
          </p>
          <div className="mt-4 space-y-3">
            {[
              "loaded task inputs and tool policy",
              "ran sandbox actions and captured artifacts",
              "scored trajectory and validator evidence",
              "attached scorecard and release verdict",
            ].map((item, index) => (
              <div key={item} className="flex items-start gap-3">
                <span className="mt-1 flex size-5 items-center justify-center rounded-md border border-cyan-300/25 bg-cyan-300/10 font-[family-name:var(--font-mono)] text-[10px] text-cyan-100">
                  {index + 1}
                </span>
                <span className="text-sm leading-6 text-white/68">{item}</span>
              </div>
            ))}
          </div>
        </div>
        <div className="bg-[#090909] p-5">
          <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-normal text-white/35">
            ci verdict
          </p>
          <div className="mt-4 rounded-md border border-emerald-300/15 bg-emerald-300/[0.06] p-4">
            <div className="flex items-center gap-2 text-sm font-medium text-emerald-100">
              <ShieldCheck className="size-4" />
              Candidate clears release gate
            </div>
            <p className="mt-3 text-xs leading-5 text-emerald-50/60">
              Correctness improved, latency within budget, and required
              artifacts were preserved for review.
            </p>
          </div>
          <pre className="mt-4 overflow-hidden rounded-md border border-white/[0.08] bg-black px-4 py-3 font-[family-name:var(--font-mono)] text-[11px] leading-5 text-white/55">
            agentclash run create --follow
          </pre>
        </div>
      </div>
    </div>
  );
}

export function SeoLandingPage({ config }: { config: SeoPageConfig }) {
  return (
    <>
      <JsonLd
        id={config.schemaId}
        data={[
          breadcrumbSchema(config.breadcrumbs),
          faqSchema(config.faqItems),
          productSchema({
            name: config.pageTitle,
            description: config.metaDescription,
            url: config.path,
            applicationSubCategory: config.applicationSubCategory,
            featureList: config.proofPoints,
          }),
        ]}
      />
      <StaticHeader />
      <main className="min-h-screen bg-[#060606] text-white">
        <section className="px-6 py-20 sm:px-12 sm:py-28">
          <div className="mx-auto grid max-w-[1440px] gap-14 lg:grid-cols-[0.88fr_1.12fr] lg:items-center">
            <div>
              <nav className="flex flex-wrap items-center gap-2 text-xs text-white/35">
                {config.breadcrumbs.map((crumb, index) => (
                  <span
                    key={`${crumb.url}-${index}`}
                    className="inline-flex items-center gap-2"
                  >
                    {index > 0 && <span>/</span>}
                    {index < config.breadcrumbs.length - 1 ? (
                      <Link
                        href={crumb.url}
                        className="transition-colors hover:text-white/70"
                      >
                        {crumb.name}
                      </Link>
                    ) : (
                      <span>{crumb.name}</span>
                    )}
                  </span>
                ))}
              </nav>
              <p className="mt-10 font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-normal text-cyan-200/70">
                {config.eyebrow}
              </p>
              <h1 className="mt-5 max-w-[14ch] font-[family-name:var(--font-display)] text-5xl font-normal leading-none tracking-normal text-white sm:text-7xl">
                {config.h1}
              </h1>
              <p className="mt-8 max-w-[58ch] text-base leading-8 text-white/62 sm:text-lg">
                {config.heroDescription}
              </p>
              <div className="mt-9 flex flex-col gap-3 sm:flex-row">
                <Link
                  href="/auth/login"
                  className="inline-flex items-center justify-center gap-2 rounded-md bg-white px-6 py-3 text-sm font-medium text-[#060606] transition-colors hover:bg-white/90"
                >
                  Start first race
                  <ArrowRight className="size-4" />
                </Link>
                <Link
                  href="/docs/getting-started/quickstart"
                  className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm font-medium text-white/80 transition-colors hover:border-white/30 hover:text-white"
                >
                  Read quickstart
                  <Terminal className="size-4" />
                </Link>
              </div>
            </div>
            <ProductVisual />
          </div>
        </section>

        <section className="border-y border-white/[0.06] px-6 py-20 sm:px-12">
          <div className="mx-auto max-w-[1440px]">
            <div className="max-w-3xl">
              <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-normal text-white/35">
                {config.proofSectionTitle}
              </p>
              <h2 className="mt-4 text-3xl font-semibold tracking-normal text-white sm:text-5xl">
                Built for reviewable agent decisions
              </h2>
              <p className="mt-5 text-sm leading-7 text-white/55 sm:text-base">
                {config.proofSectionDescription}
              </p>
            </div>
            <div className="mt-12 grid gap-4 md:grid-cols-2 xl:grid-cols-3">
              {config.proofPoints.map((point) => (
                <div
                  key={point}
                  className="rounded-md border border-white/[0.08] bg-white/[0.03] p-5"
                >
                  <CheckCircle2 className="size-5 text-emerald-200" />
                  <p className="mt-4 text-sm leading-6 text-white/78">
                    {point}
                  </p>
                </div>
              ))}
            </div>
          </div>
        </section>

        <section className="px-6 py-20 sm:px-12 sm:py-28">
          <div className="mx-auto max-w-[1440px]">
            <div className="grid gap-12 lg:grid-cols-[0.7fr_1fr]">
              <div>
                <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-normal text-white/35">
                  Workflow
                </p>
                <h2 className="mt-4 text-3xl font-semibold tracking-normal text-white sm:text-5xl">
                  {config.workflowSectionTitle}
                </h2>
              </div>
              <div className="grid gap-4 sm:grid-cols-2">
                {config.workflow.map((item, index) => {
                  const Icon = workflowIcons[index] ?? Code2;
                  return (
                    <div
                      key={item.title}
                      className="rounded-md border border-white/[0.08] bg-white/[0.03] p-5"
                    >
                      <Icon className="size-5 text-cyan-200" />
                      <h3 className="mt-5 text-lg font-semibold tracking-normal text-white">
                        {item.title}
                      </h3>
                      <p className="mt-3 text-sm leading-6 text-white/55">
                        {item.text}
                      </p>
                    </div>
                  );
                })}
              </div>
            </div>
          </div>
        </section>

        <section className="border-y border-white/[0.06] px-6 py-20 sm:px-12">
          <div className="mx-auto grid max-w-[1440px] gap-10 lg:grid-cols-[0.8fr_1fr] lg:items-start">
            <div>
              <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-normal text-white/35">
                {config.docsSectionTitle}
              </p>
              <h2 className="mt-4 text-3xl font-semibold tracking-normal text-white sm:text-5xl">
                Bring your first workload into the loop
              </h2>
              <p className="mt-5 text-sm leading-7 text-white/55 sm:text-base">
                {config.docsSectionDescription}
              </p>
            </div>
            <div className="grid gap-3">
              {config.relatedLinks.map((link) => (
                <Link
                  key={link.href}
                  href={link.href}
                  className="group rounded-md border border-white/[0.08] bg-white/[0.03] p-5 transition-colors hover:border-white/20 hover:bg-white/[0.05]"
                >
                  <div className="flex items-center justify-between gap-4">
                    <div>
                      <h3 className="text-base font-semibold tracking-normal text-white">
                        {link.title}
                      </h3>
                      <p className="mt-2 text-sm leading-6 text-white/50">
                        {link.text}
                      </p>
                    </div>
                    <ArrowRight className="size-4 shrink-0 text-white/35 transition-colors group-hover:text-white/75" />
                  </div>
                </Link>
              ))}
            </div>
          </div>
        </section>

        <section className="px-6 py-20 sm:px-12 sm:py-28">
          <div className="mx-auto max-w-[960px]">
            <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-normal text-white/35">
              FAQ
            </p>
            <h2 className="mt-4 text-3xl font-semibold tracking-normal text-white sm:text-5xl">
              {config.faqSectionTitle}
            </h2>
            <div className="mt-10 divide-y divide-white/[0.08] border-y border-white/[0.08]">
              {config.faqItems.map((item) => (
                <section key={item.question} className="py-6">
                  <h3 className="text-lg font-semibold tracking-normal text-white">
                    {item.question}
                  </h3>
                  <p className="mt-3 text-sm leading-7 text-white/55">
                    {item.answer}
                  </p>
                </section>
              ))}
            </div>
            <div className="mt-10 flex flex-col gap-3 sm:flex-row">
              <Link
                href="/auth/login"
                className="inline-flex items-center justify-center gap-2 rounded-md bg-white px-6 py-3 text-sm font-medium text-[#060606] transition-colors hover:bg-white/90"
              >
                Start first race
                <ArrowRight className="size-4" />
              </Link>
              <a
                href="https://github.com/agentclash/agentclash"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm font-medium text-white/80 transition-colors hover:border-white/30 hover:text-white"
              >
                View on GitHub
                <ArrowRight className="size-4" />
              </a>
            </div>
          </div>
        </section>
      </main>
      <StaticFooter />
    </>
  );
}
