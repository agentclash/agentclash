import type { Metadata } from "next";
import Link from "next/link";
import {
  ArrowRight,
  CheckCircle2,
} from "lucide-react";
import { CalEmbedInit } from "@/components/marketing/cal-embed-init";
import { ClosingCTA } from "@/components/marketing/closing-cta";
import { EnterprisePageCTA } from "@/components/marketing/enterprise-page-cta";
import { FAQBlock } from "@/components/marketing/faq-block";
import { FeatureGrid } from "@/components/marketing/feature-grid";
import {
  JsonLd,
  breadcrumbSchema,
  productSchema,
} from "@/components/marketing/json-ld";
import { MarketingShell } from "@/components/marketing/marketing-shell";
import { SplitSection } from "@/components/marketing/split-section";
import { PRICING_TIERS } from "@/lib/pricing-data";
import { ogImageUrl } from "@/lib/seo";

const PAGE_PATH = "/enterprise";
const PAGE_TITLE =
  "Enterprise AI Agent Evaluation — Governed Release Gates | AgentClash";
const PAGE_DESCRIPTION =
  "Prove which agent is safe to ship with governed benchmarks, replay evidence, scorecards, and CI release gates. 45-day Team pilot — no credit card.";
const SOCIAL_IMAGE = ogImageUrl({
  title: "Enterprise Agent Evaluation",
  subtitle: "Governed release gates for platform teams",
  kind: "Enterprise",
});

const enterpriseTier = PRICING_TIERS.find((tier) => tier.name === "Enterprise");

const buyerQuestions = [
  {
    label: "Trust",
    title: "Which agent should we trust?",
    body: "Compare baseline, candidate, and vendor agents inside one frozen challenge pack — not disconnected eval jobs.",
  },
  {
    label: "Constraints",
    title: "Under which runtime constraints?",
    body: "Attach release policy to the benchmark: latency, TTFT, cost ceilings, and automatic fails on policy violations.",
  },
  {
    label: "Cost",
    title: "At what cost?",
    body: "See cost per successful task alongside correctness and reliability — not token spend in isolation.",
  },
  {
    label: "Evidence",
    title: "Why did it fail?",
    body: "Replay explains divergences: routing fallbacks, tool paths, artifacts, and scorecard axes — not raw log dumps.",
  },
  {
    label: "Defense",
    title: "Can we defend the decision?",
    body: "Export pass/fail recommendations, scorecards, and redacted evidence for security, finance, and engineering leadership.",
  },
];

