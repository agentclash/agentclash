import type { Metadata } from "next";
import Link from "next/link";
import { CalEmbedInit } from "@/components/marketing/cal-embed-init";
import { EnterprisePageCTA } from "@/components/marketing/enterprise-page-cta";
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
    body: "Compare baseline, candidate, and vendor agents inside one frozen challenge pack, not disconnected eval jobs.",
  },
  {
    num: "02",
    title: "Under which constraints?",
    body: "Attach latency, cost, and policy ceilings to the benchmark. The run fails when a candidate breaks your release rules.",
  },
  {
    num: "03",
    title: "At what cost?",
    body: "See cost per successful task next to correctness and reliability, not token spend in isolation.",
  },
  {
    num: "04",
    title: "Why did it fail?",
    body: "Replay shows routing, tool paths, artifacts, and scorecard axes. Not another log dump.",
  },
  {
    num: "05",
    title: "Can we defend the decision?",
    body: "Export pass and fail recommendations, scorecards, and redacted evidence for security, finance, and engineering leadership.",
  },
];

const workflow = [
  {
    num: "01",
    title: "Freeze the benchmark",
    text: "Version challenge packs and inputs so every run compares against the same approved workload.",
  },
  {
    num: "02",
    title: "Race candidates",
    text: "Run agents in a sandbox with the same tools, time budget, and scoring rules.",
  },
  {
    num: "03",
    title: "Review replay evidence",
    text: "Inspect trajectories, artifacts, cost, and scorecards before anyone argues from anecdotes.",
  },
  {
    num: "04",
    title: "Gate the release",
    text: "Fail CI when a candidate regresses against baseline on the scorecard your team already trusts.",
  },
];

