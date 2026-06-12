"use client";

import { FormEvent, useEffect, useMemo, useState } from "react";
import {
  ArrowRight,
  ChevronDown,
  Download,
  Loader2,
  Search,
} from "lucide-react";
import { cn } from "@/lib/utils";
import type { AgentOpportunityReport } from "@/lib/agent-opportunity";
import { DimensionRadar } from "./components/dimension-radar";
import { OpportunityMap } from "./components/opportunity-map";
import { RiskHeatmap } from "./components/risk-heatmap";
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

type UseCase = AgentOpportunityReport["useCases"][number];
type Tone = "good" | "warn" | "bad" | "neutral";

const SERIF = "[font-family:var(--font-race-display)]";
const MICRO = "font-mono text-[11px] uppercase tracking-[0.18em]";

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

const VERDICT_ORDER: AgentOpportunityReport["shouldBuildAgent"][] = [
  "not_yet",
  "eval_first",
  "narrow_pilot",
  "strong_fit",
];

const toneDot: Record<Tone, string> = {
  good: "bg-emerald-400",
  warn: "bg-amber-300",
  bad: "bg-red-400",
  neutral: "bg-white",
};

const toneSegment: Record<Tone, string> = {
  good: "bg-emerald-400",
  warn: "bg-amber-300",
  bad: "bg-red-400",
  neutral: "bg-white",
};

const ADVERSARIAL_FILL = "bg-white/30";

function levelCount(level: UseCase["fit"]): number {
  return level === "high" ? 3 : level === "medium" ? 2 : 1;
}

function reportHostname(analyzedUrl: string): string {
  try {
    return new URL(analyzedUrl).hostname;
  } catch {
    return analyzedUrl;
  }
}

function useEntranceFrame(): boolean {
  const [mounted, setMounted] = useState(false);
  useEffect(() => {
    const frame = requestAnimationFrame(() => setMounted(true));
    return () => cancelAnimationFrame(frame);
  }, []);
  return mounted;
}

function useCountUp(target: number, durationMs = 900): number {
  const [value, setValue] = useState(0);
  useEffect(() => {
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
      const frame = requestAnimationFrame(() => setValue(target));
      return () => cancelAnimationFrame(frame);
    }
    let frame: number;
    const start = performance.now();
    const tick = (now: number) => {
      const progress = Math.min(1, (now - start) / durationMs);
      const eased = 1 - Math.pow(1 - progress, 3);
      setValue(Math.round(target * eased));
      if (progress < 1) frame = requestAnimationFrame(tick);
    };
    frame = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(frame);
  }, [target, durationMs]);
  return value;
}

function ScaleBar({ value, className }: { value: number; className?: string }) {
  const mounted = useEntranceFrame();
  return (
    <div
      className={cn(
        "relative h-1.5 w-full overflow-hidden rounded-full bg-white/[0.08]",
        className,
      )}
      aria-hidden
    >
      <div
        className="h-full rounded-full bg-white transition-[width] duration-700 ease-out motion-reduce:transition-none"
        style={{
          width: mounted ? `${Math.max(2, Math.min(100, value))}%` : "0%",
        }}
      />
      {[25, 50, 75].map((tick) => (
        <span
          key={tick}
          className="absolute top-0 h-full w-px bg-[#080808]"
          style={{ left: `${tick}%` }}
        />
      ))}
    </div>
  );
}

const meterToneClass: Record<Tone, string> = {
  good: "bg-emerald-400/90",
  warn: "bg-amber-300/90",
  bad: "bg-red-400/90",
  neutral: "bg-cyan-400/90",
};

function meterTone(level: UseCase["fit"], invert: boolean): Tone {
  const rank = levelCount(level);
  const goodness = invert ? 4 - rank : rank;
  return goodness === 3 ? "good" : goodness === 2 ? "warn" : "bad";
}

function LevelMeter({
  level,
  label,
  invert = false,
}: {
  level: UseCase["fit"];
  label: string;
  invert?: boolean;
}) {
  const count = levelCount(level);
  const fill = meterToneClass[meterTone(level, invert)];
  return (
    <span className="flex flex-col gap-1.5">
      <span className="flex gap-[3px]" aria-hidden>
        {[0, 1, 2].map((index) => (
          <span
            key={index}
            className={cn(
              "h-[5px] w-4 rounded-[1px]",
              index < count ? fill : "bg-white/10",
            )}
          />
        ))}
      </span>
      <span className="font-mono text-[10px] uppercase tracking-[0.1em] text-white/55">
        {level === "medium" ? "med" : level} {label}
      </span>
    </span>
  );
}

