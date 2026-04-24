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

const PATH = "/v2/use-cases/sre-agents";

export const metadata: Metadata = {
  title: "Evaluate SRE and on-call agents",
  description:
    "Evaluate SRE and on-call agents on incident triage, log analysis, and runbook execution. Replay past incidents against candidate agents with AgentClash.",
  alternates: { canonical: PATH },
  openGraph: {
    title: "Evaluate SRE agents — AgentClash",
    description:
      "Replay past incidents against candidate on-call agents. Score log analysis, runbook traversal, mitigation verification.",
    url: PATH,
  },
};

const FEATURES = [
  {
    label: "Incident replay",
    title: "Past pages become tests.",
    body: "Freeze a real incident — the alert that fired, the logs, the metric window, the cluster state — and replay it against any agent. The verdict compares against what actually resolved it.",
  },
  {
    label: "Log traversal",
    title: "Score the grep.",
    body: "Agents that pattern-match on the first stack trace they see miss root causes. We grade log query shape — time windows, label filters, distinct services sampled — alongside the final diagnosis.",
  },
  {
    label: "Runbook discipline",
    title: "Follow the doc.",
    body: "Most incidents have a runbook; most bad on-calls skip it. Trajectory signatures verify step order, tool preconditions, and abort conditions from your runbook markdown.",
  },
  {
    label: "Mitigation verify",
    title: "Did the fix actually fix it?",
    body: "After the agent declares mitigation, the sandbox checks the SLO window for the metric that fired the alert. A rollback that silences the pager but tanks latency fails.",
  },
  {
    label: "Blast radius",
    title: "Scoped tools, scoped risk.",
    body: "kube, ssh, logs, metrics — each with its own allowlist. A candidate agent can triage without being able to delete a namespace. Policy violations are hard verdict failures.",
  },
  {
    label: "Postmortem",
    title: "Write the note.",
    body: "After mitigation, grade the postmortem note the agent produces: timeline, root cause, blast radius, action items. Calibrated against your team's best postmortems.",
  },
];

const FAQ_ITEMS = [
  {
    question: "How does AgentClash replay a past incident?",
    answer:
      "We freeze the inputs that an on-call would have seen at the moment the page fired: the alert payload, a time-bounded snapshot of logs and metrics, the cluster state (kubectl get / describe), and any relevant runbook links. The agent gets a sandbox with read access to those sources and whatever mitigation tools your policy allows.",
  },
  {
    question: "What tools do SRE agents get?",
    answer:
      "logs (Loki-style query), metrics (Prom-style range query), kube (read + scoped write per policy), ssh (scoped per host pool), and runbook (structured markdown). Each tool has a per-pack allowlist — a triage-only pack can ban every write tool in one config line.",
  },
  {
    question: "How do you verify mitigation actually worked?",
    answer:
      "Packs declare the SLO metric the alert was protecting. After the agent declares mitigation, the sandbox inspects the metric over the follow-up window. An action that silenced the alert but violated the real SLO fails the verdict — this catches cosmetic fixes.",
  },
  {
    question: "Can agents execute destructive commands?",
    answer:
      "Only if the pack policy allows it, and only against the sandboxed cluster. Production credentials never enter the sandbox. For restricted modes, the entire kube write surface can be disabled — the agent can diagnose but not touch.",
  },
  {
    question: "Does this integrate with my runbook repo?",
    answer:
      "Yes. Point the runbook tool at a directory of markdown files (or a Notion / Confluence export). Runbooks are first-class pack inputs — the verdict can require that the agent opened the right one before acting.",
  },
];

