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

const PATH = "/v2/use-cases/support-agents";

export const metadata: Metadata = {
  title: "Evaluate customer support agents",
  description:
    "Evaluate customer support agents on ticket triage, tool use, and reply quality. Grade classify, gather, draft, and escalate on real ticket history.",
  alternates: { canonical: PATH },
  openGraph: {
    title: "Evaluate customer support agents — AgentClash",
    description:
      "Race support agents on real ticket replays. Score intent classification, tool use, reply quality, and escalation discipline.",
    url: PATH,
  },
};

const FEATURES = [
  {
    label: "Intent",
    title: "Classify before you reply.",
    body: "Agents that skip classification and dive into a reply end up answering the wrong ticket. We grade intent tagging against the labeled ground truth from your resolved tickets.",
  },
  {
    label: "CRM gather",
    title: "Read the account first.",
    body: "Plan, tenure, open balance, last five tickets — good agents pull context before drafting. Trajectory signatures catch the agents that reply from a cold start.",
  },
  {
    label: "KB grounding",
    title: "Cite the article, not a vibe.",
    body: "Policy-bound replies must reference a real KB article. Cited articles that don't exist, or don't say what the agent claims, fail the verdict.",
  },
  {
    label: "Escalation",
    title: "Escalate when the rule says.",
    body: "Churn risk, legal flag, VIP tier — these trigger human routing. We score whether the agent escalated when the rules said so, not just whether the final reply looked nice.",
  },
  {
    label: "Tone",
    title: "Empathy without theatrics.",
    body: "Tone scoring is calibrated against your brand's resolved replies, not a generic helpfulness judge. Agents that over-apologize or over-explain get dinged.",
  },
  {
    label: "Replay",
    title: "Every ticket, every tool call.",
    body: "Scrub through tool invocations, KB lookups, draft revisions, and the final reply against the actual resolution your team shipped.",
  },
];

const FAQ_ITEMS = [
  {
    question: "How do you evaluate support agents without real customers?",
    answer:
      "AgentClash replays your historical ticket archive. Each case is seeded with the original ticket, the account state at the time, and the known-good resolution (either the reply your team sent or an annotated ideal). The agent gets the same tools your humans use — CRM, KB, billing, escalate — against a sandboxed clone.",
  },
  {
    question: "What tools do support agents get?",
    answer:
      "crm for account lookup, kb for knowledge base search, billing for account and charge inspection, and escalate for routing to human tiers. The tool policy is defined per pack so you can test restricted agent personas (e.g. a tier-1 agent without billing access).",
  },
  {
    question: "How do you score reply quality?",
    answer:
      "Replies are compared against the shipped resolution on factual correctness, policy compliance, and tone. For open-ended replies we use calibrated trajectory and content signatures; for policy-bound replies we check exact KB citations and escalation rules. The verdict is explainable: you see exactly which sub-score moved.",
  },
  {
    question: "Can I test escalation rules?",
    answer:
      "Yes. Packs can declare escalation triggers (VIP tier, open balance > N, churn signal, legal flag) and the verdict fails any reply that should have escalated but didn't. This catches a class of silent failures where the agent writes a polite reply to a customer who needed a human 10 minutes ago.",
  },
  {
    question: "Does this work with Zendesk, Intercom, Front?",
    answer:
      "Tool adapters are pluggable. The CRM tool is a thin interface; point it at a Zendesk export or an Intercom snapshot and the sandbox serves the same ticket and account state the live agent would have seen.",
  },
];

