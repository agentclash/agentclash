"use client";

import { FormEvent, useState } from "react";
import {
  AlertTriangle,
  ArrowRight,
  ClipboardCheck,
  Download,
  Gauge,
  Loader2,
  ShieldCheck,
  Sparkles,
} from "lucide-react";
import type { AgentOpportunityReport } from "@/lib/agent-opportunity";

type ReportResponse =
  | { ok: true; report: AgentOpportunityReport }
  | { ok: false; code: string; error: string };

const fitCopy: Record<AgentOpportunityReport["shouldBuildAgent"], string> = {
  not_yet: "Do not build yet",
  narrow_pilot: "Pilot narrowly",
  strong_fit: "Strong fit",
  eval_first: "Evaluate first",
};

const painOptions = [
  "Support",
  "Sales qualification",
  "Onboarding",
  "Developer docs",
  "Internal operations",
  "Not sure yet",
];

function scoreBand(score: number) {
  if (score >= 75) return "text-emerald-200";
  if (score >= 45) return "text-amber-200";
  return "text-white/70";
}

function ReportPanel({ report }: { report: AgentOpportunityReport }) {
  const verdict = fitCopy[report.shouldBuildAgent];

  return (
    <section className="mt-14 border-t border-white/[0.08] pt-10">
      <div className="grid gap-8 lg:grid-cols-[0.8fr_1.2fr]">
        <div>
          <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.16em] text-white/40">
            Agent opportunity report
          </p>
          <h2 className="mt-4 text-3xl font-sans font-semibold tracking-tight text-white sm:text-4xl">
            {report.companyName}
          </h2>
          <p className="mt-4 text-sm leading-6 text-white/55">
            {report.summary}
          </p>
          <div className="mt-7 grid grid-cols-2 gap-3">
            <div className="rounded-md border border-white/[0.08] bg-white/[0.03] p-4">
              <Gauge className="size-4 text-white/45" />
              <p className={`mt-5 text-4xl font-semibold ${scoreBand(report.agentFitScore)}`}>
                {report.agentFitScore}
              </p>
              <p className="mt-1 text-xs text-white/40">Fit score</p>
            </div>
            <div className="rounded-md border border-white/[0.08] bg-white/[0.03] p-4">
              <ShieldCheck className="size-4 text-white/45" />
              <p className="mt-5 text-lg font-semibold text-white">{verdict}</p>
              <p className="mt-1 text-xs capitalize text-white/40">
                {report.fitLevel} confidence
              </p>
            </div>
          </div>
          <p className="mt-6 rounded-md border border-white/[0.08] bg-[#0a0a0a] p-4 text-sm leading-6 text-white/65">
            {report.honestVerdict}
          </p>
          <button
            type="button"
            onClick={() => window.print()}
            className="mt-5 inline-flex items-center gap-2 rounded-md border border-white/[0.1] px-3 py-2 text-sm text-white/70 transition-colors hover:border-white/25 hover:text-white"
          >
            <Download className="size-4" />
            Save report
          </button>
        </div>

        <div className="space-y-8">
          <div>
            <div className="flex items-center gap-2">
              <Sparkles className="size-4 text-cyan-200" />
              <h3 className="text-lg font-semibold tracking-tight text-white">
                Best agent use cases
              </h3>
            </div>
            <div className="mt-4 grid gap-3">
              {report.useCases.map((useCase) => (
                <article
                  key={useCase.title}
                  className="rounded-md border border-white/[0.08] bg-white/[0.03] p-5"
                >
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <p className="text-base font-semibold text-white">
                        {useCase.title}
                      </p>
                      <p className="mt-2 text-sm leading-6 text-white/55">
                        {useCase.workflow}
                      </p>
                    </div>
                    <span className="rounded-md border border-white/[0.08] px-2 py-1 font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.12em] text-white/45">
                      {useCase.fit} fit
                    </span>
                  </div>
                  <div className="mt-5 grid gap-3 text-sm sm:grid-cols-3">
                    <p className="text-white/50">
                      Hours:{" "}
                      <span className="text-white/80">
                        {useCase.estimatedMonthlyHoursSaved}
                      </span>
                    </p>
                    <p className="text-white/50">
                      Savings:{" "}
                      <span className="text-white/80">
                        {useCase.estimatedMonthlySavingsUsd}
                      </span>
                    </p>
                    <p className="text-white/50">
                      Complexity:{" "}
                      <span className="text-white/80">{useCase.complexity}</span>
                    </p>
                  </div>
                  <p className="mt-4 text-sm leading-6 text-white/50">
                    {useCase.why}
                  </p>
                  <ul className="mt-4 grid gap-2 text-xs text-white/45 sm:grid-cols-2">
                    {useCase.firstEvalTasks.map((task) => (
                      <li key={task} className="flex items-start gap-2">
                        <ClipboardCheck className="mt-0.5 size-3.5 shrink-0" />
                        <span>{task}</span>
                      </li>
                    ))}
                  </ul>
                </article>
              ))}
            </div>
          </div>

          <div className="grid gap-4 md:grid-cols-2">
            <div>
              <div className="flex items-center gap-2">
                <AlertTriangle className="size-4 text-amber-200" />
                <h3 className="text-lg font-semibold tracking-tight text-white">
                  Risks to evaluate
                </h3>
              </div>
              <div className="mt-4 space-y-3">
                {report.risks.map((risk) => (
                  <div
                    key={risk.risk}
                    className="rounded-md border border-white/[0.08] bg-white/[0.03] p-4"
                  >
                    <p className="text-sm font-medium text-white">{risk.risk}</p>
                    <p className="mt-2 text-xs leading-5 text-white/50">
                      {risk.mitigation}
                    </p>
                  </div>
                ))}
              </div>
            </div>
            <div>
              <div className="flex items-center gap-2">
                <ShieldCheck className="size-4 text-emerald-200" />
                <h3 className="text-lg font-semibold tracking-tight text-white">
                  AgentClash eval pack
                </h3>
              </div>
              <div className="mt-4 rounded-md border border-white/[0.08] bg-white/[0.03] p-5">
                <p className="font-medium text-white">{report.evaluationPack.name}</p>
                <p className="mt-3 text-sm leading-6 text-white/55">
                  {report.evaluationPack.recommendedCases} realistic cases,{" "}
                  {report.evaluationPack.adversarialCases} adversarial cases.
                </p>
                <ul className="mt-4 space-y-2 text-xs leading-5 text-white/45">
                  {report.evaluationPack.successCriteria.map((criterion) => (
                    <li key={criterion}>{criterion}</li>
                  ))}
                </ul>
                <a
                  href="/auth/login"
                  className="mt-5 inline-flex items-center gap-2 rounded-md bg-white px-3 py-2 text-sm font-medium text-[#060606] transition-colors hover:bg-white/90"
                >
                  Create eval workspace
                  <ArrowRight className="size-4" />
                </a>
              </div>
            </div>
          </div>

          <div className="grid gap-4 border-t border-white/[0.08] pt-6 md:grid-cols-2">
            <div>
              <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.16em] text-white/35">
                Next steps
              </p>
              <ul className="mt-3 space-y-2 text-sm leading-6 text-white/55">
                {report.nextSteps.map((step) => (
                  <li key={step}>{step}</li>
                ))}
              </ul>
            </div>
            <div>
              <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.16em] text-white/35">
                Evidence limits
              </p>
              <ul className="mt-3 space-y-2 text-sm leading-6 text-white/45">
                {report.evidenceLimitations.map((limit) => (
                  <li key={limit}>{limit}</li>
                ))}
              </ul>
            </div>
          </div>
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
  const [error, setError] = useState("");
  const [report, setReport] = useState<AgentOpportunityReport | null>(null);

  const canSubmit = url.trim().length > 0 && !loading;

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!canSubmit) return;

    setLoading(true);
    setError("");
    setReport(null);

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
      setLoading(false);
    }
  }

  return (
    <>
      <section className="px-6 pt-16 sm:px-12 sm:pt-24">
        <div className="mx-auto grid max-w-[1180px] gap-12 lg:grid-cols-[0.92fr_1.08fr] lg:items-start">
          <div>
            <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.16em] text-white/40">
              AI agent opportunity scanner
            </p>
            <h1 className="mt-6 max-w-[13ch] text-[clamp(2.75rem,7vw,5.8rem)] font-sans font-semibold leading-[0.98] tracking-tight text-white">
              Should your company have an AI agent?
            </h1>
            <p className="mt-7 max-w-[58ch] text-lg leading-8 text-white/60">
              Get an honest URL-based report on where agents could save time,
              where they would fail, and what AgentClash should test before
              anyone ships one to customers.
            </p>
            <div className="mt-9 grid gap-3 border-t border-white/[0.08] pt-6 text-sm leading-6 text-white/55 sm:grid-cols-3">
              <p>Conservative savings ranges</p>
              <p>Risk and eval requirements</p>
              <p>A real not-yet verdict</p>
            </div>
          </div>

          <form
            onSubmit={onSubmit}
            className="rounded-md border border-white/[0.1] bg-[#0a0a0a] p-5 shadow-2xl shadow-black/30 sm:p-6"
          >
            <label className="block text-sm font-medium text-white" htmlFor="url">
              Company URL
            </label>
            <div className="mt-3 flex flex-col gap-3 sm:flex-row">
              <input
                id="url"
                value={url}
                onChange={(event) => setUrl(event.target.value)}
                placeholder="https://example.com"
                className="min-h-11 flex-1 rounded-md border border-white/[0.1] bg-white/[0.04] px-3 text-sm text-white outline-none transition-colors placeholder:text-white/25 focus:border-white/30"
              />
              <button
                type="submit"
                disabled={!canSubmit}
                className="inline-flex min-h-11 items-center justify-center gap-2 rounded-md bg-white px-4 text-sm font-medium text-[#060606] transition-colors hover:bg-white/90 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {loading ? <Loader2 className="size-4 animate-spin" /> : null}
                Analyze
              </button>
            </div>

            <div className="mt-5 grid gap-3 sm:grid-cols-3">
              <label className="block">
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
              <label className="block">
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
              <label className="block">
                <span className="text-xs text-white/45">Support volume</span>
                <select
                  value={monthlySupportVolume}
                  onChange={(event) =>
                    setMonthlySupportVolume(event.target.value)
                  }
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
              <p className="mt-5 rounded-md border border-amber-300/20 bg-amber-300/10 p-3 text-sm leading-6 text-amber-100">
                {error}
              </p>
            ) : null}

            <div className="mt-6 grid gap-px overflow-hidden rounded-md border border-white/[0.08] bg-white/[0.08] sm:grid-cols-3">
              {[
                ["1", "Find the repeatable workflow"],
                ["2", "Estimate ROI and failure risk"],
                ["3", "Generate the eval pack"],
              ].map(([step, label]) => (
                <div key={step} className="bg-[#0a0a0a] p-4">
                  <p className="font-[family-name:var(--font-mono)] text-[11px] text-white/35">
                    {step}
                  </p>
                  <p className="mt-3 text-sm leading-5 text-white/60">{label}</p>
                </div>
              ))}
            </div>
          </form>
        </div>
      </section>

      <div className="px-6 pb-20 sm:px-12">
        <div className="mx-auto max-w-[1180px]">
          {report ? <ReportPanel report={report} /> : null}
        </div>
      </div>
    </>
  );
}
