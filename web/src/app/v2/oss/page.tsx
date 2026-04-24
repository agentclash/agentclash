import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight, ExternalLink, Star } from "lucide-react";
import { MarketingShell } from "@/components/marketing/marketing-shell";
import { PageHeader } from "@/components/marketing/page-header";
import { SplitSection } from "@/components/marketing/split-section";
import { FeatureGrid } from "@/components/marketing/feature-grid";
import { ClosingCTA } from "@/components/marketing/closing-cta";
import { CodeCard } from "@/components/marketing/code-card";
import { DemoButton } from "@/components/marketing/demo-button";
import { FAQBlock } from "@/components/marketing/faq-block";
import { JsonLd, breadcrumbSchema, productSchema } from "@/components/marketing/json-ld";

const PATH = "/v2/oss";

export const metadata: Metadata = {
  title: "Open-source agent evaluation — self-host AgentClash",
  description:
    "AgentClash is open source under FSL-1.1-MIT. Install the CLI in seconds, self-host the race engine on your own infra, and read every line of the scoring pipeline. No vendor lock-in.",
  alternates: { canonical: PATH },
  openGraph: {
    title: "Open-source agent evaluation — self-host AgentClash",
    description:
      "Install the CLI, self-host the race engine, read the scoring pipeline. AgentClash is open source under FSL-1.1-MIT.",
    url: PATH,
  },
};

const FEATURES = [
  {
    label: "Self-host",
    title: "Run the whole race engine on your infra.",
    body: "Go API server, Temporal worker, Postgres, optional E2B sandboxes. One docker-compose, or deploy as you'd deploy any Go service. No phone-home, no feature flags locked behind cloud.",
  },
  {
    label: "CLI",
    title: "agentclash — one binary, every channel.",
    body: "Ships via npm, Homebrew, Winget, and POSIX/PowerShell install scripts from a single GoReleaser build. Authenticate, trigger races, stream live logs, gate CI.",
  },
  {
    label: "FSL-1.1-MIT",
    title: "Open source with a two-year clock.",
    body: "Read the code, fork it, patch it, ship it. FSL becomes MIT two years after each release — durable open source without racing to the bottom on competing managed offerings.",
  },
  {
    label: "Providers",
    title: "Swap any model, bring any sandbox.",
    body: "First-class adapters for OpenAI, Anthropic, Gemini, xAI, Mistral, and OpenRouter's long tail. Sandbox is an interface — E2B today, plug your own tomorrow.",
  },
  {
    label: "Docs",
    title: "Architecture notes that aren't marketing.",
    body: "The docs ship with a full architecture overview: two-plane split, Temporal workflow hierarchy, event model, provider router, scoring pipeline. Written for people who read code.",
  },
  {
    label: "AI-friendly",
    title: "llms.txt out of the box.",
    body: "Every doc page has a markdown mirror. llms.txt and llms-full.txt expose the full corpus so coding agents can answer real integration questions without hallucinating.",
  },
];

const INSTALL_SNIPPETS: Array<{ title: string; language: string; code: string }> = [
  {
    title: "npm",
    language: "shell",
    code: "npm i -g agentclash\nagentclash auth login --device\nagentclash run create --follow",
  },
  {
    title: "Homebrew",
    language: "shell",
    code: "brew install agentclash/tap/agentclash\nagentclash auth login --device\nagentclash run create --follow",
  },
  {
    title: "Self-host",
    language: "shell",
    code: "git clone https://github.com/agentclash/agentclash\ncd agentclash\n./scripts/dev/start-local-stack.sh",
  },
];

const FAQ_ITEMS = [
  {
    question: "Is AgentClash actually open source?",
    answer:
      "Yes. The entire race engine — API server, Temporal worker, CLI, provider router, sandbox abstraction, scoring pipeline, and frontend — is public on GitHub under FSL-1.1-MIT, which converts to MIT two years after each release.",
  },
  {
    question: "Do I need the cloud to use AgentClash?",
    answer:
      "No. You can run AgentClash entirely on your own infrastructure using docker-compose or any Go-friendly deployment target. The managed cloud exists for teams who want a hosted control plane without operating Temporal and Postgres themselves.",
  },
  {
    question: "What does the CLI need to work?",
    answer:
      "An AgentClash API URL (your self-hosted server or the managed cloud) and an auth token. The CLI supports device-flow login, env vars for CI, and per-workspace configuration.",
  },
  {
    question: "Which providers are supported?",
    answer:
      "First-class adapters for OpenAI, Anthropic, Google Gemini, xAI, Mistral, and OpenRouter (which gives you the long tail of 300+ models). Adding a new provider means implementing a small Client interface.",
  },
  {
    question: "Why FSL-1.1-MIT instead of MIT or Apache-2.0?",
    answer:
      "FSL (Functional Source License) gives AgentClash maintainers two years of commercial protection before each release reverts to MIT. It's open source in every way that matters for users — read the code, run it, modify it — with guardrails against a competing vendor repackaging it verbatim.",
  },
  {
    question: "How do I contribute?",
    answer:
      "Start with the Codebase Tour in the docs, open an issue describing your change, and submit a PR. The monorepo has a Go backend (API + Temporal worker), a Go CLI, and a Next.js frontend. Every CI run is public.",
  },
];