export default function SupportAgentsPage() {
  return (
    <>
      <JsonLd
        id="ld-support-product"
        data={productSchema({
          name: "AgentClash — customer support agent evaluation",
          description:
            "Grade support agents on intent classification, tool use, reply quality, and escalation discipline against real ticket replays.",
          url: PATH,
        })}
      />
      <JsonLd
        id="ld-support-breadcrumbs"
        data={breadcrumbSchema([
          { name: "Home", url: "/v2" },
          { name: "Use cases", url: "/v2/use-cases/coding-agents" },
          { name: "Support agents", url: PATH },
        ])}
      />
      <MarketingShell>
        <PageHeader
          breadcrumbs={[
            { label: "Home", href: "/v2" },
            { label: "Use cases" },
            { label: "Support agents" },
          ]}
          eyebrow="Support agent evaluation"
          title={
            <>
              Replay tickets.
              <br />
              <span className="text-white/40">Score resolutions.</span>
            </>
          }
          subtitle={
            <>
              A support agent that writes a polite reply to a churn-risk
              customer is still a bad agent. AgentClash replays your
              ticket history against candidate agents and scores the
              parts that actually matter — classify intent, gather
              context, draft the right reply, escalate when the rule
              says so.
            </>
          }
          cta={
            <div className="flex flex-col sm:flex-row sm:flex-wrap sm:items-center gap-3">
              <DemoButton />
              <Link
                href="/v2/platform/multi-turn-evaluation"
                className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
              >
                Multi-turn evaluation
                <ArrowRight className="size-4" />
              </Link>
            </div>
          }
          aside={
            <CodeCard
              title="A ticket replay"
              code={`# packs/ticket-replay-2026-q1.yaml
ticket: t_91a2...
snapshot: crm://zendesk/2026-03-14T11:02Z
tools: [crm, kb, billing, escalate]
required_behaviors:
  - classify_intent: billing_dispute
  - gather: [account.plan, account.balance]
  - cite_kb: ["refund-eligibility"]
  - escalate_if: balance > $500`}
            />
          }
        />

        <SplitSection
          eyebrow="Classify, then reply"
          title={
            <>
              Most bad replies come
              <br />
              <span className="text-white/40">from skipped classification.</span>
            </>
          }
          body={
            <>
              <p>
                The modal failure in support agent trajectories:
                <code className="mx-1 rounded bg-white/[0.06] px-1.5 py-0.5 font-[family-name:var(--font-mono)] text-[13px] text-white/80">draft_reply</code>
                is the first tool call. No intent tag, no account pull,
                no KB lookup — just an apology and a generic next step.
                The reply is fine English and wrong English.
              </p>
              <p className="mt-4">
                Trajectory signatures grade the prep work explicitly. If
                the pack says a billing dispute requires a balance check
                and a refund-eligibility KB lookup, the verdict fails
                replies that shipped without them — even if the prose
                reads great.
              </p>
            </>
          }
          aside={
            <CodeCard
              title="Trajectory — classify vs wing it"
              code={`# agent_a.trajectory
  1 tool: classify_intent → "billing_dispute"
  2 tool: crm.get(account) → tier=pro, bal=$612
  3 tool: kb.search("refund eligibility")
  4 tool: kb.read("refund-eligibility")
  5 tool: escalate(tier=2, reason=balance>500)
  6 tool: draft_reply(cite=["refund-eligibility"])
  verdict: ✓ 9.0 / 10

# agent_b.trajectory
  1 tool: draft_reply("So sorry to hear this...")
  verdict: ✗ 3.1 / 10   no classify, no escalate,
          cited KB article not fetched`}
            />
          }
        />

        <SplitSection
          reverse
          eyebrow="Escalation discipline"
          title={
            <>
              Polite isn&apos;t always right.
              <br />
              <span className="text-white/40">Some tickets need a human.</span>
            </>
          }
          body={
            <>
              <p>
                Escalation rules exist because some tickets can&apos;t be
                resolved by an agent — VIP tier, legal exposure, churn
                signal, balance over threshold. AgentClash checks
                whether the agent escalated when the rules said so, and
                the verdict fails replies that resolved tickets that
                should have been routed.
              </p>
            </>
          }
          aside={
            <CodeCard
              title="Verdict excerpt"
              code={`$ agentclash run show run_01H... --verdict

  intent_classification:    ✓ 1.0
  context_gathering:        ✓ 0.92
  kb_citation_fidelity:     ✓ 1.0
  escalation_discipline:    ✗ 0.20  ← should have routed
    reason: ticket had churn_flag=high
            but agent resolved without escalate()
  tone_calibration:         ✓ 0.87

  overall: 6.4 / 10   flagged for review`}
            />
          }
        />

        <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
          <div className="mx-auto max-w-[1440px]">
            <div className="max-w-[52ch]">
              <p className="mb-6 inline-flex items-center gap-2 text-[11px] font-[family-name:var(--font-mono)] uppercase tracking-[0.22em] text-white/45">
                <span className="inline-block size-1 rounded-full bg-white/60" />
                Capabilities
              </p>
              <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2rem,4.5vw,3.75rem)] max-w-[22ch]">
                Evaluate support agents on the behaviors your best humans share.
              </h2>
            </div>
            <div className="mt-20">
              <FeatureGrid features={FEATURES} columns={3} />
            </div>
          </div>
        </section>

        <FAQBlock items={FAQ_ITEMS} schemaId="ld-support-faq" />

        <ClosingCTA
          title={
            <>
              Ship agents that know
              <br />
              <span className="text-white/40">when to hand off.</span>
            </>
          }
          body={
            <p>
              Let us replay a month of your hardest tickets against two
              candidate agents and show you intent, context, citation,
              and escalation side by side.
            </p>
          }
        >
          <div className="flex flex-col sm:flex-row sm:flex-wrap gap-3">
            <DemoButton
              className="inline-flex items-center justify-center gap-2 rounded-md bg-white px-7 py-3 text-sm font-medium text-[#060606] hover:bg-white/90 transition-colors"
            />
            <Link
              href="/v2/platform/regression-testing"
              className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-7 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
            >
              Regression testing
              <ArrowRight className="size-4" />
            </Link>
          </div>
        </ClosingCTA>
      </MarketingShell>
    </>
  );
}
