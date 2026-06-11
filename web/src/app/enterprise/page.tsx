import type { Metadata } from "next";
import Link from "next/link";
import {
  ArrowRight,
  CheckCircle2,
  GitBranch,
  Lock,
  Play,
  ShieldCheck,
  Sparkles,
} from "lucide-react";
import { CalEmbedInit } from "@/components/marketing/cal-embed-init";
import { EnterprisePageCTA } from "@/components/marketing/enterprise-page-cta";
import { EnterpriseHeroVisual } from "@/components/marketing/enterprise/enterprise-hero-visual";
import { FAQBlock } from "@/components/marketing/faq-block";
import {
  JsonLd,
  breadcrumbSchema,
  productSchema,
} from "@/components/marketing/json-ld";
import { MarketingShell } from "@/components/marketing/marketing-shell";
import { PRICING_TIERS } from "@/lib/pricing-data";
import { ogImageUrl } from "@/lib/seo";

const PAGE_PATH = "/enterprise";
const PAGE_TITLE =
  "Enterprise AI Agent Evaluation — Release Gates & Pilot | AgentClash";
const PAGE_DESCRIPTION =
  "Prove which agent is safe to ship with governed benchmarks, replay evidence, scorecards, and CI release gates. Start a 45-day Team pilot with no credit card.";
const SOCIAL_IMAGE = ogImageUrl({
  title: "Enterprise Agent Evaluation",
  subtitle: "Release gates platform teams can defend",
  kind: "Enterprise",
});

const enterpriseTier = PRICING_TIERS.find((tier) => tier.name === "Enterprise");

const trustItems = [
  "MIT open source",
  "Bring your own keys",
  "45-day Team pilot",
  "No token markup",
];

const buyerQuestions = [
  {
    num: "01",
    title: "Which agent should we trust?",
    body: "Race baseline, candidate, and vendor agents on the same frozen challenge pack instead of disconnected eval jobs.",
  },
  {
    num: "02",
    title: "Under which constraints?",
    body: "Attach latency, cost, and policy ceilings to the benchmark. Fail the run when a candidate breaks your release rules.",
  },
  {
    num: "03",
    title: "At what cost?",
    body: "See cost per successful task next to correctness and reliability. Not token spend in isolation.",
  },
  {
    num: "04",
    title: "Why did it fail?",
    body: "Replay shows routing, tool paths, artifacts, and scorecard axes. Not another log dump.",
  },
  {
    num: "05",
    title: "Can we defend the decision?",
    body: "Export pass/fail recommendations, scorecards, and redacted evidence for security, finance, and engineering leadership.",
  },
];

const workflow = [
  {
    icon: GitBranch,
    title: "Freeze the benchmark",
    text: "Version challenge packs and inputs so every run compares against the same approved workload.",
  },
  {
    icon: Play,
    title: "Race candidates",
    text: "Run agents in sandbox with the same tools, time budget, and scoring rules.",
  },
  {
    icon: Sparkles,
    title: "Review replay evidence",
    text: "Inspect trajectories, artifacts, cost, and scorecards before anyone argues from vibes.",
  },
  {
    icon: ShieldCheck,
    title: "Gate the release",
    text: "Fail CI when a candidate regresses against baseline on the scorecard your team already trusts.",
  },
];

const faqItems = [
  {
    question: "Can we self-host instead of using the hosted pilot?",
    answer:
      "Yes. AgentClash is MIT-licensed and open source. Many enterprises start hosted for the 45-day Team pilot, then move to self-host or a hybrid model. See the self-host guide in docs for the full stack.",
  },
  {
    question: "Do you mark up LLM tokens?",
    answer:
      "No. AgentClash is bring-your-own-key (BYOK) on every tier. You connect provider keys and pay vendors directly; we never mark up tokens.",
  },
  {
    question: "What about data residency for UAE and other regions?",
    answer:
      "Hosted pilots run on our standard cloud regions today. Enterprise contracts can discuss dedicated deployment, private networking, and residency requirements during the architecture review. Contact hello@agentclash.dev.",
  },
  {
    question: "How is the 45-day Team pilot different from a services engagement?",
    answer:
      "The pilot is product access on the Team tier: your workspace, challenge packs, and gates, with no credit card required. Optional hands-on eval sprints (pack build, benchmark setup) are fixed-scope services. Ask us about a 2-week eval sprint intro.",
  },
];

