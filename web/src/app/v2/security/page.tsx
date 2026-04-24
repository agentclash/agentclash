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

const PATH = "/v2/security";

export const metadata: Metadata = {
  title: "Security posture — AgentClash",
  description:
    "Sandbox isolation, explicit data flow, workspace-scoped secrets, and append-only audit. How AgentClash handles your prompts, provider keys, and replay archives.",
  alternates: { canonical: PATH },
  openGraph: {
    title: "AgentClash security posture",
    description:
      "Sandbox isolation, explicit data flow, workspace-scoped secrets, and append-only audit trails.",
    url: PATH,
  },
};

const FEATURES = [
  {
    label: "Isolation",
    title: "Per-run E2B microVMs.",
    body: "Every agent runs inside an ephemeral E2B sandbox. Filesystem, process table, and network namespace are torn down when the race ends. No state leaks between runs, agents, or workspaces.",
  },
  {
    label: "Secrets",
    title: "Workspace-scoped, never logged.",
    body: "Provider keys and sandbox secrets are encrypted at rest, scoped to a single workspace, and redacted from every event before it hits the replay archive. They're readable inside the sandbox and nowhere else.",
  },
  {
    label: "AuthN / AuthZ",
    title: "WorkOS AuthKit, RBAC, SSO.",
    body: "Real org / workspace / role model backed by WorkOS AuthKit. Roles gate challenge packs, runs, and secrets at the manager layer — authorization is data-aware, not a middleware afterthought. SSO on request.",
  },
  {
    label: "Provider calls",
    title: "Straight from sandbox to provider.",
    body: "Model traffic exits the sandbox with the keys you supplied and goes directly to the provider endpoint. We don't proxy your prompts, and we don't route completions through a shared service.",
  },
  {
    label: "Network policy",
    title: "Default deny, allowlist egress.",
    body: "Sandboxes start with no network access. Each challenge pack declares the exact set of hosts its tools and provider calls are allowed to reach. Everything else is blocked at the sandbox boundary.",
  },
  {
    label: "Audit",
    title: "Append-only, sequence-numbered.",
    body: "Run events are immutable envelopes with a database-assigned sequence number. You can replay the exact ordering of tool calls, model outputs, and state transitions months later without fear of retroactive edits.",
  },
];

const FAQ_ITEMS = [
  {
    question: "Is AgentClash SOC 2 compliant?",
    answer:
      "SOC 2 Type II is being pursued during private beta. Design partners get the attestation path as it lands — controls, vendor inventory, and the independent audit timeline. If you need SOC 2 before you can onboard, tell us on the first call and we'll be honest about where we are.",
  },
  {
    question: "Where does my provider traffic go?",
    answer:
      "Directly from the E2B sandbox to the model provider, using the API keys you supply. We don't proxy the call, we don't see the prompt or completion outside the replay archive you control, and we don't store provider credentials outside the encrypted workspace vault.",
  },
  {
    question: "Can I self-host for zero-egress?",
    answer:
      "Yes. The open-source engine is the same code that runs our managed cloud — same API, same CLI, same scoring pipeline. Deploy it inside your VPC, point the sandbox provider at your own E2B tenant, and nothing leaves your environment. The /v2/oss page has the quickstart.",
  },
  {
    question: "What do you log?",
    answer:
      "Run metadata: timestamps, sequence numbers, verdicts, token counts, tool call names. The full payloads (prompt contents, tool outputs, model completions) live in the replay archive you control — managed cloud customers get signed-URL access and retention knobs per workspace. Application logs never contain secrets or prompt bodies.",
  },
  {
    question: "How are secrets stored?",
    answer:
      "Encrypted at rest with per-workspace keys, injected into the sandbox environment at boot, and wiped when the sandbox tears down. They never appear in run events, replay archives, or application logs — the event serializer redacts any value registered as a secret before it's persisted.",
  },
];

