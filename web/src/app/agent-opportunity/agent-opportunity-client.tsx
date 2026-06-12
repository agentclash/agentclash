"use client";

import { FormEvent, useMemo, useState } from "react";
import {
  AlertTriangle,
  ArrowRight,
  ClipboardCheck,
  Download,
  FlaskConical,
  Loader2,
  Search,
  ShieldCheck,
  Sparkles,
  Target,
  TrendingUp,
} from "lucide-react";
import type { AgentOpportunityReport } from "@/lib/agent-opportunity";
import { FitScoreRing } from "./components/fit-score-ring";
import { MetricTile } from "./components/metric-tile";
import {
  AGENT_OPPORTUNITY_FAQ,
  AGENT_OPPORTUNITY_H1,
  AGENT_OPPORTUNITY_LEDE,
  AGENT_OPPORTUNITY_SECTIONS,
} from "./agent-opportunity-seo";
import {
  confidenceCopy,
  deriveOpportunityMetrics,
  fitCopy,
  fitTone,
} from "./report-metrics";

type ReportResponse =
  | { ok: true; report: AgentOpportunityReport }
  | { ok: false; code: string; error: string };

type ReportTab = "overview" | "opportunities" | "risks" | "eval";

const painOptions = [
  "Support",
  "Sales qualification",
  "Onboarding",
  "Developer docs",
  "Internal operations",
  "Not sure yet",
];

const loadingSteps = [
  "Fetching public pages",
  "Researching the company on the web",
  "Scoring workflows and risks",
];

function toneFromScore(score: number): "good" | "warn" | "bad" | "neutral" {
  if (score >= 75) return "good";
  if (score >= 45) return "warn";
  if (score >= 25) return "neutral";
  return "bad";
}

function ReportTabs({
  active,
  onChange,
}: {
  active: ReportTab;
  onChange: (tab: ReportTab) => void;
}) {
  const tabs: { id: ReportTab; label: string }[] = [
    { id: "overview", label: "Overview" },
    { id: "opportunities", label: "Opportunities" },
    { id: "risks", label: "Risks" },
    { id: "eval", label: "Eval plan" },
  ];

  return (
    <div className="flex flex-wrap gap-1 rounded-md border border-white/[0.08] bg-white/[0.02] p-1">
      {tabs.map((tab) => (
        <button
          key={tab.id}
          type="button"
          onClick={() => onChange(tab.id)}
          className={
            active === tab.id
              ? "rounded-sm bg-white px-3 py-1.5 text-xs font-medium text-[#060606]"
              : "rounded-sm px-3 py-1.5 text-xs text-white/55 transition-colors hover:text-white"
          }
        >
          {tab.label}
        </button>
      ))}
    </div>
  );
}

function UseCaseCard({ useCase }: { useCase: AgentOpportunityReport["useCases"][number] }) {
  const fitScore =
    useCase.fit === "high" ? 88 : useCase.fit === "medium" ? 58 : 28;

  return (
    <article className="rounded-md border border-white/[0.08] bg-[#060606] p-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <p className="text-sm font-medium text-white">{useCase.title}</p>
          <p className="mt-2 text-sm leading-6 text-white/55">{useCase.workflow}</p>
        </div>
        <span className="rounded-sm border border-white/[0.08] px-2 py-1 font-[family-name:var(--font-mono)] text-[10px] uppercase tracking-[0.12em] text-white/45">
          {useCase.fit} fit
        </span>
      </div>
      <div className="mt-4 h-[3px] overflow-hidden rounded-full bg-white/[0.05]">
        <div
          className="h-full rounded-full bg-cyan-500/80"
          style={{ width: `${fitScore}%` }}
        />
      </div>
      <div className="mt-4 grid gap-2 text-xs sm:grid-cols-3">
        <p className="text-white/45">
          Hours saved{" "}
          <span className="text-white/75">{useCase.estimatedMonthlyHoursSaved}</span>
        </p>
        <p className="text-white/45">
          Savings{" "}
          <span className="text-white/75">{useCase.estimatedMonthlySavingsUsd}</span>
        </p>
        <p className="text-white/45">
          Complexity{" "}
          <span className="capitalize text-white/75">{useCase.complexity}</span>
        </p>
      </div>
      <p className="mt-4 text-sm leading-6 text-white/50">{useCase.why}</p>
      <ul className="mt-4 grid gap-2 text-xs text-white/45 sm:grid-cols-2">
        {useCase.firstEvalTasks.map((task) => (
          <li key={task} className="flex items-start gap-2">
            <ClipboardCheck className="mt-0.5 size-3.5 shrink-0" />
            <span>{task}</span>
          </li>
        ))}
      </ul>
    </article>
  );
}

