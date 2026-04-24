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

const PATH = "/v2/use-cases/coding-agents";

export const metadata: Metadata = {
  title: "Evaluate coding agents",
  description:
    "Benchmark autonomous dev agents head-to-head on real repos. Score read-file, exec-test, and open-PR behaviors against SWE-bench-style tasks with AgentClash.",
  alternates: { canonical: PATH },
  openGraph: {
    title: "Evaluate coding agents — AgentClash",
    description:
      "Race autonomous coding agents on real repos. Score behaviors that matter: read file, run tests, open PR.",
    url: PATH,
  },
};

const FEATURES = [
  {
    label: "Tools",
    title: "Real fs, exec, git, http.",
    body: "Agents get a sandboxed checkout with live filesystem, shell, and git. No mocked test harness — if the patch doesn't compile, the run fails the way it would in CI.",
  },
  {
    label: "Behaviors",
    title: "Score the plan, not the prose.",
    body: "Did the agent read the failing test before editing? Did it run the suite before opening the PR? Trajectory signatures grade the method, not just the final diff.",
  },
  {
    label: "Repos",
    title: "Your codebase, not a toy.",
    body: "Point AgentClash at a private monorepo or a seeded fixture. Same agent, same tools, same runtime — the only thing that changes is the task.",
  },
  {
    label: "Patches",
    title: "Apply, build, test, verdict.",
    body: "Every run ends with a patch applied against the base commit and the full test suite green or red. No graders guessing if the agent was on the right track.",
  },
  {
    label: "Budgets",
    title: "Cost and latency as first-class.",
    body: "A correct PR that burns 400k tokens and 11 minutes is still a bad PR. Budgets ride alongside correctness so you can pick the agent that ships.",
  },
  {
    label: "Head-to-head",
    title: "Race four agents at once.",
    body: "Claude Code, Cursor, Aider, and a custom harness on the exact same task. Side-by-side replay of tool calls, diffs, and terminal output.",
  },
];

const FAQ_ITEMS = [
  {
    question: "How is this different from SWE-bench?",
    answer:
      "SWE-bench is a fixed task set with a fixed harness. AgentClash is the harness: bring your own repos, your own tools, your own agents. You can run SWE-bench-style tasks on it, but you can also run your team's real tickets against the same scoring pipeline.",
  },
  {
    question: "What tools do coding agents get?",
    answer:
      "Each run gets an E2B sandbox with filesystem access, a shell for exec calls (build, test, lint, run), git for branch and commit operations, and optional http for package installs or external APIs. The tool policy is per challenge pack so you can restrict what each agent is allowed to touch.",
  },
  {
    question: "Can I evaluate my own internal coding agent?",
    answer:
      "Yes. AgentClash treats any agent that speaks the run protocol as a first-class competitor — OpenAI, Anthropic, a fine-tune, or a custom harness with its own tool router. Race it head-to-head against frontier models on the same tasks.",
  },
  {
    question: "How do you grade a patch?",
    answer:
      "The sandbox applies the patch to the base commit, runs the build and test suite declared by the challenge pack, and captures the exit code. On top of that, trajectory signatures grade behaviors: did it read the failing test, did it re-run locally before submitting, did it avoid tool loops.",
  },
  {
    question: "What about agents that can't finish in one turn?",
    answer:
      "Coding tasks are multi-turn by default. The run workflow persists tool state between turns and tails everything live. Long tasks get replayed end-to-end in the UI — you can scrub through a 40-step trajectory and see exactly where the agent got stuck.",
  },
];

