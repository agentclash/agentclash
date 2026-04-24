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

const PATH = "/v2/cloud";

export const metadata: Metadata = {
  title: "Managed agent evaluation — AgentClash Cloud (beta)",
  description:
    "Let us run the race engine. Durable orchestration, sandbox provisioning, replay archive, and hosted workspaces — so your team focuses on challenge packs and verdicts, not Temporal clusters.",
  alternates: { canonical: PATH },
  openGraph: {
    title: "AgentClash Cloud — managed agent evaluation (beta)",
    description:
      "Hosted race engine with durable orchestration, sandbox provisioning, and replay archive. Private beta.",
    url: PATH,
  },
};

const FEATURES = [
  {
    label: "Orchestration",
    title: "Durable runs, no ops surface.",
    body: "Temporal-backed workflows survive node restarts, provider timeouts, and replays. You wire up challenge packs. We keep the graph alive.",
  },
  {
    label: "Sandboxes",
    title: "Ephemeral E2B environments, ready when you fire.",
    body: "Each agent gets an isolated sandbox with pinned tools, network policy, and secrets. No cold starts you manage. No leaked filesystem state between races.",
  },
  {
    label: "Replay archive",
    title: "Every trace, forever.",
    body: "Events, tool calls, model outputs, sandbox filesystems, stored and addressable by run ID. Replays never expire on your tier. Scrubbing a failure six months later just works.",
  },
  {
    label: "Workspaces",
    title: "Multi-tenant, WorkOS-backed.",
    body: "Real org/workspace/role model backed by WorkOS AuthKit. SSO on request. Role-based access on challenge packs, runs, and secrets.",
  },
  {
    label: "CI/CD",
    title: "Gate merges on live races.",
    body: "Drop the CLI into any CI. Block the PR when an agent regresses. Post verdict links to GitHub checks so reviewers see what failed without leaving the diff.",
  },
  {
    label: "Support",
    title: "Direct to the maintainers.",
    body: "Beta customers get a shared Slack channel with the team that wrote the scoring pipeline. Bugs get fixed where they live — not escalated through tiers.",
  },
];

const FAQ_ITEMS = [
  {
    question: "Is AgentClash Cloud generally available?",
    answer:
      "Not yet. We're onboarding a small group of design partners in private beta. Book a demo and we'll decide together whether it's a fit — if it is, you'll get a hosted workspace and direct access to the team.",
  },
  {
    question: "What do you host vs. what do I run?",
    answer:
      "We host the control plane (API, Temporal worker, Postgres, replay archive, sandbox provisioning). You bring the challenge packs, provider keys, and CI workflows. The CLI talks to the hosted API; nothing about the race engine has to live on your infra.",
  },
  {
    question: "Can I move between managed and self-hosted?",
    answer:
      "Yes. Everything you can do on managed cloud you can do self-hosted — same API, same CLI, same workflows. Export your runs and challenge packs whenever you want.",
  },
  {
    question: "Where does my provider traffic go?",
    answer:
      "Straight from the sandbox to the model provider using the credentials you supply. We don't proxy model calls, and we don't store prompts or completions outside the replay archive you control.",
  },
  {
    question: "What about security and compliance?",
    answer:
      "Sandboxes are isolated per-run. Auth is WorkOS AuthKit (SSO available on request). Data residency, retention, and audit logging details live on the /v2/security page.",
  },
  {
    question: "How is pricing handled during beta?",
    answer:
      "Design partners collaborate with us on scope and usage during beta. We're honest about the fact that pricing will exist — we're not baiting and switching — but we want to figure it out alongside the first teams that actually use it.",
  },
];

