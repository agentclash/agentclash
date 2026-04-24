import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight, Star, ExternalLink } from "lucide-react";
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

const PATH = "/v2/platform/self-hosted";

export const metadata: Metadata = {
  title: "Self-hosted agent evaluation",
  description:
    "Run AgentClash entirely on your own infra. Open-source under FSL-1.1-MIT. Bring your own Postgres, your own Temporal, your own sandbox provider — read every line of the scoring pipeline.",
  alternates: { canonical: PATH },
  openGraph: {
    title: "Self-hosted agent evaluation — AgentClash",
    description:
      "Run the full agent-eval race engine on your own infra. Open source. No phone-home. No vendor lock-in.",
    url: PATH,
  },
};

const FEATURES = [
  {
    label: "docker-compose",
    title: "One command to boot the stack.",
    body: "Postgres, Temporal, API server, worker, all wired up by ./scripts/dev/start-local-stack.sh. From fresh clone to first race in under five minutes.",
  },
  {
    label: "No phone-home",
    title: "Your data, your network, your rules.",
    body: "Nothing leaves your environment except the provider calls you authorize. No telemetry. No license server. No mandatory feature flags.",
  },
  {
    label: "Swap sandboxes",
    title: "E2B, Firecracker, bare VMs.",
    body: "Sandbox is a Go interface. The default provider is E2B, but you can plug in Firecracker microVMs, your own Kubernetes jobs, or anything else that runs isolated code.",
  },
  {
    label: "Own your data",
    title: "Postgres + S3 you already run.",
    body: "Event metadata lives in Postgres. Large replay payloads go to S3 or any S3-compatible blob store you point it at.",
  },
  {
    label: "Same CLI",
    title: "Managed parity, without the cloud.",
    body: "The CLI, the API, the challenge pack format — all identical. A self-host user and a managed-cloud user hit the same endpoints, just at different URLs.",
  },
  {
    label: "Auditable scoring",
    title: "Read every scorer.",
    body: "The scoring pipeline is plain Go, not an opaque SDK. Fork, patch, or replace any scorer. Audit exactly how every verdict was computed.",
  },
];

const FAQ_ITEMS = [
  {
    question: "What do I need to self-host AgentClash?",
    answer:
      "Go 1.25+, Docker, the Temporal CLI, and a Postgres instance. The included docker-compose stands all of that up locally in one command. For production you'd run Temporal, Postgres, and the API/worker services as you'd run any Go service — Kubernetes, Nomad, plain VMs all work.",
  },
  {
    question: "Can I bring my own sandbox provider?",
    answer:
      "Yes. The Sandbox interface has a small surface — provision, exec, filesystem I/O, teardown. The default implementation is E2B; a noop provider ships for environments where sandboxes are unconfigured. Your own provider can plug straight in.",
  },
  {
    question: "What about auth in self-host?",
    answer:
      "Production uses WorkOS AuthKit. Local dev uses AUTH_MODE=dev, which reads an X-Dev-User-ID header — no setup required. You can also wire your own SSO into the same middleware hooks.",
  },
  {
    question: "Is the scoring pipeline customizable?",
    answer:
      "Completely. Scorers are Go packages behind a small interface, and challenge packs reference them by name. Fork, patch, or write from scratch — nothing about the scoring is gated behind the managed product.",
  },
  {
    question: "What license is AgentClash under?",
    answer:
      "FSL-1.1-MIT — Functional Source License with a two-year MIT future. Read the code, run it, modify it, self-host for your company. Each release converts to plain MIT two years after it ships.",
  },
];