const crossLinks = [
  {
    href: "/platform/agent-evaluation",
    label: "Agent evaluation platform",
    description: "Same-tools races, sandbox execution, and replay on real tasks.",
  },
  {
    href: "/platform/agent-regression-testing",
    label: "Agent regression testing",
    description: "Turn failed runs into permanent gates in CI.",
  },
  {
    href: "/compare",
    label: "Compare eval tools",
    description: "How AgentClash differs from prompt-eval and observability stacks.",
  },
  {
    href: "/pricing",
    label: "Pricing",
    description: "Free, Pro, Team, and custom Enterprise tiers.",
  },
];

export const metadata: Metadata = {
  title: PAGE_TITLE,
  description: PAGE_DESCRIPTION,
  alternates: { canonical: PAGE_PATH },
  openGraph: {
    title: PAGE_TITLE,
    description: PAGE_DESCRIPTION,
    url: PAGE_PATH,
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

export default function EnterprisePage() {
  return (
    <MarketingShell>
      <CalEmbedInit />
      <JsonLd
        id="agentclash-enterprise-schema"
        data={[
          breadcrumbSchema([
            { name: "Home", url: "/" },
            { name: "Enterprise", url: PAGE_PATH },
          ]),
          productSchema({
            name: "AgentClash Enterprise",
            description: PAGE_DESCRIPTION,
            url: `https://www.agentclash.dev${PAGE_PATH}`,
            applicationSubCategory: "Enterprise AI agent evaluation",
          }),
        ]}
      />

      <section className="px-6 pt-16 sm:px-12 sm:pt-24">
        <div className="mx-auto grid max-w-[1200px] gap-14 lg:grid-cols-[1fr_0.95fr] lg:items-center">
          <div>
            <nav className="flex items-center gap-2 text-xs text-white/35">
              <Link href="/" className="transition-colors hover:text-white/70">
                Home
              </Link>
              <span>/</span>
              <span>Enterprise</span>
            </nav>
            <p className="mt-8 font-mono text-[11px] uppercase tracking-[0.14em] text-cyan-200/80">
              Enterprise evaluation
            </p>
            <h1 className="mt-4 max-w-[14ch] text-[clamp(2.25rem,5vw,3.75rem)] font-sans font-semibold leading-[1.08] tracking-[-0.03em] text-white">
              Ship agents with evidence your team can defend
            </h1>
            <p className="mt-6 max-w-[52ch] text-base leading-7 text-white/60 sm:text-lg sm:leading-8">
              AgentClash turns agent behavior into a release decision: frozen
              benchmarks, replay, scorecards, and CI gates. Built for platform,
              security, and vendor review committees, not another trace dashboard.
            </p>
            <EnterprisePageCTA className="mt-8" />
            <ul className="mt-8 flex flex-wrap gap-x-5 gap-y-2 border-t border-white/[0.06] pt-6">
              {trustItems.map((item) => (
                <li
                  key={item}
                  className="flex items-center gap-2 text-xs text-white/45"
                >
                  <CheckCircle2 className="size-3.5 shrink-0 text-emerald-300/80" />
                  {item}
                </li>
              ))}
            </ul>
          </div>
          <EnterpriseHeroVisual />
        </div>
      </section>

      <section className="border-t border-white/[0.06] px-6 py-20 sm:px-12 sm:py-28">
        <div className="mx-auto max-w-[1200px]">
          <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-white/40">
            Release committee
          </p>
          <h2 className="mt-3 max-w-[20ch] text-3xl font-semibold tracking-tight text-white sm:text-4xl">
            Five questions every agent release needs answered
          </h2>
          <p className="mt-4 max-w-[56ch] text-sm leading-7 text-white/55 sm:text-base">
            Platform leads do not need another leaderboard. They need governed
            evidence that connects benchmark, replay, gate, and ship or block.
          </p>
          <ol className="mt-12 grid gap-4 sm:grid-cols-2">
            {buyerQuestions.map((item) => (
              <li
                key={item.num}
                className="group relative overflow-hidden rounded-xl border border-white/[0.08] bg-gradient-to-br from-white/[0.04] to-transparent p-6 transition-colors hover:border-white/[0.14]"
              >
                <span className="font-mono text-[11px] tabular-nums text-cyan-200/70">
                  {item.num}
                </span>
                <h3 className="mt-3 text-lg font-semibold tracking-tight text-white">
                  {item.title}
                </h3>
                <p className="mt-2 text-sm leading-6 text-white/55">{item.body}</p>
              </li>
            ))}
          </ol>
        </div>
      </section>

      <section className="border-t border-white/[0.06] px-6 py-20 sm:px-12 sm:py-28">
        <div className="mx-auto max-w-[1200px]">
          <div className="grid gap-12 lg:grid-cols-[0.85fr_1.15fr] lg:items-start">
            <div>
              <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-white/40">
                How it works
              </p>
              <h2 className="mt-3 text-3xl font-semibold tracking-tight text-white sm:text-4xl">
                From live run to release gate in one system
              </h2>
              <p className="mt-4 text-sm leading-7 text-white/55 sm:text-base">
                No stitching traces, eval spreadsheets, and policy docs in
                separate tools. AgentClash produces one decision artifact your
                team can gate on.
              </p>
              {enterpriseTier ? (
                <div className="mt-10 rounded-xl border border-white/[0.08] bg-white/[0.03] p-6">
                  <div className="flex items-center gap-2">
                    <Lock className="size-4 text-white/50" />
                    <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-white/45">
                      Enterprise tier
                    </p>
                  </div>
                  <p className="mt-3 text-sm leading-relaxed text-white/55">
                    {enterpriseTier.blurb}
                  </p>
                  <ul className="mt-5 space-y-2.5 text-sm text-white/75">
                    {enterpriseTier.features.map((feature) => (
                      <li key={feature} className="flex gap-2">
                        <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-emerald-300/80" />
                        {feature}
                      </li>
                    ))}
                  </ul>
                  <Link
                    href="/pricing"
                    className="mt-6 inline-flex items-center gap-1.5 text-sm font-medium text-cyan-200/90 transition-colors hover:text-cyan-100"
                  >
                    View full pricing
                    <ArrowRight className="size-3.5" />
                  </Link>
                </div>
              ) : null}
            </div>
            <ol className="space-y-4">
              {workflow.map((step, index) => {
                const Icon = step.icon;
                return (
                  <li
                    key={step.title}
                    className="flex gap-4 rounded-xl border border-white/[0.08] bg-[#0a0a0a] p-5"
                  >
                    <div className="flex size-10 shrink-0 items-center justify-center rounded-lg border border-white/[0.1] bg-white/[0.04]">
                      <Icon className="size-4 text-white/70" strokeWidth={1.5} />
                    </div>
                    <div>
                      <p className="font-mono text-[10px] uppercase tracking-[0.14em] text-white/35">
                        Step {index + 1}
                      </p>
                      <h3 className="mt-1 text-base font-semibold text-white">
                        {step.title}
                      </h3>
                      <p className="mt-1.5 text-sm leading-6 text-white/55">
                        {step.text}
                      </p>
                    </div>
                  </li>
                );
              })}
            </ol>
          </div>
        </div>
      </section>

      <section className="border-t border-white/[0.06] px-6 py-20 sm:px-12">
        <div className="mx-auto max-w-[960px]">
          <div className="overflow-hidden rounded-2xl border border-white/[0.1] bg-gradient-to-br from-cyan-500/[0.08] via-transparent to-transparent p-8 sm:p-10">
            <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-cyan-200/80">
              Pilot offer
            </p>
            <h2 className="mt-3 text-3xl font-semibold tracking-tight text-white sm:text-4xl">
              Start with a 45-day Team pilot
            </h2>
            <p className="mt-4 max-w-[58ch] text-sm leading-7 text-white/60 sm:text-base">
              Run governed benchmarks on your workloads in a dedicated workspace:
              challenge packs, replay retention, CI integration, and workspace
              audit logs. No credit card required.
            </p>
            <ul className="mt-6 grid gap-2 sm:grid-cols-2">
              {[
                "Dedicated workspace on Team tier",
                "Challenge packs and replay retention",
                "CI integration and audit logs",
                "Architecture review with our team",
              ].map((item) => (
                <li key={item} className="flex gap-2 text-sm text-white/70">
                  <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-emerald-300/80" />
                  {item}
                </li>
              ))}
            </ul>
            <div className="mt-8 flex flex-col gap-3 sm:flex-row sm:flex-wrap">
              <Link
                href="/auth/login?plan=team"
                className="inline-flex items-center justify-center gap-2 rounded-lg bg-white px-6 py-3.5 text-sm font-semibold text-[#060606] transition-colors hover:bg-white/90"
              >
                Start Team pilot
                <ArrowRight className="size-4" />
              </Link>
              <a
                href="mailto:hello@agentclash.dev?subject=AgentClash%202-week%20eval%20sprint"
                className="inline-flex items-center justify-center gap-2 rounded-lg border border-white/20 bg-black/20 px-6 py-3.5 text-sm font-medium text-white/85 transition-colors hover:border-white/35 hover:text-white"
              >
                Ask about a 2-week eval sprint
                <ArrowRight className="size-4" />
              </a>
            </div>
            <p className="mt-5 text-xs leading-relaxed text-white/45">
              The Team pilot is self-serve product access. Fixed-scope eval
              sprints are optional services we scope on the architecture review.
            </p>
          </div>
        </div>
      </section>

      <section className="border-t border-white/[0.06] px-6 py-16 sm:px-12">
        <div className="mx-auto max-w-[960px]">
          <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-white/40">
            Explore
          </p>
          <h2 className="mt-3 text-2xl font-semibold tracking-tight text-white sm:text-3xl">
            Related resources
          </h2>
          <ul className="mt-8 grid gap-3 sm:grid-cols-2">
            {crossLinks.map((link) => (
              <li key={link.href}>
                <Link
                  href={link.href}
                  className="group flex h-full items-start justify-between gap-4 rounded-xl border border-white/[0.08] bg-white/[0.02] px-5 py-4 transition-all hover:border-white/16 hover:bg-white/[0.04]"
                >
                  <div>
                    <span className="text-sm font-semibold text-white group-hover:text-white">
                      {link.label}
                    </span>
                    <span className="mt-1.5 block text-xs leading-relaxed text-white/45">
                      {link.description}
                    </span>
                  </div>
                  <ArrowRight className="mt-0.5 size-4 shrink-0 text-white/30 transition-transform group-hover:translate-x-0.5 group-hover:text-white/60" />
                </Link>
              </li>
            ))}
          </ul>
        </div>
      </section>

      <FAQBlock
        schemaId="agentclash-enterprise-faq-schema"
        eyebrow="FAQ"
        title="Enterprise evaluation questions"
        items={faqItems}
        sansHeadlines
      />

      <section className="border-t border-white/[0.06] px-6 py-20 sm:px-12 sm:py-28">
        <div className="mx-auto max-w-[960px]">
          <h2 className="max-w-[16ch] text-3xl font-semibold tracking-tight text-white sm:text-4xl">
            Ready to gate your next agent release?
          </h2>
          <p className="mt-4 max-w-[52ch] text-base leading-7 text-white/55">
            Book a 30-minute eval architecture review or email us to scope a Team
            pilot on your workloads.
          </p>
          <EnterprisePageCTA className="mt-8" />
        </div>
      </section>
    </MarketingShell>
  );
}