export default function OSSPage() {
  return (
    <>
      <JsonLd
        id="ld-oss-product"
        data={productSchema({
          name: "AgentClash (open source)",
          description:
            "Open-source AI agent evaluation platform. Self-host the full race engine, or install the CLI and connect to the managed cloud.",
          url: PATH,
        })}
      />
      <JsonLd
        id="ld-oss-breadcrumbs"
        data={breadcrumbSchema([
          { name: "Home", url: "/v2" },
          { name: "Open source", url: PATH },
        ])}
      />
      <MarketingShell>
        <PageHeader
          breadcrumbs={[
            { label: "Home", href: "/v2" },
            { label: "Open source" },
          ]}
          eyebrow="Open source · FSL-1.1-MIT"
          title={
            <>
              An eval engine you can actually{" "}
              <span className="text-white/40">audit.</span>
            </>
          }
          subtitle={
            <>
              AgentClash is open-source AI agent evaluation infrastructure.
              Star the repo, run the whole stack on your infra, or install
              the CLI in seconds and connect to the managed cloud. No
              vendor lock-in. No black-box scoring.
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
              title="Install"
              code={`# Install the CLI
npm i -g agentclash

# Authenticate against a workspace
agentclash auth login --device

# Fire a race
agentclash run create --follow`}
            />
          }
        />

        {/* Install paths */}
        <section className="border-t border-white/[0.06] px-8 sm:px-12 py-28 sm:py-40">
          <div className="mx-auto max-w-[1440px]">
            <div className="max-w-[52ch]">
              <p className="mb-6 inline-flex items-center gap-2 text-[11px] font-[family-name:var(--font-mono)] uppercase tracking-[0.22em] text-white/45">
                <span className="inline-block size-1 rounded-full bg-white/60" />
                Install
              </p>
              <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2rem,4.5vw,3.75rem)] max-w-[22ch]">
                One binary. Every channel.
              </h2>
              <p className="mt-8 text-lg leading-[1.6] text-white/55">
                A single GoReleaser build feeds npm, Homebrew, Winget, and
                POSIX/PowerShell install scripts — pick whichever matches
                your runner.
              </p>
            </div>

            <div className="mt-16 grid gap-6 md:grid-cols-3">
              {INSTALL_SNIPPETS.map((snip) => (
                <CodeCard
                  key={snip.title}
                  title={snip.title}
                  language={snip.language}
                  code={snip.code}
                />
              ))}
            </div>
          </div>
        </section>

        {/* Self-host vs managed */}
        <SplitSection
          eyebrow="Self-host or managed"
          title={
            <>
              Bring your own infra. Or skip
              <br />
              <span className="text-white/40">that part entirely.</span>
            </>
          }
          body={
            <>
              <p>
                Self-host when you want the scoring pipeline audited in
                your own environment, when your challenge packs touch
                sensitive data, or when you just enjoy running Temporal
                clusters.
              </p>
              <p className="mt-4">
                Use the managed cloud when you&apos;d rather focus on the
                challenges themselves and let us run the durable
                orchestration, sandbox provisioning, and replay archive.
              </p>
              <div className="mt-8 flex flex-col sm:flex-row gap-3">
                <Link
                  href="/v2/cloud"
                  className="inline-flex items-center gap-2 text-[14px] text-white/80 hover:text-white border-b border-white/25 hover:border-white/60 transition-colors pb-1 w-fit"
                >
                  Managed cloud <ArrowRight className="size-3.5" />
                </Link>
                <Link
                  href="/docs/architecture/overview"
                  className="inline-flex items-center gap-2 text-[14px] text-white/60 hover:text-white/90 border-b border-white/15 hover:border-white/40 transition-colors pb-1 w-fit"
                >
                  Architecture overview <ArrowRight className="size-3.5" />
                </Link>
              </div>
            </>
          }
          aside={
            <CodeCard
              title="docker-compose up"
              code={`# Clone and boot the full stack
git clone https://github.com/agentclash/agentclash
cd agentclash

# Postgres, Temporal, API server, worker
./scripts/dev/start-local-stack.sh

# Health check
curl http://localhost:8080/healthz`}
            />
          }
        />

        {/* What's in the box */}
        <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
          <div className="mx-auto max-w-[1440px]">
            <div className="max-w-[52ch]">
              <p className="mb-6 inline-flex items-center gap-2 text-[11px] font-[family-name:var(--font-mono)] uppercase tracking-[0.22em] text-white/45">
                <span className="inline-block size-1 rounded-full bg-white/60" />
                What ships
              </p>
              <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2rem,4.5vw,3.75rem)] max-w-[22ch]">
                Six pieces of real infrastructure.
              </h2>
            </div>
            <div className="mt-20">
              <FeatureGrid features={FEATURES} columns={3} />
            </div>
          </div>
        </section>

        <FAQBlock
          eyebrow="FAQ"
          title="Questions people actually ask."
          items={FAQ_ITEMS}
          schemaId="ld-oss-faq"
        />

        <ClosingCTA
          title={
            <>
              Read the source.
              <br />
              <span className="text-white/40">Run the race.</span>
            </>
          }
          body={
            <p>
              Star on GitHub to follow along, or fork it and race your own
              models tonight.
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
              href="/docs/getting-started/self-host"
              className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-7 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
            >
              Self-host quickstart
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
