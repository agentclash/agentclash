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

const PATH = "/v2/use-cases/sql-data-agents";

export const metadata: Metadata = {
  title: "Evaluate text-to-SQL and data agents",
  description:
    "Evaluate text-to-SQL and data analysis agents on correctness, schema awareness, and query economy. Benchmark analytics copilots with AgentClash.",
  alternates: { canonical: PATH },
  openGraph: {
    title: "Evaluate SQL and data agents — AgentClash",
    description:
      "Grade text-to-SQL and analytics agents on rows returned, schema awareness, and query economy — not just whether the SQL parsed.",
    url: PATH,
  },
};

const FEATURES = [
  {
    label: "Row correctness",
    title: "Did it return the right answer?",
    body: "We execute the agent's query against the actual warehouse and diff the result set against the golden rows. No LLM judge, no exec-plan heuristic — real rows, real verdict.",
  },
  {
    label: "Schema awareness",
    title: "Introspect before you guess.",
    body: "Good data agents inspect tables, columns, and foreign keys before writing a join. Trajectory signatures grade whether the agent looked before it leaped.",
  },
  {
    label: "Query economy",
    title: "One query beats a thousand.",
    body: "An answer that takes 40 CTEs and a full-table scan is a worse answer. We score query shape — predicate pushdown, join order, scanned bytes — alongside correctness.",
  },
  {
    label: "Dialect safety",
    title: "Postgres isn't Snowflake.",
    body: "Window functions, date types, and NULL handling diverge across dialects. Run the same prompt against multiple warehouses and see which agents actually respect the dialect.",
  },
  {
    label: "Guardrails",
    title: "No DROP on the way out.",
    body: "Policy-enforced tool scopes let analytics agents read without breaking production. A SELECT-only sandbox is one line of pack config.",
  },
  {
    label: "Replay",
    title: "Every query, every row.",
    body: "Scrub through the full trajectory — introspections, intermediate selects, the final query, and the rows returned — side by side with the golden answer.",
  },
];

const FAQ_ITEMS = [
  {
    question: "How does AgentClash grade text-to-SQL correctness?",
    answer:
      "We execute both the agent's query and the reference query against the same warehouse snapshot and diff the result sets. Rows, types, and ordering are compared explicitly. Queries that parse but return the wrong rows fail — this catches a lot of silent failures that plan-based graders miss.",
  },
  {
    question: "What about agents that rewrite the question?",
    answer:
      "Analytics agents often clarify or rephrase the user's request. The pack can specify allowed clarifications, and the verdict is graded against the canonical intent — not just the surface string. If the agent answers a different question, the verdict reflects that.",
  },
  {
    question: "Can I evaluate against my own schema?",
    answer:
      "Yes. Point a pack at a warehouse snapshot (or a synthetic clone of your prod schema with seeded rows) and the sandbox exposes it as a read-only sql tool. No warehouse credentials leave the sandbox.",
  },
  {
    question: "Does this work for Snowflake, BigQuery, Postgres?",
    answer:
      "Yes — the sql tool is dialect-agnostic at the interface level; the sandbox image controls which engine actually runs the query. Race the same prompt against Postgres, Snowflake, and BigQuery and see which agents respect each dialect's quirks.",
  },
  {
    question: "How do you score query economy?",
    answer:
      "We capture the EXPLAIN output and the scanned-bytes cost reported by the warehouse. Two agents that return the same rows but scan 10× different data get different economy scores, which roll into the overall verdict.",
  },
];