export default function SelfHostedPage() {
  return (
    <>
      <JsonLd
        id="ld-sh-product"
        data={productSchema({
          name: "AgentClash — self-hosted agent evaluation",
          description:
            "Self-host the open-source AgentClash race engine. FSL-1.1-MIT. Bring your own Postgres, Temporal, and sandbox provider.",
          url: PATH,
        })}
      />
      <JsonLd
        id="ld-sh-breadcrumbs"
        data={breadcrumbSchema([
          { name: "Home", url: "/v2" },
          { name: "Platform", url: "/v2/platform/agent-evaluation" },
          { name: "Self-hosted", url: PATH },
        ])}
      />
      <MarketingShell>
        <PageHeader
          breadcrumbs={[
            { label: "Home", href: "/v2" },
            { label: "Platform" },
            { label: "Self-hosted" },
          ]}
          eyebrow="Self-hosted · FSL-1.1-MIT"
          title={
            <>
              Your infra.
              <br />
              <span className="text-white/40">Your scoring pipeline.</span>
            </>
          }
          subtitle={
            <>
              When your agents touch sensitive data, or you just want
              the full scoring pipeline in reach of your debugger,
              self-host AgentClash end-to-end. Same API, same CLI, same
              challenge packs — no managed dependency.
            </>
          }
          cta={
            <div className="flex flex-col sm:flex-row sm:flex-wrap sm:items-center gap-3">
              <a
                href="https://github.com/agentclash/agentclash"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center justify-center gap-2 rounded-md bg-white px-6 py-3 text-sm font-medium text-[#060606] hover:bg-white/90 transition-colors"
              >
                <Star className="size-4" />
                Star on GitHub
                <ExternalLink className="size-3.5 text-black/40" />
              </a>
              <Link
                href="/docs/getting-started/self-host"
                className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
              >
                Self-host quickstart
                <ArrowRight className="size-4" />
              </Link>
            </div>
          }
          aside={
            <CodeCard
              title="start-local-stack.sh"
              code={`git clone https://github.com/agentclash/agentclash
cd agentclash
./scripts/dev/start-local-stack.sh

# Postgres   → localhost:5432
# Temporal   → localhost:7233
# API        → localhost:8080
# Worker     → connected`}
            />
          }
        />

        <SplitSection
          eyebrow="Why self-host"
          title={
            <>
              Some data shouldn&apos;t
              <br />
              <span className="text-white/40">leave your network.</span>
            </>
          }
          body={
            <>
              <p>
                Agents that run against production data, internal tools,
                or customer PII belong in your own environment. AgentClash
                self-host is the same race engine as the managed cloud —
                no feature gating, no telemetry phone-home.
              </p>
              <p className="mt-4">
                You provide Postgres, Temporal, and whatever sandbox
                provider matches your security model. The CLI points at
                your API URL and everything else behaves identically.
              </p>
            </>
          }
          aside={
            <CodeCard
              title="Point CLI at your server"
              code={`export AGENTCLASH_API_URL="https://eval.internal.corp"
agentclash auth login --device

# Everything else is the same
agentclash workspace list
agentclash run create --follow`}
            />
          }
        />

        <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
          <div className="mx-auto max-w-[1440px]">
            <div className="max-w-[52ch]">
              <p className="mb-6 inline-flex items-center gap-2 text-[11px] font-[family-name:var(--font-mono)] uppercase tracking-[0.22em] text-white/45">
                <span className="inline-block size-1 rounded-full bg-white/60" />
                Self-host capabilities
              </p>
              <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2rem,4.5vw,3.75rem)] max-w-[22ch]">
                Real infrastructure, not a demo.
              </h2>
            </div>
            <div className="mt-20">
              <FeatureGrid features={FEATURES} columns={3} />
            </div>
          </div>
        </section>

        <FAQBlock items={FAQ_ITEMS} schemaId="ld-sh-faq" />

        <ClosingCTA
          title={
            <>
              Read the source.
              <br />
              <span className="text-white/40">Run it tonight.</span>
            </>
          }
          body={
            <p>
              The quickstart puts a full AgentClash stack on your
              machine in about five minutes.
            </p>
          }
        >
          <div className="flex flex-col sm:flex-row sm:flex-wrap gap-3">
            <a
              href="https://github.com/agentclash/agentclash"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center justify-center gap-2 rounded-md bg-white px-7 py-3 text-sm font-medium text-[#060606] hover:bg-white/90 transition-colors"
            >
              <Star className="size-4" />
              Star on GitHub
              <ExternalLink className="size-3.5 text-black/40" />
            </a>
            <Link
              href="/docs/architecture/overview"
              className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-7 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
            >
              Architecture overview
              <ArrowRight className="size-4" />
            </Link>
            <DemoButton
              label="Talk to a maintainer"
              className="inline-flex items-center justify-center gap-2 rounded-md border border-white/[0.08] bg-white/[0.02] px-7 py-3 text-sm font-medium text-white/60 hover:text-white/90 hover:border-white/20 transition-colors"
            />
          </div>
        </ClosingCTA>
      </MarketingShell>
    </>
  );
}
