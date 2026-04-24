import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { MarketingShell } from "@/components/marketing/marketing-shell";
import { PageHeader } from "@/components/marketing/page-header";
import { SplitSection } from "@/components/marketing/split-section";
import { FeatureGrid } from "@/components/marketing/feature-grid";
import { ClosingCTA } from "@/components/marketing/closing-cta";
import { DemoButton } from "@/components/marketing/demo-button";
import { CodeCard } from "@/components/marketing/code-card";
import { FAQBlock } from "@/components/marketing/faq-block";
import {
  JsonLd,
  breadcrumbSchema,
  productSchema,
} from "@/components/marketing/json-ld";

const PATH = "/v2/design-partners";

export const metadata: Metadata = {
  title: "Design partner program — AgentClash",
  description:
    "Work directly with the AgentClash maintainers during private beta. Roadmap influence, custom challenge-pack help, hosted workspaces, and no cost during beta. About ten partners total.",
  alternates: { canonical: PATH },
  openGraph: {
    title: "AgentClash design partner program",
    description:
      "Small cohort of teams shipping agents to production. Direct Slack with maintainers, roadmap influence, hosted workspaces during beta.",
    url: PATH,
  },
};

const FEATURES = [
  {
    label: "Direct line",
    title: "Shared Slack with maintainers.",
    body: "You get a private channel with the engineers who wrote the scoring pipeline. Bugs get answered, not tiered. Feature questions get honest yes/no/when, not roadmap theater.",
  },
  {
    label: "Roadmap influence",
    title: "We publish what you asked for.",
    body: "Design-partner asks land on the public roadmap with attribution (if you want it) and a shipping window. You see your own feedback turn into commits — and you see us say no when the right answer is no.",
  },
  {
    label: "Hosted workspace",
    title: "Private-beta managed cloud.",
    body: "A hosted workspace on AgentClash Cloud during the beta — Temporal, Postgres, sandbox provisioning, replay archive, all operated by us. No cost during the beta window.",
  },
  {
    label: "Pack authorship",
    title: "We write the first pack with you.",
    body: "The hardest part of agent eval is the first challenge pack. We sit on a call, walk your real task through rubric authorship, and hand you a working pack with tests passing. You own the file from there.",
  },
  {
    label: "CI integrations",
    title: "Early access to CI gating.",
    body: "Early builds of the GitHub Checks integration, GitLab pipeline hooks, and nightly drift sweeps. Partners shape what 'block the merge' actually looks like in their own CI before it's public.",
  },
  {
    label: "Case study",
    title: "Optional. On your timeline.",
    body: "A case study is available if and when it helps you — we'll draft it, you approve every word, you say when it publishes. If you'd rather stay unnamed, that's also fine. Your logo is never a condition.",
  },
];

const FAQ_ITEMS = [
  {
    question: "Is there a cost during beta?",
    answer:
      "No. Managed cloud usage, sandbox minutes, replay storage, and direct support are free for design partners during private beta. When we turn on pricing, partners get early notice, migration help, and a grandfathered window — we're not baiting and switching.",
  },
  {
    question: "Do I have to do a case study?",
    answer:
      "No. The case study is optional, drafted by us, approved by you, and published on your timeline. Many partners never do one; that's fine. Your feedback is the value — a logo on /customers is a bonus, not a gate.",
  },
  {
    question: "What if we self-host?",
    answer:
      "Still eligible. Self-host partners get the same direct Slack, the same roadmap influence, and the same pack-authorship help. You miss the hosted workspace (because you don't need it), but the rest is identical. We work with both shapes.",
  },
  {
    question: "How many partners are you onboarding?",
    answer:
      "Around ten in this cohort. The number is small on purpose — we promise every partner real weekly access to the maintainers, and the math only works if we keep the group tight. If the fit is right, we'd rather stretch the cohort than dilute the access.",
  },
  {
    question: "How do I apply?",
    answer:
      "Book a 20-minute fit call from this page. We talk about your actual agent stack, the regression pain point, and whether the thing you need is in our next three months. If the fit is clear, we onboard immediately; if it isn't, we say so and often point you somewhere better.",
  },
];