export default function CodingAgentsPage() {
  return (
    <>
      <JsonLd
        id="ld-coding-product"
        data={productSchema({
          name: "AgentClash — coding agent evaluation",
          description:
            "Race autonomous coding agents against each other on real repos. Score tool use, patch correctness, and cost in one pipeline.",
          url: PATH,
        })}
      />
      <JsonLd
        id="ld-coding-breadcrumbs"
        data={breadcrumbSchema([
          { name: "Home", url: "/v2" },
          { name: "Use cases", url: "/v2/use-cases/coding-agents" },
          { name: "Coding agents", url: PATH },
        ])}
      />
      <MarketingShell>
        <PageHeader
          breadcrumbs={[
            { label: "Home", href: "/v2" },
            { label: "Use cases" },
            { label: "Coding agents" },
          ]}
          eyebrow="Coding agent evaluation"
          title={
            <>
              Race coding agents
              <br />
              <span className="text-white/40">on your actual repo.</span>
            </>
          }
          subtitle={
            <>
              Stop comparing agents on leaderboard screenshots. AgentClash
              drops Claude Code, Cursor, Aider, and your own harness into
              the same sandbox with your repo, your tools, and your tests —
              then grades them on the patch, the plan, and the bill.
            </>
          }
          cta={
            <div className="flex flex-col sm:flex-row sm:flex-wrap sm:items-center gap-3">
              <DemoButton />
              <Link
                href="/v2/platform/agent-evaluation"
                className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
              >
                Evaluation primitives
                <ArrowRight className="size-4" />
              </Link>
            </div>
          }
          aside={
            <CodeCard
              title="A coding task"
              code={`# packs/fix-flaky-queue-test.yaml
base: agentclash/monorepo@e3f1...
tools: [fs, exec, git]
task: |
  The test in internal/queue/worker_test.go
  flakes under -race. Find it, fix it, open
  a PR against main.
expected_signature:
  - tool_order: [fs:read, exec:go_test, fs:write, git:commit]
  - terminated: pr_opened
  - cost_max: $0.08`}
            />
          }
        />

        <SplitSection
          eyebrow="Trajectory over output"
          title={
            <>
              A correct diff from a bad plan
              <br />
              <span className="text-white/40">is still a bad agent.</span>
            </>
          }
          body={
            <>
              <p>
                Two agents can ship the same one-line fix. One reads the
                failing test, runs it, edits the fix, and re-runs before
                opening a PR. The other guesses, patches, and submits
                without verifying. Both land green in CI — only one is
                safe to put on your critical path.
              </p>
              <p className="mt-4">
                AgentClash scores tool order, redundancy, recovery from
                build errors, and unnecessary file reads. The verdict
                isn&apos;t just &ldquo;tests pass&rdquo; — it&apos;s
                &ldquo;this agent behaved like a senior engineer.&rdquo;
              </p>
            </>
          }
          aside={
            <CodeCard
              title="Trajectory — agent A vs B"
              code={`# agent_a.trajectory
  1 tool: fs.read("worker_test.go")
  2 tool: exec("go test -race ./internal/queue")
  3 tool: fs.read("internal/queue/worker.go")
  4 tool: fs.write("internal/queue/worker.go")
  5 tool: exec("go test -race ./internal/queue")  → PASS
  6 tool: git.commit; git.push; pr.open

# agent_b.trajectory
  1 tool: fs.write("internal/queue/worker.go")    ← no reads
  2 tool: git.commit; git.push; pr.open

  verdict_a:   ✓ 9.2 / 10   cost $0.04   4m12s
  verdict_b:   ✗ 4.1 / 10   cost $0.01   0m38s`}
            />
          }
        />

        <SplitSection
          reverse
          eyebrow="Head-to-head"
          title={
            <>
              Four agents, one task,
              <br />
              <span className="text-white/40">one replay to scrub.</span>
            </>
          }
          body={
            <>
              <p>
                Point four providers at the same seeded repo and let them
                go. Every tool call, stdout chunk, and patch hunk streams
                into the UI in real time. When the race ends you get a
                side-by-side replay with verdicts, cost, and a diff of
                what each one actually shipped.
              </p>
            </>
          }
          aside={
            <CodeCard
              title="Race a new model"
              code={`$ agentclash run create \\
    --pack coding-swe-easy \\
    --agents gpt-5.2,claude-4.6,aider-main,custom-harness \\
    --follow

  [00:41] claude-4.6    ✓ tests green, PR opened
  [01:08] gpt-5.2       ✓ tests green, PR opened
  [02:33] aider-main    ✗ build failure, gave up
  [03:14] custom-harness✓ tests green, PR opened

  leaderboard:
    1. claude-4.6      9.4  $0.031  2m18s
    2. custom-harness  9.1  $0.092  3m14s
    3. gpt-5.2         8.6  $0.048  1m08s
    4. aider-main      2.3  $0.004  2m33s`}
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
                Built for the agents that actually touch production code.
              </h2>
            </div>
            <div className="mt-20">
              <FeatureGrid features={FEATURES} columns={3} />
            </div>
          </div>
        </section>

        <FAQBlock items={FAQ_ITEMS} schemaId="ld-coding-faq" />

        <ClosingCTA
          title={
            <>
              Ship the agent
              <br />
              <span className="text-white/40">that reads the test first.</span>
            </>
          }
          body={
            <p>
              Let us race your shortlist against a seeded copy of your
              monorepo so you can see how each one handles a real ticket
              before you let it touch main.
            </p>
          }
        >
          <div className="flex flex-col sm:flex-row sm:flex-wrap gap-3">
            <DemoButton
              className="inline-flex items-center justify-center gap-2 rounded-md bg-white px-7 py-3 text-sm font-medium text-[#060606] hover:bg-white/90 transition-colors"
            />
            <Link
              href="/v2/platform/ci-cd-gating"
              className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-7 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
            >
              Gate your CI
              <ArrowRight className="size-4" />
            </Link>
          </div>
        </ClosingCTA>
      </MarketingShell>
    </>
  );
}