const pilotIncludes = [
  "Dedicated workspace on the Team tier",
  "Challenge packs and replay retention",
  "CI integration and audit logs",
  "Architecture review with our team",
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
  {
    href: "/services",
    label: "Eval services",
    description: "Fixed-scope pack build, benchmark setup, and managed eval retainers.",
  },
  {
    href: "/industries",
    label: "Industry playbooks",
    description: "Banking, insurance, and government evaluation starting points.",
  },
  {
    href: "/glossary",
    label: "Glossary",
    description: "Agent evaluation, challenge packs, and release gate definitions.",
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

const eyebrowClass =
  "font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.16em] text-white/40";

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

      {/* Hero: pure typographic, no product chrome */}
      <section className="px-6 pt-20 sm:px-12 sm:pt-32">
        <div className="mx-auto max-w-[960px]">
          <nav className="flex items-center gap-2 text-xs text-white/35">
            <Link href="/" className="transition-colors hover:text-white/70">
              Home
            </Link>
            <span aria-hidden>/</span>
            <span>Enterprise</span>
          </nav>
          <p className={`mt-12 ${eyebrowClass}`}>Enterprise evaluation</p>
          <h1 className="mt-6 max-w-[16ch] text-[clamp(2.5rem,6vw,4.5rem)] font-sans font-semibold leading-[1.04] tracking-[-0.03em] text-white">
            Ship agents with evidence your team can defend
          </h1>
          <p className="mt-8 max-w-[58ch] text-lg leading-8 text-white/60">
            AgentClash turns agent behavior into a release decision: frozen
            benchmarks, replay, scorecards, and CI gates. Built for platform,
            security, and vendor review committees, not another trace dashboard.
          </p>
          <EnterprisePageCTA className="mt-10" />
          <ul className="mt-12 flex flex-wrap items-center gap-x-3 gap-y-2 border-t border-white/[0.08] pt-6 text-sm text-white/45">
            {trustItems.map((item, index) => (
              <li key={item} className="flex items-center gap-3">
                {index > 0 ? (
                  <span aria-hidden className="text-white/20">
                    /
                  </span>
                ) : null}
                {item}
              </li>
            ))}
          </ul>
        </div>
      </section>

      {/* Buyer questions */}
      <section className="px-6 pt-28 sm:px-12 sm:pt-40">
        <div className="mx-auto max-w-[960px]">
          <p className={eyebrowClass}>Release committee</p>
          <h2 className="mt-4 max-w-[20ch] text-3xl font-sans font-semibold tracking-[-0.02em] text-white sm:text-[2.75rem] sm:leading-[1.1]">
            Five questions every agent release needs answered
          </h2>
          <p className="mt-5 max-w-[56ch] text-base leading-7 text-white/55">
            Platform leads do not need another leaderboard. They need governed
            evidence that connects benchmark, replay, gate, and the decision to
            ship or block.
          </p>
          <ol className="mt-14 border-t border-white/[0.08]">
            {buyerQuestions.map((item) => (
              <li
                key={item.num}
                className="grid grid-cols-1 gap-2 border-b border-white/[0.08] py-8 sm:grid-cols-[7rem_1fr] sm:gap-8 sm:py-9"
              >
                <span className="text-sm font-sans tabular-nums text-white/30">
                  {item.num}
                </span>
                <div>
                  <h3 className="text-xl font-sans font-semibold tracking-[-0.01em] text-white">
                    {item.title}
                  </h3>
                  <p className="mt-3 max-w-[60ch] text-base leading-7 text-white/55">
                    {item.body}
                  </p>
                </div>
              </li>
            ))}
          </ol>
        </div>
      </section>

      {/* How it works + enterprise tier */}
      <section className="px-6 pt-28 sm:px-12 sm:pt-40">
        <div className="mx-auto grid max-w-[1080px] gap-16 lg:grid-cols-[0.9fr_1.1fr] lg:gap-20">
          <div className="lg:sticky lg:top-28 lg:self-start">
            <p className={eyebrowClass}>How it works</p>
            <h2 className="mt-4 max-w-[16ch] text-3xl font-sans font-semibold tracking-[-0.02em] text-white sm:text-[2.5rem] sm:leading-[1.1]">
              From live run to release gate in one system
            </h2>
            <p className="mt-5 max-w-[44ch] text-base leading-7 text-white/55">
              No stitching together traces, eval spreadsheets, and policy docs
              in separate tools. AgentClash produces one decision artifact your
              team can gate on.
            </p>

            {enterpriseTier ? (
              <div className="mt-12 rounded-xl border border-white/[0.1] bg-white/[0.02] p-7">
                <p className={eyebrowClass}>Enterprise tier</p>
                <p className="mt-4 text-base leading-7 text-white/60">
                  {enterpriseTier.blurb}
                </p>
                <ul className="mt-6 divide-y divide-white/[0.08] border-t border-white/[0.08]">
                  {enterpriseTier.features.map((feature) => (
                    <li
                      key={feature}
                      className="py-3 text-sm leading-6 text-white/70"
                    >
                      {feature}
                    </li>
                  ))}
                </ul>
                <Link
                  href="/pricing"
                  className="mt-6 inline-block text-sm font-medium text-white/70 underline decoration-white/20 underline-offset-4 transition-colors hover:text-white hover:decoration-white/50"
                >
                  View full pricing
                </Link>
              </div>
            ) : null}
          </div>

          <ol>
            {workflow.map((step) => (
              <li
                key={step.num}
                className="grid grid-cols-[3rem_1fr] gap-5 border-b border-white/[0.08] py-8 first:border-t"
              >
                <span className="text-sm font-sans tabular-nums text-white/30">
                  {step.num}
                </span>
                <div>
                  <h3 className="text-lg font-sans font-semibold tracking-[-0.01em] text-white">
                    {step.title}
                  </h3>
                  <p className="mt-2.5 text-base leading-7 text-white/55">
                    {step.text}
                  </p>
                </div>
              </li>
            ))}
          </ol>
        </div>
      </section>

      {/* Pilot offer */}
      <section className="px-6 pt-28 sm:px-12 sm:pt-40">
        <div className="mx-auto max-w-[1080px]">
          <div className="rounded-2xl border border-white/[0.1] bg-white/[0.02] p-8 sm:p-12">
            <p className={eyebrowClass}>Pilot offer</p>
            <h2 className="mt-4 max-w-[18ch] text-3xl font-sans font-semibold tracking-[-0.02em] text-white sm:text-[2.5rem] sm:leading-[1.1]">
              Start with a 45-day Team pilot
            </h2>
            <p className="mt-5 max-w-[58ch] text-base leading-7 text-white/60">
              Run governed benchmarks on your workloads in a dedicated
              workspace: challenge packs, replay retention, CI integration, and
              workspace audit logs. No credit card required.
            </p>
            <ul className="mt-10 grid gap-x-12 gap-y-4 border-t border-white/[0.08] pt-8 sm:grid-cols-2">
              {pilotIncludes.map((item) => (
                <li
                  key={item}
                  className="text-base leading-7 text-white/70"
                >
                  {item}
                </li>
              ))}
            </ul>
            <div className="mt-10 flex flex-col gap-3 sm:flex-row sm:flex-wrap">
              <Link
                href="/auth/login?plan=team"
                className="inline-flex items-center justify-center rounded-lg bg-white px-7 py-3.5 text-sm font-semibold text-[#060606] transition-colors hover:bg-white/90"
              >
                Start Team pilot
              </Link>
              <a
                href="mailto:hello@agentclash.dev?subject=AgentClash%202-week%20eval%20sprint"
                className="inline-flex items-center justify-center rounded-lg border border-white/15 px-7 py-3.5 text-sm font-medium text-white/85 transition-colors hover:border-white/35 hover:text-white"
              >
                Ask about a 2-week eval sprint
              </a>
            </div>
            <p className="mt-6 max-w-[64ch] text-sm leading-6 text-white/45">
              The Team pilot is self-serve product access. Fixed-scope eval
              sprints are optional{" "}
              <Link
                href="/services"
                className="text-white/65 underline decoration-white/20 underline-offset-4 transition-colors hover:text-white/85"
              >
                services packages
              </Link>{" "}
              we scope on the architecture review.
            </p>
          </div>
        </div>
      </section>

      {/* Related resources */}
      <section className="px-6 pt-28 sm:px-12 sm:pt-40">
        <div className="mx-auto max-w-[1080px]">
          <p className={eyebrowClass}>Explore</p>
          <h2 className="mt-4 text-2xl font-sans font-semibold tracking-[-0.02em] text-white sm:text-3xl">
            Related resources
          </h2>
          <ul className="mt-10 grid gap-px overflow-hidden rounded-xl border border-white/[0.08] bg-white/[0.08] sm:grid-cols-2">
            {crossLinks.map((link) => (
              <li key={link.href}>
                <Link
                  href={link.href}
                  className="flex h-full flex-col bg-[#060606] px-6 py-6 transition-colors hover:bg-white/[0.025]"
                >
                  <span className="text-base font-sans font-semibold text-white">
                    {link.label}
                  </span>
                  <span className="mt-2 text-sm leading-6 text-white/45">
                    {link.description}
                  </span>
                </Link>
              </li>
            ))}
          </ul>
        </div>
      </section>

      <div className="mt-28 sm:mt-40">
        <FAQBlock
          schemaId="agentclash-enterprise-faq-schema"
          eyebrow="FAQ"
          title="Enterprise evaluation questions"
          items={faqItems}
          sansHeadlines
        />
      </div>

      {/* Closing CTA */}
      <section className="border-t border-white/[0.06] px-6 py-28 sm:px-12 sm:py-40">
        <div className="mx-auto max-w-[960px]">
          <h2 className="max-w-[18ch] text-[clamp(2rem,4.5vw,3.25rem)] font-sans font-semibold leading-[1.08] tracking-[-0.03em] text-white">
            Ready to gate your next agent release?
          </h2>
          <p className="mt-6 max-w-[52ch] text-lg leading-8 text-white/55">
            Book a 30-minute eval architecture review, or email us to scope a
            Team pilot on your workloads.
          </p>
          <EnterprisePageCTA className="mt-10" />
        </div>
      </section>
    </MarketingShell>
  );
}