function ReportDashboard({ report }: { report: AgentOpportunityReport }) {
  const [tab, setTab] = useState<ReportTab>("overview");
  const metrics = useMemo(() => deriveOpportunityMetrics(report), [report]);
  const verdict = fitCopy[report.shouldBuildAgent];

  return (
    <section className="mt-10 overflow-hidden rounded-lg border border-white/[0.08] bg-[#080808]">
      <div className="border-b border-white/[0.08] p-5 sm:p-6">
        <div className="flex flex-col gap-6 lg:flex-row lg:items-start lg:justify-between">
          <div className="min-w-0 flex-1">
            <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.16em] text-white/40">
              Agent opportunity report
            </p>
            <h2 className="mt-3 truncate text-2xl font-semibold tracking-tight text-white sm:text-3xl">
              {report.companyName}
            </h2>
            <p className="mt-3 max-w-2xl text-sm leading-6 text-white/55">
              {report.summary}
            </p>
            <div className="mt-4 flex flex-wrap items-center gap-2">
              <span
                className={
                  fitTone[report.shouldBuildAgent] === "good"
                    ? "rounded-sm border border-emerald-500/25 bg-emerald-500/10 px-2.5 py-1 text-xs font-medium text-emerald-200"
                    : fitTone[report.shouldBuildAgent] === "warn"
                      ? "rounded-sm border border-amber-500/25 bg-amber-500/10 px-2.5 py-1 text-xs font-medium text-amber-200"
                      : fitTone[report.shouldBuildAgent] === "bad"
                        ? "rounded-sm border border-red-500/25 bg-red-500/10 px-2.5 py-1 text-xs font-medium text-red-200"
                        : "rounded-sm border border-white/10 bg-white/[0.04] px-2.5 py-1 text-xs font-medium text-white/75"
                }
              >
                {verdict}
              </span>
              <span className="text-xs text-white/40">
                {confidenceCopy[report.fitLevel]}
              </span>
            </div>
          </div>

          <div className="flex shrink-0 flex-col items-center gap-3 sm:flex-row lg:flex-col">
            <FitScoreRing score={report.agentFitScore} />
            <button
              type="button"
              onClick={() => window.print()}
              className="inline-flex items-center gap-2 rounded-md border border-white/[0.1] px-3 py-2 text-xs text-white/70 transition-colors hover:border-white/25 hover:text-white print:hidden"
            >
              <Download className="size-3.5" />
              Save report
            </button>
          </div>
        </div>

        <div className="mt-6 grid grid-cols-1 gap-px overflow-hidden rounded-md border border-white/[0.08] bg-white/[0.08] sm:grid-cols-2 xl:grid-cols-4">
          <MetricTile
            icon={Target}
            label="Workflow fit"
            value={
              metrics.workflowFit >= 75
                ? "Repeatable workflows found"
                : metrics.workflowFit >= 45
                  ? "Some repeatable work"
                  : "Weak workflow signal"
            }
            hint="How clearly the site exposes agent-ready work."
            score={metrics.workflowFit}
            tone={toneFromScore(metrics.workflowFit)}
          />
          <MetricTile
            icon={TrendingUp}
            label="ROI signal"
            value={
              metrics.roiSignal >= 75
                ? "Conservative upside"
                : metrics.roiSignal >= 45
                  ? "Limited upside"
                  : "Hard to justify now"
            }
            hint="Blended fit score and workflow quality."
            score={metrics.roiSignal}
            tone={toneFromScore(metrics.roiSignal)}
          />
          <MetricTile
            icon={ShieldCheck}
            label="Risk profile"
            value={
              metrics.riskProfile >= 75
                ? "Risks look manageable"
                : metrics.riskProfile >= 45
                  ? "Needs guardrails"
                  : "High failure risk"
            }
            hint="Higher is safer. Based on severity of listed risks."
            score={metrics.riskProfile}
            tone={toneFromScore(metrics.riskProfile)}
          />
          <MetricTile
            icon={FlaskConical}
            label="Eval readiness"
            value={
              metrics.evalReadiness >= 75
                ? "Ready to test"
                : metrics.evalReadiness >= 45
                  ? "Test before building"
                  : "Do not ship yet"
            }
            hint="Whether AgentClash eval should come next."
            score={metrics.evalReadiness}
            tone={toneFromScore(metrics.evalReadiness)}
          />
        </div>
      </div>

      <div className="border-b border-white/[0.08] px-5 py-4 sm:px-6">
        <ReportTabs active={tab} onChange={setTab} />
      </div>

      <div className="max-h-[min(70vh,720px)] overflow-y-auto p-5 sm:p-6">
        {tab === "overview" ? (
          <div className="grid gap-6 lg:grid-cols-[1.1fr_0.9fr]">
            <div className="space-y-4">
              <h3 className="text-sm font-medium text-white">Honest verdict</h3>
              <p className="rounded-md border border-white/[0.08] bg-[#060606] p-4 text-sm leading-7 text-white/65">
                {report.honestVerdict}
              </p>
              <div>
                <h3 className="text-sm font-medium text-white">Next steps</h3>
                <ul className="mt-3 space-y-2 text-sm leading-6 text-white/55">
                  {report.nextSteps.map((step) => (
                    <li key={step} className="flex gap-2">
                      <span className="font-[family-name:var(--font-mono)] text-white/30">
                        →
                      </span>
                      <span>{step}</span>
                    </li>
                  ))}
                </ul>
              </div>
            </div>
            <div>
              <h3 className="text-sm font-medium text-white">Evidence limits</h3>
              <ul className="mt-3 space-y-2 text-sm leading-6 text-white/45">
                {report.evidenceLimitations.map((limit) => (
                  <li key={limit}>{limit}</li>
                ))}
              </ul>
              <p className="mt-5 font-[family-name:var(--font-mono)] text-[10px] uppercase tracking-[0.14em] text-white/30">
                Analyzed {report.analyzedUrl}
              </p>
            </div>
          </div>
        ) : null}

        {tab === "opportunities" ? (
          <div className="space-y-4">
            <div className="flex items-center gap-2">
              <Sparkles className="size-4 text-cyan-200" />
              <h3 className="text-sm font-medium text-white">Best agent use cases</h3>
            </div>
            <div className="grid gap-3">
              {report.useCases.map((useCase) => (
                <UseCaseCard key={useCase.title} useCase={useCase} />
              ))}
            </div>
          </div>
        ) : null}

        {tab === "risks" ? (
          <div className="grid gap-3 md:grid-cols-2">
            {report.risks.map((risk) => (
              <article
                key={risk.risk}
                className="rounded-md border border-white/[0.08] bg-[#060606] p-4"
              >
                <div className="flex items-start justify-between gap-3">
                  <div className="flex items-start gap-2">
                    <AlertTriangle className="mt-0.5 size-4 shrink-0 text-amber-200" />
                    <p className="text-sm font-medium text-white">{risk.risk}</p>
                  </div>
                  <span className="rounded-sm border border-white/[0.08] px-2 py-0.5 text-[10px] uppercase tracking-[0.12em] text-white/40">
                    {risk.severity}
                  </span>
                </div>
                <p className="mt-3 text-sm leading-6 text-white/50">{risk.mitigation}</p>
              </article>
            ))}
          </div>
        ) : null}

        {tab === "eval" ? (
          <div className="grid gap-6 lg:grid-cols-[1fr_0.9fr]">
            <div className="rounded-md border border-white/[0.08] bg-[#060606] p-5">
              <div className="flex items-center gap-2">
                <ShieldCheck className="size-4 text-emerald-200" />
                <h3 className="text-sm font-medium text-white">AgentClash eval pack</h3>
              </div>
              <p className="mt-4 text-base font-medium text-white">
                {report.evaluationPack.name}
              </p>
              <p className="mt-3 text-sm leading-6 text-white/55">
                {report.evaluationPack.recommendedCases} realistic cases,{" "}
                {report.evaluationPack.adversarialCases} adversarial cases.
              </p>
              <ul className="mt-4 space-y-2 text-sm leading-6 text-white/45">
                {report.evaluationPack.successCriteria.map((criterion) => (
                  <li key={criterion} className="flex gap-2">
                    <span className="text-white/25">•</span>
                    <span>{criterion}</span>
                  </li>
                ))}
              </ul>
            </div>
            <div className="flex flex-col justify-between gap-4 rounded-md border border-white/[0.08] bg-white/[0.02] p-5">
              <div>
                <p className="text-sm font-medium text-white">
                  Race agents on real cases before you ship
                </p>
                <p className="mt-2 text-sm leading-6 text-white/50">
                  Turn this report into a challenge pack and compare models on the
                  workflows that actually matter for {report.companyName}.
                </p>
              </div>
              <a
                href="/auth/login"
                className="inline-flex w-fit items-center gap-2 rounded-md bg-white px-3 py-2 text-sm font-medium text-[#060606] transition-colors hover:bg-white/90"
              >
                Create eval workspace
                <ArrowRight className="size-4" />
              </a>
            </div>
          </div>
        ) : null}
      </div>
    </section>
  );
}