function StatCell({
  label,
  children,
  caption,
}: {
  label: string;
  children: React.ReactNode;
  caption: string;
}) {
  return (
    <div className="flex min-w-0 flex-col bg-[#080808] p-5 sm:px-6">
      <p className={cn(MICRO, "text-white/60")}>{label}</p>
      <div className="mt-3 flex-1">{children}</div>
      <p className="mt-3 font-mono text-[11px] tracking-[0.04em] text-white/55">
        {caption}
      </p>
    </div>
  );
}

function UseCaseRow({ useCase, index }: { useCase: UseCase; index: number }) {
  return (
    <details className="group">
      <summary className="grid cursor-pointer list-none grid-cols-[2.25rem_minmax(0,1fr)_auto] items-center gap-x-4 px-5 py-4 transition-colors marker:content-none hover:bg-white/[0.03] sm:grid-cols-[2.5rem_minmax(0,1fr)_5.5rem_6.5rem_7rem_7rem] sm:px-7 [&::-webkit-details-marker]:hidden">
        <span
          className={cn(SERIF, "text-2xl leading-none text-white/30")}
          aria-hidden
        >
          {String(index + 1).padStart(2, "0")}
        </span>
        <span className="flex min-w-0 items-center gap-2">
          <span className="truncate text-sm font-medium text-white">
            {useCase.title}
          </span>
          <ChevronDown className="size-3.5 shrink-0 text-white/40 transition-transform group-open:rotate-180" />
        </span>
        <span className="hidden sm:block">
          <LevelMeter level={useCase.fit} label="fit" />
        </span>
        <span className="hidden sm:block">
          <LevelMeter level={useCase.complexity} label="cmplx" invert />
        </span>
        <span className="hidden text-right font-mono text-xs tabular-nums text-white/75 sm:block">
          {useCase.estimatedMonthlyHoursSaved}
        </span>
        <span className="text-right font-mono text-xs tabular-nums text-white">
          {useCase.estimatedMonthlySavingsUsd}
        </span>
      </summary>
      <div className="grid gap-6 px-5 pb-5 pt-1 sm:grid-cols-2 sm:px-7 sm:pl-[4.25rem]">
        <div>
          <div className="mb-3 flex items-end gap-5 sm:hidden">
            <LevelMeter level={useCase.fit} label="fit" />
            <LevelMeter level={useCase.complexity} label="cmplx" invert />
            <span className="font-mono text-xs text-white/75">
              {useCase.estimatedMonthlyHoursSaved}
            </span>
          </div>
          <p className="text-[13px] leading-6 text-white/70">
            {useCase.workflow}
          </p>
          <p className="mt-2 text-[13px] leading-6 text-white/55">
            {useCase.why}
          </p>
        </div>
        <div>
          <p className={cn(MICRO, "tracking-[0.14em] text-white/55")}>
            First eval tasks
          </p>
          <ul className="mt-2.5 space-y-1.5">
            {useCase.firstEvalTasks.map((task) => (
              <li
                key={task}
                className="flex gap-2 text-[13px] leading-5 text-white/70"
              >
                <span className="font-mono text-white/40">·</span>
                {task}
              </li>
            ))}
          </ul>
        </div>
      </div>
    </details>
  );
}