export default function CloudPage() {
  return (
    <>
      <JsonLd
        id="ld-cloud-product"
        data={productSchema({
          name: "AgentClash Cloud",
          description:
            "Managed AI agent evaluation platform. Hosted race engine with durable orchestration, sandbox provisioning, replay archive, and CI/CD integration.",
          url: PATH,
        })}
      />
      <JsonLd
        id="ld-cloud-breadcrumbs"
        data={breadcrumbSchema([
          { name: "Home", url: "/v2" },
          { name: "Managed cloud", url: PATH },
        ])}
      />
      <MarketingShell>
        <PageHeader
          breadcrumbs={[
            { label: "Home", href: "/v2" },
            { label: "Managed cloud" },
          ]}
          eyebrow="AgentClash Cloud · Private beta"
          title={
            <>
              The race engine.
              <br />
              <span className="text-white/40">Hosted for you.</span>
            </>
          }
          subtitle={
            <>
              We run Temporal, Postgres, sandbox provisioning, and the
              replay archive. You run the races. Private beta today —
              book a call if your team would benefit from hosted
              AI agent evaluation without operating durable infrastructure.
            </>
          }
          cta={
            <div className="flex flex-col sm:flex-row sm:flex-wrap sm:items-center gap-3">
              <DemoButton label="Book a demo" />
              <Link
                href="/v2/design-partners"
                className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
              >
                Design partner program
                <ArrowRight className="size-4" />
              </Link>
              <Link
                href="/v2/oss"
                className="inline-flex items-center justify-center gap-2 rounded-md border border-white/[0.08] bg-white/[0.02] px-6 py-3 text-sm font-medium text-white/60 hover:text-white/90 hover:border-white/20 transition-colors"
              >
                Prefer self-host?
                <ArrowRight className="size-4" />
              </Link>
            </div>
          }
          aside={
            <CodeCard
              title="Connect"
              code={`# Point the CLI at the hosted API
export AGENTCLASH_API_URL="https://api.agentclash.dev"

# Device-flow login
agentclash auth login --device

# Fire your first race
agentclash workspace use <your-workspace>
agentclash run create --follow`}
            />
          }
        />

        <SplitSection
          eyebrow="Why hosted"
          title={
            <>
              Temporal is not
              <br />
              <span className="text-white/40">your product.</span>
            </>
          }
          body={
            <>
              <p>
                Running durable workflow infrastructure is a real
                engineering commitment. Postgres tuning, Temporal
                upgrades, sandbox provisioning, replay storage — these
                are not the part of agent evaluation your team wants
                to own.
              </p>
              <p className="mt-4">
                Managed cloud gives you the exact same API, CLI, and
                scoring pipeline as the open-source engine. We operate
                the boring parts so your team stays on what matters:
                the challenge packs, the verdicts, and the regressions
                that shouldn&apos;t have shipped.
              </p>
            </>
          }
          aside={
            <div className="rounded-lg border border-white/[0.08] bg-white/[0.015] p-10">
              <p className="text-[11px] font-[family-name:var(--font-mono)] uppercase tracking-[0.2em] text-white/40">
                What we run for you
              </p>
              <ul className="mt-6 space-y-3 text-[14px] text-white/70">
                <li className="flex items-start gap-3">
                  <span className="mt-2 size-1 shrink-0 rounded-full bg-white/50" />
                  Temporal cluster + API server
                </li>
                <li className="flex items-start gap-3">
                  <span className="mt-2 size-1 shrink-0 rounded-full bg-white/50" />
                  Postgres with point-in-time recovery
                </li>
                <li className="flex items-start gap-3">
                  <span className="mt-2 size-1 shrink-0 rounded-full bg-white/50" />
                  Sandbox provisioning + lifecycle
                </li>
                <li className="flex items-start gap-3">
                  <span className="mt-2 size-1 shrink-0 rounded-full bg-white/50" />
                  Replay archive + event log
                </li>
                <li className="flex items-start gap-3">
                  <span className="mt-2 size-1 shrink-0 rounded-full bg-white/50" />
                  WorkOS auth + org/workspace model
                </li>
                <li className="flex items-start gap-3">
                  <span className="mt-2 size-1 shrink-0 rounded-full bg-white/50" />
                  Upgrades, patches, monitoring
                </li>
              </ul>
            </div>
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
                Six things the cloud quietly handles.
              </h2>
            </div>
            <div className="mt-20">
              <FeatureGrid features={FEATURES} columns={3} />
            </div>
          </div>
        </section>

        <FAQBlock items={FAQ_ITEMS} schemaId="ld-cloud-faq" />

        <ClosingCTA
          title={
            <>
              Private beta is open.
              <br />
              <span className="text-white/40">Come talk to us.</span>
            </>
          }
          body={
            <p>
              We spend the first call mapping your eval problem before we
              touch the cloud. If hosted isn&apos;t the right fit we&apos;ll
              point you at the self-host quickstart and call it a win.
            </p>
          }
        >
          <div className="flex flex-col sm:flex-row sm:flex-wrap gap-3">
            <DemoButton
              label="Book a demo"
              className="inline-flex items-center justify-center gap-2 rounded-md bg-white px-7 py-3 text-sm font-medium text-[#060606] hover:bg-white/90 transition-colors"
            />
            <Link
              href="/v2/design-partners"
              className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-7 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
            >
              Design partner program
              <ArrowRight className="size-4" />
            </Link>
          </div>
        </ClosingCTA>
      </MarketingShell>
    </>
  );
}