function LoadingPanel({ step }: { step: number }) {
  return (
    <section className="mt-10 overflow-hidden rounded-lg border border-white/[0.08] bg-[#080808] p-6">
      <div className="flex items-center gap-3">
        <Loader2 className="size-5 animate-spin text-white/60" />
        <div>
          <p className="text-sm font-medium text-white">Generating your report</p>
          <p className="mt-1 text-xs text-white/45">
            This usually takes 20 to 60 seconds while we search the web.
          </p>
        </div>
      </div>
      <ol className="mt-6 space-y-3">
        {loadingSteps.map((label, index) => {
          const active = index === step;
          const done = index < step;
          return (
            <li
              key={label}
              className={
                done
                  ? "flex items-center gap-3 text-sm text-emerald-200/80"
                  : active
                    ? "flex items-center gap-3 text-sm text-white"
                    : "flex items-center gap-3 text-sm text-white/35"
              }
            >
              <span className="flex size-6 items-center justify-center rounded-full border border-white/10 font-[family-name:var(--font-mono)] text-[11px]">
                {done ? "✓" : index + 1}
              </span>
              {label}
            </li>
          );
        })}
      </ol>
    </section>
  );
}

function SeoContent() {
  return (
    <section className="mt-16 border-t border-white/[0.08] pt-12">
      <div className="grid gap-10 lg:grid-cols-[1.1fr_0.9fr]">
        <div>
          <h2 className="text-2xl font-semibold tracking-tight text-white sm:text-3xl">
            Free AI agent ROI calculator and build vs buy assessment
          </h2>
          <p className="mt-4 text-sm leading-7 text-white/55">
            Teams search for AI agent ROI calculators, build vs buy frameworks,
            and honest answers to &quot;should we build an AI agent?&quot; This
            page combines those intents into one URL-based report with agentic AI
            use cases, conservative savings ranges, and eval guidance.
          </p>
          <div className="mt-8 grid gap-4">
            {AGENT_OPPORTUNITY_SECTIONS.map((section) => (
              <article
                key={section.title}
                className="rounded-md border border-white/[0.08] bg-white/[0.02] p-4"
              >
                <h3 className="text-sm font-medium text-white">{section.title}</h3>
                <p className="mt-2 text-sm leading-6 text-white/50">{section.body}</p>
              </article>
            ))}
          </div>
        </div>

        <div>
          <h2 className="text-lg font-semibold tracking-tight text-white">
            AI agent evaluation FAQ
          </h2>
          <div className="mt-5 space-y-4">
            {AGENT_OPPORTUNITY_FAQ.map((item) => (
              <details
                key={item.question}
                className="group rounded-md border border-white/[0.08] bg-[#060606] p-4"
              >
                <summary className="cursor-pointer list-none text-sm font-medium text-white marker:content-none [&::-webkit-details-marker]:hidden">
                  <span className="flex items-start justify-between gap-3">
                    <span>{item.question}</span>
                    <span className="text-white/30 transition-transform group-open:rotate-45">
                      +
                    </span>
                  </span>
                </summary>
                <p className="mt-3 text-sm leading-6 text-white/50">{item.answer}</p>
              </details>
            ))}
          </div>
          <p className="mt-6 text-xs leading-6 text-white/35">
            Ready to test before you ship? Explore{" "}
            <a href="/platform/agent-evaluation" className="text-white/55 underline-offset-2 hover:text-white hover:underline">
              AI agent evaluation
            </a>
            ,{" "}
            <a href="/enterprise" className="text-white/55 underline-offset-2 hover:text-white hover:underline">
              enterprise eval gates
            </a>
            , or{" "}
            <a href="/tryouts" className="text-white/55 underline-offset-2 hover:text-white hover:underline">
              public agent tryouts
            </a>
            .
          </p>
        </div>
      </div>
    </section>
  );
}