export default function DesignPartnersPage() {
  return (
    <>
      <JsonLd
        id="ld-partners-product"
        data={productSchema({
          name: "AgentClash — design partner program",
          description:
            "Private-beta design partner program. Direct access to AgentClash maintainers, roadmap influence, hosted workspaces, and pack-authorship help for teams shipping agents to production.",
          url: PATH,
        })}
      />
      <JsonLd
        id="ld-partners-breadcrumbs"
        data={breadcrumbSchema([
          { name: "Home", url: "/v2" },
          { name: "Design partners", url: PATH },
        ])}
      />
      <MarketingShell>
        <PageHeader
          breadcrumbs={[
            { label: "Home", href: "/v2" },
            { label: "Design partners" },
          ]}
          eyebrow="Design partner program"
          title={
            <>
              Build this with us.
              <br />
              <span className="text-white/40">Not for us.</span>
            </>
          }
          subtitle={
            <>
              We&apos;re working directly with a small group of teams
              shipping agents into production. Design partners get direct
              access to the maintainers, shape the roadmap, and get a
              hosted workspace during private beta. About ten partners
              total.
            </>
          }
          cta={
            <div className="flex flex-col sm:flex-row sm:flex-wrap sm:items-center gap-3">
              <DemoButton label="Book a fit call" />
              <Link
                href="/v2/cloud"
                className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
              >
                Managed cloud
                <ArrowRight className="size-4" />
              </Link>
              <Link
                href="/v2/oss"
                className="inline-flex items-center justify-center gap-2 rounded-md border border-white/[0.08] bg-white/[0.02] px-6 py-3 text-sm font-medium text-white/60 hover:text-white/90 hover:border-white/20 transition-colors"
              >
                Self-host instead
                <ArrowRight className="size-4" />
              </Link>
            </div>
          }
          aside={
            <CodeCard
              title="Program shape"
              code={`cohort:      ~10 teams
window:      private beta
cost:        $0 during beta
cadence:     30 min weekly
channel:     shared Slack
commitment:  honest feedback`}
            />
          }
        />

        <SplitSection
          eyebrow="Who this is for"
          title={
            <>
              Teams shipping agents
              <br />
              <span className="text-white/40">to real users.</span>
            </>
          }
          body={
            <>
              <p>
                The partner program exists to sharpen the product against
                actual production pain. That means we&apos;re most useful
                to teams who already have agents in front of real users
                and a recurring regression story they&apos;re tired of
                telling.
              </p>
              <p className="mt-4">
                If you&apos;re still pre-prototype, we&apos;ll point you
                at the open-source quickstart and revisit when you&apos;re
                further along. The fit conversation on the first call is
                honest — nobody benefits from the wrong match.
              </p>
            </>
          }
          aside={
            <CodeCard
              title="Ideal-fit profile"
              code={`you:
  - ship agents to production users
  - have a recurring regression pain
  - care about cost / latency / drift
  - willing to talk weekly in beta

your stack (any one is enough):
  - OpenAI / Anthropic / Gemini / xAI
  - tool-using agents in a loop
  - CI that blocks merges

not a fit (yet):
  - pre-prototype, no real users
  - one-shot completions, no tools
  - no appetite for weekly feedback`}
            />
          }
        />

        <SplitSection
          reverse
          eyebrow="What we ask in return"
          title={
            <>
              Small list.
              <br />
              <span className="text-white/40">No surprises.</span>
            </>
          }
          body={
            <>
              <p>
                The partnership is a two-way street — not a discount in
                exchange for a press release. We need honest signal about
                what&apos;s working and what isn&apos;t, and we need it
                while the product is still shaped like clay.
              </p>
              <p className="mt-4">
                Everything below is the full list. No gotchas, no
                mid-beta renegotiation, no hidden exclusivity. If any of
                these don&apos;t fit your team, tell us on the first call
                and we&apos;ll figure out whether a lighter arrangement
                works.
              </p>
            </>
          }
          aside={
            <CodeCard
              title="What we ask"
              code={`1. honest feedback
     say when something sucks.
     say when something finally worked.

2. 30 minutes weekly
     we bring the agenda.
     you bring the week's friction.

3. pack re-run on new features
     point your challenge pack at
     new scoring features before GA.

4. optional: logo on /customers
     only if you opt in.
     only when you're ready.`}
            />
          }
        />

        <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
          <div className="mx-auto max-w-[1440px]">
            <div className="max-w-[52ch]">
              <p className="mb-6 inline-flex items-center gap-2 text-[11px] font-[family-name:var(--font-mono)] uppercase tracking-[0.22em] text-white/45">
                <span className="inline-block size-1 rounded-full bg-white/60" />
                What you get
              </p>
              <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2rem,4.5vw,3.75rem)] max-w-[22ch]">
                Six things the partnership quietly delivers.
              </h2>
            </div>
            <div className="mt-20">
              <FeatureGrid features={FEATURES} columns={3} />
            </div>
          </div>
        </section>

        <FAQBlock items={FAQ_ITEMS} schemaId="ld-partners-faq" />

        <ClosingCTA
          title={
            <>
              Twenty minutes.
              <br />
              <span className="text-white/40">Honest on both sides.</span>
            </>
          }
          body={
            <p>
              The fit call is short on purpose. We talk about your actual
              stack, your actual regression pain, and whether the thing
              you need is in our next three months. If it isn&apos;t,
              we&apos;ll say so.
            </p>
          }
        >
          <div className="flex flex-col sm:flex-row sm:flex-wrap gap-3">
            <DemoButton
              label="Book the fit call"
              className="inline-flex items-center justify-center gap-2 rounded-md bg-white px-7 py-3 text-sm font-medium text-[#060606] hover:bg-white/90 transition-colors"
            />
            <Link
              href="/v2/methodology"
              className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-7 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
            >
              How we score
              <ArrowRight className="size-4" />
            </Link>
          </div>
        </ClosingCTA>
      </MarketingShell>
    </>
  );
}