export function ReportDashboard({
  report,
}: {
  report: AgentOpportunityReport;
}) {
  const metrics = useMemo(() => deriveOpportunityMetrics(report), [report]);
  const tone = fitTone[report.shouldBuildAgent];
  const animatedScore = useCountUp(report.agentFitScore);
  const highFitCount = report.useCases.filter(
    (useCase) => useCase.fit === "high",
  ).length;
  const highRiskCount = report.risks.filter(
    (risk) => risk.severity === "high",
  ).length;
  const totalEvalCases =
    report.evaluationPack.recommendedCases +
    report.evaluationPack.adversarialCases;

  return (
    <section className="mt-10 border border-white/[0.08] bg-[#080808]">
      <header className="border-b border-white/[0.08] px-5 py-5 sm:px-7">
        <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
          <p className={cn(MICRO, "text-white/60")}>Agent opportunity report</p>
          <p className="font-mono text-[11px] tracking-[0.06em] text-white/50">
            {reportHostname(report.analyzedUrl)}
          </p>
        </div>
        <div className="mt-4 flex flex-wrap items-end justify-between gap-4">
          <div className="min-w-0">
            <h2
              className={cn(
                SERIF,
                "text-3xl tracking-tight text-white sm:text-4xl",
              )}
            >
              {report.companyName}
            </h2>
            <p className="mt-2 max-w-[68ch] text-sm leading-6 text-white/65">
              {report.summary}
            </p>
          </div>
          <button
            type="button"
            onClick={() => window.print()}
            className="inline-flex items-center gap-2 border border-white/[0.15] px-3 py-2 font-mono text-[10px] uppercase tracking-[0.12em] text-white/70 transition-colors hover:border-white/35 hover:text-white print:hidden"
          >
            <Download className="size-3.5" />
            Save report
          </button>
        </div>
      </header>

      <div className="grid grid-cols-2 gap-px border-b border-white/[0.08] bg-white/[0.06] xl:grid-cols-4">
        <StatCell
          label="Agent fit"
          caption={confidenceCopy[report.fitLevel].toLowerCase()}
        >
          <p className="font-mono text-[40px] leading-none tabular-nums text-white">
            {animatedScore}
            <span className="ml-1 text-sm text-white/50">/100</span>
          </p>
          <ScaleBar value={report.agentFitScore} className="mt-3" />
        </StatCell>

        <StatCell
          label="Verdict"
          caption={`stage ${
            VERDICT_ORDER.indexOf(report.shouldBuildAgent) + 1
          } of 4`}
        >
          <p className="flex items-center gap-2 text-[22px] font-medium leading-none tracking-tight text-white sm:text-[26px]">
            <span
              className={cn("size-2 shrink-0 rounded-full", toneDot[tone])}
              aria-hidden
            />
            {fitCopy[report.shouldBuildAgent]}
          </p>
          <div className="mt-4 flex gap-1" aria-hidden>
            {VERDICT_ORDER.map((verdict) => (
              <span
                key={verdict}
                className={cn(
                  "h-1 flex-1 rounded-full",
                  verdict === report.shouldBuildAgent
                    ? toneSegment[tone]
                    : "bg-white/10",
                )}
              />
            ))}
          </div>
        </StatCell>

        <StatCell label="Use cases" caption={`${highFitCount} high fit`}>
          <p className="font-mono text-[40px] leading-none tabular-nums text-white">
            {report.useCases.length}
          </p>
          <div className="mt-3 flex gap-1.5" aria-hidden>
            {report.useCases.map((useCase) => (
              <span
                key={useCase.title}
                className={cn(
                  "size-2.5 rounded-full",
                  useCase.fit === "high"
                    ? "bg-emerald-400"
                    : useCase.fit === "medium"
                      ? "bg-amber-300"
                      : "bg-red-400/60",
                )}
              />
            ))}
          </div>
        </StatCell>

        <StatCell label="Risks" caption={`${highRiskCount} high severity`}>
          <p className="font-mono text-[40px] leading-none tabular-nums text-white">
            {report.risks.length}
          </p>
          <div className="mt-3 flex gap-1" aria-hidden>
            {report.risks.map((risk) => (
              <span
                key={risk.risk}
                className={cn(
                  "h-2.5 w-5 rounded-[2px]",
                  risk.severity === "high"
                    ? "bg-red-400"
                    : risk.severity === "medium"
                      ? "bg-amber-300"
                      : "bg-emerald-400",
                )}
              />
            ))}
          </div>
        </StatCell>
      </div>

      <div className="grid gap-px border-b border-white/[0.08] bg-white/[0.06] lg:grid-cols-[0.9fr_1.1fr]">
        <div className="bg-[#080808] p-5 sm:p-6">
          <div className="flex items-baseline justify-between gap-3">
            <p className={cn(MICRO, "text-white/60")}>Dimension profile</p>
            <p className="font-mono text-[10px] tracking-[0.08em] text-white/45">
              0–100
            </p>
          </div>
          <DimensionRadar
            metrics={metrics}
            className="mx-auto mt-2 max-w-[340px]"
          />
        </div>
        <div className="bg-[#080808] p-5 sm:p-6">
          <div className="flex items-baseline justify-between gap-3">
            <p className={cn(MICRO, "text-white/60")}>Opportunity map</p>
            <p className="font-mono text-[10px] tracking-[0.08em] text-white/45">
              fit × complexity
            </p>
          </div>
          <OpportunityMap useCases={report.useCases} className="mt-4" />
        </div>
      </div>

      <div className="border-b border-white/[0.08]">
        <div className="flex items-baseline justify-between gap-3 px-5 pt-5 sm:px-7">
          <p className={cn(MICRO, "text-white/60")}>Use cases</p>
          <p className="hidden font-mono text-[10px] tracking-[0.08em] text-white/45 sm:block">
            hours · savings / month
          </p>
        </div>
        <div className="mt-2 divide-y divide-white/[0.05]">
          {report.useCases.map((useCase, index) => (
            <UseCaseRow key={useCase.title} useCase={useCase} index={index} />
          ))}
        </div>
      </div>

      <div className="grid gap-px border-b border-white/[0.08] bg-white/[0.06] lg:grid-cols-2">
        <div className="bg-[#080808] p-5 sm:p-6">
          <div className="flex items-baseline justify-between gap-3">
            <p className={cn(MICRO, "text-white/60")}>Risk heatmap</p>
            <p className="font-mono text-[10px] tracking-[0.08em] text-white/45">
              {highRiskCount} high · {report.risks.length} total
            </p>
          </div>
          <RiskHeatmap risks={report.risks} className="mt-4" />
        </div>

        <div className="flex flex-col bg-[#080808] p-5 sm:p-6">
          <div className="flex items-baseline justify-between gap-3">
            <p className={cn(MICRO, "text-white/60")}>Eval plan</p>
            <p className="font-mono text-[10px] tracking-[0.08em] text-white/45">
              {totalEvalCases} cases
            </p>
          </div>
          <p className="mt-4 text-sm font-medium text-white">
            {report.evaluationPack.name}
          </p>
          <div
            className="mt-3 flex h-3 w-full overflow-hidden rounded-full bg-white/[0.06]"
            aria-hidden
          >
            <div
              className="bg-white"
              style={{
                width: `${
                  (report.evaluationPack.recommendedCases /
                    Math.max(1, totalEvalCases)) *
                  100
                }%`,
              }}
            />
            <div className={cn("flex-1", ADVERSARIAL_FILL)} />
          </div>
          <div className="mt-2 flex flex-wrap gap-x-5 gap-y-1 font-mono text-[11px] tracking-[0.04em] text-white/65">
            <span className="flex items-center gap-1.5">
              <span className="size-2 rounded-[1px] bg-white" aria-hidden />
              {report.evaluationPack.recommendedCases} realistic
            </span>
            <span className="flex items-center gap-1.5">
              <span className={cn("size-2 rounded-[1px]", ADVERSARIAL_FILL)} aria-hidden />
              {report.evaluationPack.adversarialCases} adversarial
            </span>
          </div>
          <ul className="mt-4 space-y-1.5">
            {report.evaluationPack.successCriteria.map((criterion) => (
              <li
                key={criterion}
                className="flex gap-2 text-[13px] leading-5 text-white/75"
              >
                <span className="font-mono text-white/60">✓</span>
                {criterion}
              </li>
            ))}
          </ul>
          <a
            href="/auth/login"
            className="mt-5 inline-flex w-fit items-center gap-2 bg-white px-3.5 py-2 text-sm font-medium text-[#060606] transition-colors hover:bg-white/90 lg:mt-auto"
          >
            Create eval workspace
            <ArrowRight className="size-4" />
          </a>
        </div>
      </div>

      <div className="grid gap-px bg-white/[0.06] lg:grid-cols-[1.1fr_0.9fr]">
        <div className="bg-[#080808] p-5 sm:p-7">
          <p className={cn(MICRO, "text-white/60")}>The verdict</p>
          <p className="mt-3 text-[15px] leading-7 text-white/80">
            {report.honestVerdict}
          </p>
          <div className="mt-5 border-t border-white/[0.08] pt-3">
            {report.evidenceLimitations.map((limit, index) => (
              <p
                key={limit}
                className="mt-1 text-xs leading-5 text-white/50"
              >
                <span className="font-mono">{index + 1}.</span> {limit}
              </p>
            ))}
          </div>
        </div>
        <div className="bg-[#080808] p-5 sm:p-7">
          <p className={cn(MICRO, "text-white/60")}>Next steps</p>
          <ol className="mt-3 space-y-3">
            {report.nextSteps.map((step, index) => (
              <li key={step} className="flex gap-3">
                <span
                  className={cn(SERIF, "text-xl leading-6 text-white/30")}
                  aria-hidden
                >
                  {String(index + 1).padStart(2, "0")}
                </span>
                <span className="text-sm leading-6 text-white/80">{step}</span>
              </li>
            ))}
          </ol>
        </div>
      </div>
    </section>
  );
}

function LoadingPanel({ step }: { step: number }) {
  return (
    <section className="mt-10 border border-white/[0.08] bg-[#080808] p-6">
      <div className="flex items-center gap-3">
        <Loader2 className="size-5 animate-spin text-white/70" />
        <div>
          <p className="text-sm font-medium text-white">
            Generating your report
          </p>
          <p className="mt-1 text-xs text-white/60">
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
              className={cn(
                "flex items-center gap-3 text-sm",
                done
                  ? "text-white/75"
                  : active
                    ? "text-white"
                    : "text-white/45",
              )}
            >
              <span
                className={cn(
                  "flex size-6 items-center justify-center border font-mono text-[11px]",
                  done
                    ? "border-white/30 bg-white/10 text-white"
                    : active
                      ? "border-white/40 text-white"
                      : "border-white/15",
                )}
              >
                {done ? "✓" : index + 1}
              </span>
              {label}
              {active ? (
                <span
                  className="size-1.5 animate-pulse rounded-full bg-white/70"
                  aria-hidden
                />
              ) : null}
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