export function AgentOpportunityClient() {
  const [url, setUrl] = useState("");
  const [companySize, setCompanySize] = useState("");
  const [currentPain, setCurrentPain] = useState("");
  const [monthlySupportVolume, setMonthlySupportVolume] = useState("");
  const [loading, setLoading] = useState(false);
  const [loadingStep, setLoadingStep] = useState(0);
  const [error, setError] = useState("");
  const [report, setReport] = useState<AgentOpportunityReport | null>(null);

  const canSubmit = url.trim().length > 0 && !loading;

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!canSubmit) return;

    setLoading(true);
    setLoadingStep(0);
    setError("");
    setReport(null);

    const stepTimer = window.setInterval(() => {
      setLoadingStep((current) => Math.min(current + 1, loadingSteps.length - 1));
    }, 9000);

    try {
      const response = await fetch("/api/agent-opportunity", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          url,
          companySize,
          currentPain,
          monthlySupportVolume,
        }),
      });
      const payload = (await response.json()) as ReportResponse;
      if (!payload.ok) {
        setError(payload.error);
        return;
      }
      setReport(payload.report);
    } catch {
      setError("The report request failed. Please try again.");
    } finally {
      window.clearInterval(stepTimer);
      setLoading(false);
    }
  }

  return (
    <>
      <section className="px-6 pt-14 sm:px-12 sm:pt-20">
        <div className="mx-auto max-w-[980px]">
            <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.16em] text-white/40">
              AI agent ROI calculator
            </p>
            <h1 className="mt-5 max-w-[16ch] text-[clamp(2.4rem,6vw,4.5rem)] font-sans font-semibold leading-[0.98] tracking-tight text-white">
              {AGENT_OPPORTUNITY_H1}
            </h1>
            <p className="mt-5 max-w-[52ch] text-base leading-7 text-white/60 sm:text-lg">
              {AGENT_OPPORTUNITY_LEDE}
            </p>

          <form
            onSubmit={onSubmit}
            className="mt-8 overflow-hidden rounded-lg border border-white/[0.1] bg-[#0a0a0a]"
          >
            <div className="border-b border-white/[0.08] px-4 py-3 sm:px-5">
              <label className="block text-sm font-medium text-white" htmlFor="url">
                Company URL
              </label>
              <div className="mt-3 flex flex-col gap-3 sm:flex-row">
                <input
                  id="url"
                  value={url}
                  onChange={(event) => setUrl(event.target.value)}
                  placeholder="https://example.com"
                  className="min-h-11 min-w-0 flex-1 rounded-md border border-white/[0.1] bg-white/[0.04] px-3 text-sm text-white outline-none transition-colors placeholder:text-white/25 focus:border-white/30"
                />
                <button
                  type="submit"
                  disabled={!canSubmit}
                  className="inline-flex min-h-11 shrink-0 items-center justify-center gap-2 rounded-md bg-white px-4 text-sm font-medium text-[#060606] transition-colors hover:bg-white/90 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {loading ? <Loader2 className="size-4 animate-spin" /> : <Search className="size-4" />}
                  Analyze
                </button>
              </div>
            </div>

            <div className="grid gap-px bg-white/[0.08] sm:grid-cols-3">
              <label className="block bg-[#0a0a0a] p-4">
                <span className="text-xs text-white/45">Company size</span>
                <select
                  value={companySize}
                  onChange={(event) => setCompanySize(event.target.value)}
                  className="mt-2 min-h-10 w-full rounded-md border border-white/[0.1] bg-[#111] px-3 text-sm text-white/75 outline-none focus:border-white/30"
                >
                  <option value="">Unknown</option>
                  <option value="1-10">1-10</option>
                  <option value="11-50">11-50</option>
                  <option value="51-200">51-200</option>
                  <option value="201-1000">201-1000</option>
                  <option value="1000+">1000+</option>
                </select>
              </label>
              <label className="block bg-[#0a0a0a] p-4">
                <span className="text-xs text-white/45">Main pain</span>
                <select
                  value={currentPain}
                  onChange={(event) => setCurrentPain(event.target.value)}
                  className="mt-2 min-h-10 w-full rounded-md border border-white/[0.1] bg-[#111] px-3 text-sm text-white/75 outline-none focus:border-white/30"
                >
                  <option value="">Unknown</option>
                  {painOptions.map((option) => (
                    <option key={option} value={option}>
                      {option}
                    </option>
                  ))}
                </select>
              </label>
              <label className="block bg-[#0a0a0a] p-4">
                <span className="text-xs text-white/45">Support volume</span>
                <select
                  value={monthlySupportVolume}
                  onChange={(event) => setMonthlySupportVolume(event.target.value)}
                  className="mt-2 min-h-10 w-full rounded-md border border-white/[0.1] bg-[#111] px-3 text-sm text-white/75 outline-none focus:border-white/30"
                >
                  <option value="">Unknown</option>
                  <option value="0-100/month">0-100/month</option>
                  <option value="100-500/month">100-500/month</option>
                  <option value="500-2000/month">500-2000/month</option>
                  <option value="2000+/month">2000+/month</option>
                </select>
              </label>
            </div>

            {error ? (
              <p className="border-t border-amber-300/20 bg-amber-300/10 px-4 py-3 text-sm leading-6 text-amber-100 sm:px-5">
                {error}
              </p>
            ) : null}
          </form>
        </div>
      </section>

      <div className="px-6 pb-20 sm:px-12">
        <div className="mx-auto max-w-[980px]">
          {loading ? <LoadingPanel step={loadingStep} /> : null}
          {report ? <ReportDashboard report={report} /> : null}
          <SeoContent />
        </div>
      </div>
    </>
  );
}