const productProof = [
  {
    title: "Evidence, not debug logs",
    body: "Inspect tool trajectories, routing, latency spikes, and artifacts in a replay shaped for ship decisions.",
  },
  {
    title: "Benchmark outcomes you can gate on",
    body: "Completion, correctness, cost, reliability, and challenge-specific policy checks in one scorecard.",
  },
  {
    title: "Your workloads, versioned",
    body: "Freeze challenge packs and input sets so every run compares against the same approved benchmark.",
  },
  {
    title: "Block regressions before prod",
    body: "Fail builds when a candidate regresses against baseline on the scorecard your team already trusts.",
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
      "Hosted pilots run on our standard cloud regions today. Enterprise contracts can discuss dedicated deployment, private networking, and residency requirements during the architecture review — contact hello@agentclash.dev.",
  },
  {
    question: "How is the 45-day Team pilot different from a services engagement?",
    answer:
      "The pilot is product access on the Team tier — your workspace, challenge packs, and gates — with no credit card required. Optional hands-on eval sprints (pack build, benchmark setup) are fixed-scope services; ask us about a 2-week eval sprint intro.",
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

      <section className="px-6 pt-20 sm:px-12 sm:pt-28">
        <div className="mx-auto max-w-[1080px]">
          <nav className="flex items-center gap-2 text-xs text-white/35">
            <Link href="/" className="transition-colors hover:text-white/70">
              Home
            </Link>
            <span>/</span>
            <span>Enterprise</span>
          </nav>
          <p className="mt-10 font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-normal text-cyan-200/70">
            Enterprise
          </p>
          <h1 className="mt-5 max-w-[18ch] font-[family-name:var(--font-display)] text-4xl font-normal leading-[1.05] tracking-tight text-white sm:text-6xl">
            Governed agent release gates
          </h1>
          <p className="mt-8 max-w-[62ch] text-base leading-8 text-white/62 sm:text-lg">
            Prove which agent is safe to ship — with replay evidence, scorecards,
            and CI gates your platform, security, and finance teams can defend.
            Not another trace dashboard. A benchmark control room for agent trust.
          </p>
          <EnterprisePageCTA className="mt-10" />
        </div>
      </section>

      <section className="border-t border-white/[0.06] px-6 py-16 sm:px-12 sm:py-24">
        <div className="mx-auto max-w-[1080px]">
          <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.14em] text-white/35">
            The decision loop
          </p>
          <h2 className="mt-4 max-w-[24ch] font-[family-name:var(--font-display)] text-3xl tracking-[-0.02em] text-white sm:text-4xl">
            Five questions every release needs answered
          </h2>
          <p className="mt-4 max-w-[58ch] text-sm leading-relaxed text-white/50">
            Platform leads and vendor committees do not need another eval score.
            They need governed evidence that connects benchmark → replay → gate →
            ship or block.
          </p>
        </div>
        <div className="mt-12">
          <FeatureGrid features={buyerQuestions} columns={2} />
        </div>
      </section>

      <SplitSection
        eyebrow="Product proof"
        title="From live run to release gate — one system"
        body={
          <>
            <p>
              AgentClash turns agent behavior into a decision artifact: frozen
              challenge versions, sandbox execution, replay, scorecards, and
              release gates — without stitching traces, evals, and policy in
              separate tools.
            </p>
            <ul className="mt-6 space-y-3 text-sm text-white/55">
              {productProof.map((item) => (
                <li key={item.title} className="flex gap-2">
                  <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-cyan-200/80" />
                  <span>
                    <span className="font-medium text-white/80">{item.title}.</span>{" "}
                    {item.body}
                  </span>
                </li>
              ))}
            </ul>
          </>
        }
        aside={
          enterpriseTier ? (
            <div className="rounded-lg border border-white/[0.08] bg-white/[0.03] p-6">
              <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.14em] text-white/40">
                Enterprise tier
              </p>
              <p className="mt-3 text-sm leading-relaxed text-white/55">
                {enterpriseTier.blurb}
              </p>
              <ul className="mt-6 space-y-2.5 text-sm text-white/70">
                {enterpriseTier.features.map((feature) => (
                  <li key={feature} className="flex gap-2">
                    <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-emerald-300/80" />
                    {feature}
                  </li>
                ))}
              </ul>
              <Link
                href="/pricing"
                className="mt-6 inline-flex items-center gap-1.5 text-sm text-cyan-200/80 transition-colors hover:text-cyan-100"
              >
                View full pricing
                <ArrowRight className="size-3.5" />
              </Link>
            </div>
          ) : null
        }
      />

      <section className="border-t border-white/[0.06] px-6 py-16 sm:px-12 sm:py-24">
        <div className="mx-auto max-w-[960px]">
          <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.14em] text-white/35">
            Pilot offer
          </p>
          <h2 className="mt-4 font-[family-name:var(--font-display)] text-3xl tracking-[-0.02em] text-white sm:text-4xl">
            Start with a 45-day Team pilot
          </h2>
          <p className="mt-4 max-w-[62ch] text-sm leading-7 text-white/55">
            Run governed benchmarks on your workloads in a dedicated workspace —
            challenge packs, replay retention, CI integration, and workspace audit
            logs. No credit card required.
          </p>
          <div className="mt-8 flex flex-col gap-4 sm:flex-row sm:flex-wrap">
            <Link
              href="/auth/login?plan=team"
              className="inline-flex items-center justify-center gap-2 rounded-md bg-white px-6 py-3 text-sm font-medium text-[#060606] hover:bg-white/90 transition-colors"
            >
              Start Team pilot
              <ArrowRight className="size-4" />
            </Link>
            <a
              href="mailto:hello@agentclash.dev?subject=AgentClash%202-week%20eval%20sprint"
              className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
            >
              Ask about a 2-week eval sprint
              <ArrowRight className="size-4" />
            </a>
          </div>
          <p className="mt-4 text-xs leading-relaxed text-white/40">
            Product pilot vs services: the Team pilot is self-serve product access.
            Fixed-scope eval sprints (pack build, benchmark setup) are optional
            services engagements — we&apos;ll scope them on the architecture review.
          </p>
        </div>
      </section>

      <section className="border-t border-white/[0.06] px-6 py-16 sm:px-12">
        <div className="mx-auto max-w-[960px]">
          <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.14em] text-white/35">
            Explore
          </p>
          <h2 className="mt-4 text-2xl font-semibold tracking-tight text-white sm:text-3xl">
            Related resources
          </h2>
          <ul className="mt-8 grid gap-4 sm:grid-cols-2">
            {crossLinks.map((link) => (
              <li key={link.href}>
                <Link
                  href={link.href}
                  className="group flex h-full flex-col rounded-lg border border-white/[0.08] bg-white/[0.03] px-5 py-4 transition-colors hover:border-white/15"
                >
                  <span className="text-sm font-medium text-white group-hover:text-white/90">
                    {link.label}
                  </span>
                  <span className="mt-2 text-xs leading-relaxed text-white/45">
                    {link.description}
                  </span>
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
      />

      <ClosingCTA
        title={
          <>
            Ready to gate
            <br />
            <span className="text-white/40">your next agent release?</span>
          </>
        }
        body="Book a 30-minute eval architecture review or email us to scope a Team pilot on your workloads."
      >
        <EnterprisePageCTA />
      </ClosingCTA>
    </MarketingShell>
  );
}