export default function SecurityPage() {
  return (
    <>
      <JsonLd
        id="ld-security-product"
        data={productSchema({
          name: "AgentClash — security posture",
          description:
            "AI agent evaluation platform with sandbox isolation, workspace-scoped secrets, WorkOS-backed auth, and append-only audit.",
          url: PATH,
        })}
      />
      <JsonLd
        id="ld-security-breadcrumbs"
        data={breadcrumbSchema([
          { name: "Home", url: "/v2" },
          { name: "Security", url: PATH },
        ])}
      />
      <MarketingShell>
        <PageHeader
          breadcrumbs={[
            { label: "Home", href: "/v2" },
            { label: "Security" },
          ]}
          eyebrow="Security posture"
          title={
            <>
              Sandboxes, secrets,
              <br />
              <span className="text-white/40">and straight answers.</span>
            </>
          }
          subtitle={
            <>
              AgentClash is built around sandbox isolation and explicit data
              flow. Everything we run for you on managed cloud, and
              everything you can run yourself on self-host, honors the same
              model — the same boundaries, the same redaction, the same
              append-only audit.
            </>
          }
          cta={
            <div className="flex flex-col sm:flex-row sm:flex-wrap sm:items-center gap-3">
              <DemoButton label="Talk to us" />
              <Link
                href="/v2/oss"
                className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
              >
                Self-host for zero egress
                <ArrowRight className="size-4" />
              </Link>
            </div>
          }
          aside={
            <CodeCard
              title="Trust surface"
              code={`isolation:       E2B microVM per run
secrets:         workspace-scoped, redacted
auth:            WorkOS AuthKit, RBAC, SSO
network:         default deny + allowlist
audit:           append-only, sequenced
residency:       self-host = your VPC`}
            />
          }
        />

        <SplitSection
          eyebrow="Data flow, in writing"
          title={
            <>
              Every hop is
              <br />
              <span className="text-white/40">on the map.</span>
            </>
          }
          body={
            <>
              <p>
                There&apos;s no hidden tier of infrastructure between your
                sandbox and the model provider. A run has a fixed, short
                list of components, and you can name each one — what it
                does, what it sees, and what it stores.
              </p>
              <p className="mt-4">
                The CLI submits to the API server. The API server submits a
                Temporal workflow. The workflow activity provisions a
                sandbox. The sandbox calls the provider with your keys.
                Events are persisted, in order, to the replay archive you
                control. That&apos;s the whole diagram.
              </p>
            </>
          }
          aside={
            <CodeCard
              title="Request path"
              code={`[ you ]
   │  agentclash run create
   ▼
[ CLI ] ──── HTTPS ────┐
                       ▼
             [ API server ]
                  │ submit workflow
                  ▼
             [ Temporal ]
                  │ activity
                  ▼
             [ Sandbox (E2B) ]
                  │ your provider keys
                  ▼
             [ Model provider ]

 events ──▶ [ Replay archive ]`}
            />
          }
        />

        <SplitSection
          reverse
          eyebrow="What leaves your environment"
          title={
            <>
              Explicit about what we see.
              <br />
              <span className="text-white/40">And what we don&apos;t.</span>
            </>
          }
          body={
            <>
              <p>
                On managed cloud we see run metadata — timestamps, token
                counts, verdict scores, tool call names — because that&apos;s
                what makes the race engine work. We do not see prompt
                bodies, tool outputs, or model completions outside the
                replay archive attached to your workspace.
              </p>
              <p className="mt-4">
                On self-host, we see nothing. The whole system lives inside
                your network. The same boundaries still apply internally —
                redaction, sandbox isolation, immutable audit — they just
                run on your iron.
              </p>
            </>
          }
          aside={
            <CodeCard
              title="Managed cloud visibility"
              code={`we see (operational):
  - run id, workspace id, actor
  - start/end, status, verdict
  - token counts, cost, latency
  - tool names, sequence numbers

we do not see:
  - prompt bodies
  - tool call arguments / outputs
  - model completions
  - secrets, provider keys

retention knobs:
  replay archive:  per workspace
  metadata:        per workspace
  secrets:         wiped on tear-down`}
            />
          }
        />

        <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
          <div className="mx-auto max-w-[1440px]">
            <div className="max-w-[52ch]">
              <p className="mb-6 inline-flex items-center gap-2 text-[11px] font-[family-name:var(--font-mono)] uppercase tracking-[0.22em] text-white/45">
                <span className="inline-block size-1 rounded-full bg-white/60" />
                Controls
              </p>
              <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2rem,4.5vw,3.75rem)] max-w-[22ch]">
                Six boundaries that hold the shape.
              </h2>
            </div>
            <div className="mt-20">
              <FeatureGrid features={FEATURES} columns={3} />
            </div>
          </div>
        </section>

        <FAQBlock items={FAQ_ITEMS} schemaId="ld-security-faq" />

        <ClosingCTA
          title={
            <>
              Questions we haven&apos;t answered?
              <br />
              <span className="text-white/40">Put them on the call.</span>
            </>
          }
          body={
            <p>
              Security reviews are a conversation, not a checkbox. Bring
              your vendor questionnaire, your threat model, your
              compliance counsel — we&apos;ll walk the diagram with you.
            </p>
          }
        >
          <div className="flex flex-col sm:flex-row sm:flex-wrap gap-3">
            <DemoButton
              label="Book a security call"
              className="inline-flex items-center justify-center gap-2 rounded-md bg-white px-7 py-3 text-sm font-medium text-[#060606] hover:bg-white/90 transition-colors"
            />
            <Link
              href="/v2/oss"
              className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-7 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
            >
              Self-host quickstart
              <ArrowRight className="size-4" />
            </Link>
          </div>
        </ClosingCTA>
      </MarketingShell>
    </>
  );
}