export default function SreAgentsPage() {
  return (
    <>
      <JsonLd
        id="ld-sre-product"
        data={productSchema({
          name: "AgentClash — SRE and on-call agent evaluation",
          description:
            "Replay past incidents against candidate on-call agents. Grade log analysis, runbook traversal, and mitigation verification.",
          url: PATH,
        })}
      />
      <JsonLd
        id="ld-sre-breadcrumbs"
        data={breadcrumbSchema([
          { name: "Home", url: "/v2" },
          { name: "Use cases", url: "/v2/use-cases/coding-agents" },
          { name: "SRE agents", url: PATH },
        ])}
      />
      <MarketingShell>
        <PageHeader
          breadcrumbs={[
            { label: "Home", href: "/v2" },
            { label: "Use cases" },
            { label: "SRE agents" },
          ]}
          eyebrow="SRE and on-call agents"
          title={
            <>
              Replay the page.
              <br />
              <span className="text-white/40">Grade the on-call.</span>
            </>
          }
          subtitle={
            <>
              Your best on-call reads the runbook, diffs the last deploy,
              and verifies the SLO before going back to bed. AgentClash
              replays past incidents against candidate agents with the
              same logs, metrics, and cluster state, then scores the
              parts that separate a triage from a rollback.
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
              title="An incident replay"
              code={`# packs/incident-2026-03-12-p99.yaml
alert: HighCheckoutP99@v2.checkout
snapshot:
  logs:    loki://snap/2026-03-12T14:00Z/+30m
  metrics: prom://snap/2026-03-12T14:00Z/+30m
  kube:    kube://snap/prod-us-east-1
runbook: runbooks/checkout-p99.md
tools: [logs, metrics, kube, ssh]
slo: checkout_p99_ms < 450
expected_signature:
  - opened_runbook: ["checkout-p99.md"]
  - diff_incident_deploy: true
  - verify_slo_window: 15m`}
            />
          }
        />

        <SplitSection
          eyebrow="Diff the incident"
          title={
            <>
              Most P1s are caused
              <br />
              <span className="text-white/40">by the last deploy.</span>
            </>
          }
          body={
            <>
              <p>
                A good on-call looks at the deploy pipeline before they
                start reading stack traces. A bad on-call scrolls logs
                for 40 minutes and reboots the wrong pod. AgentClash
                tracks whether the agent correlated the alert window
                against recent deploys, config changes, and feature-flag
                flips — before it started hunting.
              </p>
              <p className="mt-4">
                Trajectory signatures can require specific early moves:
                open the runbook, pull the last 2 hours of deploys,
                check saturation metrics. Agents that skip straight to
                <code className="mx-1 rounded bg-white/[0.06] px-1.5 py-0.5 font-[family-name:var(--font-mono)] text-[13px] text-white/80">kubectl delete pod</code>
                have their verdicts docked before the outcome is even
                known.
              </p>
            </>
          }
          aside={
            <CodeCard
              title="Trajectory — triage vs flail"
              code={`# agent_a.trajectory
  1 tool: runbook.open("checkout-p99.md")
  2 tool: kube.deploys(svc=checkout, last=2h)
     → release r_92a (10m before alert)
  3 tool: metrics(checkout_p99, window=45m)
  4 tool: logs(svc=checkout, level=error, 20m)
     → N+1 on ORDER_ITEMS after r_92a
  5 tool: kube.rollback(release=r_91f)
  6 verify: p99 < 450 for 15m → ✓
  verdict: ✓ 9.5 / 10   MTTR 7m42s

# agent_b.trajectory
  1 tool: kube.delete pod checkout-7d...
  2 tool: kube.delete pod checkout-7d...
  3 (looped for 18m, alert re-fired)
  verdict: ✗ 2.0 / 10   no runbook, no rollback`}
            />
          }
        />

        <SplitSection
          reverse
          eyebrow="Verify the SLO"
          title={
            <>
              Silencing the pager
              <br />
              <span className="text-white/40">isn&apos;t the same as mitigation.</span>
            </>
          }
          body={
            <>
              <p>
                Agents that reduce alert sensitivity, redirect traffic
                away from the symptom, or roll back a healthy canary can
                all make the page go away without fixing anything. Packs
                declare the SLO the alert was protecting, and the
                verdict inspects the metric window after mitigation. A
                quiet pager with a broken SLO fails the run.
              </p>
            </>
          }
          aside={
            <CodeCard
              title="Verdict excerpt"
              code={`$ agentclash run show run_01H... --verdict

  opened_runbook:           ✓ checkout-p99.md
  diff_last_deploy:         ✓ r_92a flagged
  log_query_shape:          ✓ 0.91
  mitigation_action:        ✓ rollback r_91f
  slo_verified_15m:         ✓ p99 held < 450
  postmortem_note_quality:  ✓ 0.88

  overall: 9.3 / 10   candidate promoted to regression suite`}
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
                Evaluate on-call agents against the incidents you already survived.
              </h2>
            </div>
            <div className="mt-20">
              <FeatureGrid features={FEATURES} columns={3} />
            </div>
          </div>
        </section>

        <FAQBlock items={FAQ_ITEMS} schemaId="ld-sre-faq" />

        <ClosingCTA
          title={
            <>
              Ship the on-call
              <br />
              <span className="text-white/40">that opens the runbook.</span>
            </>
          }
          body={
            <p>
              Let us replay your last three P1s against two candidate
              agents and show you runbook discipline, mitigation shape,
              and SLO verification side by side.
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