export default function SqlDataAgentsPage() {
  return (
    <>
      <JsonLd
        id="ld-sql-product"
        data={productSchema({
          name: "AgentClash — SQL and data agent evaluation",
          description:
            "Grade text-to-SQL and analytics agents on row correctness, schema awareness, and query economy.",
          url: PATH,
        })}
      />
      <JsonLd
        id="ld-sql-breadcrumbs"
        data={breadcrumbSchema([
          { name: "Home", url: "/v2" },
          { name: "Use cases", url: "/v2/use-cases/coding-agents" },
          { name: "SQL and data agents", url: PATH },
        ])}
      />
      <MarketingShell>
        <PageHeader
          breadcrumbs={[
            { label: "Home", href: "/v2" },
            { label: "Use cases" },
            { label: "SQL and data agents" },
          ]}
          eyebrow="SQL and data agents"
          title={
            <>
              Grade the rows,
              <br />
              <span className="text-white/40">not the SQL.</span>
            </>
          }
          subtitle={
            <>
              A query that parses and a query that answers the question
              are not the same thing. AgentClash runs text-to-SQL and
              analytics agents against real warehouse snapshots and
              scores them on rows returned, schema awareness, and the
              shape of the query they chose.
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
              title="A text-to-SQL case"
              code={`# packs/revenue-cohort-rollup.yaml
warehouse: snowflake://snap/prod-clone-2026-04-01
tools: [sql, introspect, plot, cache]
question: |
  Top 5 customers by Q1-2026 net new ARR,
  excluding self-serve and including only
  accounts with > 12 months tenure.
golden_rows: artifacts/gold/q1-ranks.parquet
require:
  - row_diff: exact
  - introspected_tables_min: 3
  - scanned_bytes_max: 8GB`}
            />
          }
        />

        <SplitSection
          eyebrow="Inspect before you guess"
          title={
            <>
              The best SQL agents
              <br />
              <span className="text-white/40">read the schema first.</span>
            </>
          }
          body={
            <>
              <p>
                A common failure: the agent assumes
                <code className="mx-1 rounded bg-white/[0.06] px-1.5 py-0.5 font-[family-name:var(--font-mono)] text-[13px] text-white/80">customers.tier</code>
                is an enum when the real column is a foreign key to
                <code className="mx-1 rounded bg-white/[0.06] px-1.5 py-0.5 font-[family-name:var(--font-mono)] text-[13px] text-white/80">plans.id</code>.
                The query parses, the rows are wrong, nobody catches it
                until a dashboard disagrees with finance.
              </p>
              <p className="mt-4">
                We score
                <code className="mx-1 rounded bg-white/[0.06] px-1.5 py-0.5 font-[family-name:var(--font-mono)] text-[13px] text-white/80">introspect</code>
                calls explicitly: which tables were inspected, whether
                foreign keys were loaded, whether sample rows were pulled
                before the agent committed to a join. Agents that skip
                the inspection step have their verdicts docked even when
                the rows happen to come back right.
              </p>
            </>
          }
          aside={
            <CodeCard
              title="Trajectory — inspect vs guess"
              code={`# agent_a.trajectory
  1 tool: introspect("customers")
  2 tool: introspect("plans")
  3 tool: introspect("subscriptions")
  4 tool: sql("SELECT ... FROM customers c
               JOIN plans p ON p.id = c.plan_id
               WHERE c.created_at < ...")
  5 verify: rows match golden ✓

# agent_b.trajectory
  1 tool: sql("SELECT ... WHERE c.tier='enterprise'")
     → ERROR: column "tier" does not exist
  2 tool: sql("SELECT ... WHERE c.plan='enterprise'")
     → returns 0 rows (wrong)
  verdict: ✗ 2.1 / 10   no introspection, wrong rows`}
            />
          }
        />

        <SplitSection
          reverse
          eyebrow="Query economy"
          title={
            <>
              Right rows, wrong bill.
              <br />
              <span className="text-white/40">Still a regression.</span>
            </>
          }
          body={
            <>
              <p>
                A correct query that scans 2TB because the agent forgot a
                partition predicate is a production incident waiting to
                happen. AgentClash captures EXPLAIN output and scanned
                bytes for every run, so you can see which agents write
                queries that would survive a warehouse bill review.
              </p>
            </>
          }
          aside={
            <CodeCard
              title="Economy leaderboard"
              code={`$ agentclash run show run_01H... --economy

  agent                  rows  scan    cost    verdict
  claude-4.6             ✓     0.8GB   $0.04   9.3
  gpt-5.2                ✓     2.1GB   $0.11   8.7
  custom-harness         ✓    14.2GB   $0.68   6.1   ← no partition
  open-source-copilot    ✗     —       $0.02   3.2`}
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
                Benchmark SQL agents on the only thing that matters — the rows.
              </h2>
            </div>
            <div className="mt-20">
              <FeatureGrid features={FEATURES} columns={3} />
            </div>
          </div>
        </section>

        <FAQBlock items={FAQ_ITEMS} schemaId="ld-sql-faq" />

        <ClosingCTA
          title={
            <>
              Ship the analyst
              <br />
              <span className="text-white/40">that reads the schema.</span>
            </>
          }
          body={
            <p>
              Let us race two text-to-SQL agents against a cloned
              warehouse and your hardest ambiguous question — rows,
              economy, and trajectory side by side.
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
