"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import type { FormEvent } from "react";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Activity,
  ArrowRight,
  ArrowUp,
  Calculator,
  CheckCircle2,
  ChevronDown,
  Download,
  FileText,
  Gauge,
  Loader2,
  Lock,
  ShieldAlert,
  TrendingUp,
  XCircle,
} from "lucide-react";

import {
  createAnonymousAgentTryout,
  getPublicAgentTryout,
  getPublicAgentTryoutEvents,
  listAgentTryoutTemplates,
  submitAgentTryoutTurn,
} from "@/lib/api/agent-tryouts";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type {
  AgentHarnessKind,
  AgentTryout,
  AgentTryoutTemplate,
  TryoutTimelineEvent,
} from "@/lib/api/types";
import {
  formatTryoutCost,
  formatTryoutLatency,
  tryoutIsActive,
  tryoutStatusVariant,
} from "@/lib/agent-tryout-status";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

type FieldSpec = {
  type: string;
  minimum?: number;
  maximum?: number;
};

type AgentOption = {
  value: "" | AgentHarnessKind;
  label: string;
  hint: string;
};

const AGENT_OPTIONS: AgentOption[] = [
  { value: "", label: "Auto", hint: "Hosted default agent" },
  { value: "codex_e2b", label: "Codex", hint: "OpenAI Codex CLI" },
  { value: "claude_e2b", label: "Claude", hint: "Anthropic Claude Code" },
  { value: "openclaw_e2b", label: "OpenClaw", hint: "OpenRouter-routed" },
];

function agentLabel(value: string): string {
  return AGENT_OPTIONS.find((option) => option.value === value)?.label ?? "Auto";
}

// Starter prompts shown under the composer so people know what to throw at the
// agent. Many lean on Python (PDFs, charts, spreadsheets) on purpose — that's
// where agents shine and where the sandbox now has the full toolchain.
const TASK_SUGGESTIONS: Record<string, string[]> = {
  "support-ticket-resolution": [
    "Customer says their invoice was charged twice and wants a refund today. Draft a reply and decide whether to escalate.",
    "Angry customer threatening to churn over downtime. Reply empathetically, cite our SLA, flag for a human.",
  ],
  "document-extraction": [
    "Extract line items, totals, and vendor from this invoice, then render a clean summary PDF.",
    "Pull every date, amount, and party from this contract into a spreadsheet.",
  ],
  "contract-review": [
    "Review this NDA for one-sided indemnity and unlimited liability. List risks with severity.",
    "Compare these payment terms against net-30 standard and propose redlines.",
  ],
  "sdr-outreach": [
    "Qualify this prospect for our eval platform and draft a 3-sentence cold email.",
    "Write a follow-up to a VP Eng who opened but didn't reply, referencing their hiring spike.",
  ],
  "spreadsheet-builder": [
    "Turn this raw sales data into a spreadsheet with a pivot summary and a bar chart PNG.",
    "Build a 12-month cashflow model from these assumptions and chart the runway.",
  ],
  "slide-deck": [
    "Make a 6-slide investor update from these metrics, with a revenue trend chart.",
    "Turn this product brief into a deck outline with speaker notes.",
  ],
  "status-report": [
    "Turn these scattered updates into a polished weekly status report and export it as a PDF.",
    "Summarize this sprint into highlights, risks, and next steps.",
  ],
  "meeting-minutes": [
    "Summarize these notes into minutes with owners and due dates.",
    "Extract action items and render them as a one-page PDF checklist.",
  ],
  "inbox-triage": [
    "Prioritize these 8 emails and draft replies for the urgent ones.",
  ],
};

const GENERIC_SUGGESTIONS = [
  "Generate a PDF report from this data with a chart.",
  "Build a spreadsheet with formulas and a summary tab.",
  "Turn this into a labeled bar chart and explain the trend.",
];

function suggestionsFor(slug: string): string[] {
  return TASK_SUGGESTIONS[slug] ?? GENERIC_SUGGESTIONS;
}

const api = createApiClient();

export function PublicTryoutsClient() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const urlTryoutId = searchParams.get("tryout") ?? "";

  const [templates, setTemplates] = useState<AgentTryoutTemplate[]>([]);
  const [templateSlug, setTemplateSlug] = useState("");
  const [agent, setAgent] = useState<"" | AgentHarnessKind>("");
  const [fieldValues, setFieldValues] = useState<Record<string, string>>({});
  const [templatesLoading, setTemplatesLoading] = useState(true);
  const [launching, setLaunching] = useState(false);
  const [tryout, setTryout] = useState<AgentTryout | null>(null);
  const [events, setEvents] = useState<TryoutTimelineEvent[]>([]);
  const [tryoutLoading, setTryoutLoading] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [quotaMessage, setQuotaMessage] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    async function loadTemplates() {
      setTemplatesLoading(true);
      setMessage(null);
      try {
        const res = await listAgentTryoutTemplates(api);
        const publicTemplates = res.items.filter(
          (template) => template.available && template.anonymous_enabled,
        );
        if (cancelled) return;
        setTemplates(publicTemplates);
        setTemplateSlug((current) => current || publicTemplates[0]?.slug || "");
      } catch (err) {
        if (!cancelled) {
          setMessage(
            err instanceof ApiError
              ? err.message
              : "Could not load public tryout tasks.",
          );
        }
      } finally {
        if (!cancelled) setTemplatesLoading(false);
      }
    }
    void loadTemplates();
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    setFieldValues({});
  }, [templateSlug]);

  useEffect(() => {
    if (!urlTryoutId) {
      setTryout(null);
      setEvents([]);
      setQuotaMessage(null);
      return;
    }

    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let nextCursor = 0;
    const seen = new Set<number>();

    async function pollTryout() {
      setTryoutLoading(true);
      try {
        const [nextTryout, page] = await Promise.all([
          getPublicAgentTryout(api, urlTryoutId),
          getPublicAgentTryoutEvents(api, urlTryoutId, {
            after: nextCursor,
            limit: 200,
          }),
        ]);
        if (cancelled) return;

        setTryout(nextTryout);
        setEvents((current) => {
          const fresh = page.events.filter((event) => {
            if (seen.has(event.cursor)) return false;
            seen.add(event.cursor);
            return true;
          });
          return fresh.length > 0 ? [...current, ...fresh] : current;
        });
        if (page.next_cursor > nextCursor) {
          nextCursor = page.next_cursor;
        }
        if (tryoutIsActive(nextTryout.status)) {
          timer = setTimeout(pollTryout, 2000);
        }
      } catch (err) {
        if (!cancelled) {
          setMessage(
            err instanceof ApiError
              ? err.message
              : "Could not load this public tryout.",
          );
        }
      } finally {
        if (!cancelled) setTryoutLoading(false);
      }
    }

    setEvents([]);
    setMessage(null);
    void pollTryout();
    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
    };
  }, [urlTryoutId]);

  const template = useMemo(
    () => templates.find((item) => item.slug === templateSlug) ?? null,
    [templates, templateSlug],
  );

  const fields = useMemo(() => getFields(template), [template]);
  const required = useMemo(
    () => new Set(template?.input_schema.required ?? []),
    [template],
  );
  const primaryField = useMemo(
    () =>
      fields.find(([field, spec]) => required.has(field) && spec.type === "string") ??
      fields.find(([, spec]) => spec.type === "string") ??
      null,
    [fields, required],
  );
  const secondaryFields = fields.filter(([field]) => field !== primaryField?.[0]);

  const updateField = useCallback((field: string, value: string) => {
    setFieldValues((current) => ({ ...current, [field]: value }));
  }, []);

  async function handleLaunch(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!template || launching) return;

    const input = buildInput(fields, required, fieldValues);
    if ("error" in input) {
      setMessage(input.error);
      return;
    }

    setLaunching(true);
    setMessage(null);
    setQuotaMessage(null);
    try {
      const nextTryout = await createAnonymousAgentTryout(api, {
        template_slug: template.slug,
        input: input.value,
        ...(agent ? { selected_harness_kind: agent } : {}),
      });
      setTryout(nextTryout);
      setEvents([]);
      router.replace(`/tryouts?tryout=${encodeURIComponent(nextTryout.id)}`, {
        scroll: false,
      });
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 402 || err.status === 429) {
          setQuotaMessage(err.message);
        } else {
          setMessage(err.message);
        }
      } else {
        setMessage("Could not launch this tryout. Please try again.");
      }
    } finally {
      setLaunching(false);
    }
  }

  const loginHref = `/auth/login?mode=signup&returnTo=${encodeURIComponent(
    urlTryoutId ? `/tryouts?tryout=${urlTryoutId}` : "/tryouts",
  )}`;

  const primaryValue = primaryField ? fieldValues[primaryField[0]] ?? "" : "";
  const canRun = Boolean(template) && !templatesLoading && !launching;

  return (
    <main className="min-h-screen overflow-hidden bg-[#0c0b0a] text-[#f4efe6]">
      <div className="pointer-events-none fixed inset-0 bg-[radial-gradient(circle_at_50%_-10%,rgba(216,161,93,0.16),transparent_42%)]" />
      <div className="relative mx-auto flex min-h-screen w-full max-w-5xl flex-col px-4 py-5 sm:px-6">
        <header className="flex items-center justify-between gap-4">
          <Link href="/" className="text-sm font-semibold tracking-tight text-[#f4efe6]/90">
            AgentClash
          </Link>
          <div className="flex items-center gap-3">
            <Link
              href="/pricing"
              className="hidden text-sm text-[#f4efe6]/55 transition hover:text-[#f4efe6] sm:inline"
            >
              For teams
            </Link>
            <Link
              href={loginHref}
              className="rounded-full border border-[#f4efe6]/15 px-3 py-1.5 text-sm text-[#f4efe6]/80 transition hover:border-[#f4efe6]/30 hover:text-[#f4efe6]"
            >
              Sign in
            </Link>
          </div>
        </header>

        <section className="mx-auto flex w-full max-w-3xl flex-1 flex-col items-center justify-center py-10 sm:py-14">
          <h1 className="text-center text-[clamp(2.8rem,9vw,5rem)] font-semibold leading-none tracking-tight">
            agentclash
          </h1>
          <p className="mt-4 text-center text-base text-[#f4efe6]/55">
            Hand a real AI agent your office work. Pick a task, pick an agent, watch it run.
          </p>

          <form onSubmit={handleLaunch} className="mt-8 w-full">
            <div className="rounded-[28px] border border-[#f4efe6]/12 bg-[#16140f]/80 p-2 shadow-2xl shadow-black/40 backdrop-blur transition focus-within:border-[#d8a15d]/40">
              <textarea
                value={primaryValue}
                onChange={(event) =>
                  primaryField && updateField(primaryField[0], event.target.value)
                }
                rows={3}
                disabled={!template}
                placeholder={
                  template
                    ? `Paste the work for "${template.name}" — notes, brief, data, or context.`
                    : "Loading tasks…"
                }
                className="block w-full resize-none bg-transparent px-4 pt-3 text-[15px] leading-7 text-[#f4efe6] outline-none placeholder:text-[#f4efe6]/30"
              />

              {secondaryFields.length > 0 ? (
                <div className="grid gap-2 px-2 pb-1 sm:grid-cols-2">
                  {secondaryFields.map(([field, spec]) => (
                    <CompactField
                      key={field}
                      field={field}
                      spec={spec}
                      value={fieldValues[field] ?? ""}
                      required={required.has(field)}
                      onChange={updateField}
                    />
                  ))}
                </div>
              ) : null}

              <div className="flex items-center justify-between gap-2 px-1.5 pb-1.5 pt-1.5">
                <div className="flex flex-wrap items-center gap-2">
                  <PillSelect
                    icon={<FileText className="size-3.5" />}
                    value={templateSlug}
                    onChange={setTemplateSlug}
                    disabled={templatesLoading}
                    options={templates.map((t) => ({ value: t.slug, label: t.name }))}
                  />
                  <PillSelect
                    icon={<Gauge className="size-3.5" />}
                    value={agent}
                    onChange={(value) => setAgent(value as "" | AgentHarnessKind)}
                    options={AGENT_OPTIONS.map((option) => ({
                      value: option.value,
                      label: option.label,
                    }))}
                  />
                </div>
                <button
                  type="submit"
                  disabled={!canRun}
                  aria-label="Run public tryout"
                  className="flex size-9 shrink-0 items-center justify-center rounded-full bg-[#e7c18d] text-[#14120f] transition hover:bg-[#f0cf9d] disabled:cursor-not-allowed disabled:opacity-40"
                >
                  {launching ? (
                    <Loader2 className="size-4 animate-spin" />
                  ) : (
                    <ArrowUp className="size-4" />
                  )}
                </button>
              </div>
            </div>

            {message ? (
              <div className="mt-3 rounded-2xl border border-red-400/25 bg-red-400/10 p-3 text-sm text-red-100">
                {message}
              </div>
            ) : null}
            {quotaMessage ? (
              <div className="mt-3 rounded-2xl border border-[#d8a15d]/25 bg-[#d8a15d]/10 p-3 text-sm text-[#f2d6ad]">
                <p>{quotaMessage}</p>
                <Link
                  href={loginHref}
                  className="mt-2 inline-flex items-center gap-1 font-medium text-[#f4efe6] hover:underline"
                >
                  Save this tryout in a workspace
                  <ArrowRight className="size-3.5" />
                </Link>
              </div>
            ) : null}

            {template ? (
              <p className="mt-3 text-center text-xs text-[#f4efe6]/40">
                {template.description} · runs on{" "}
                <span className="text-[#f4efe6]/70">{agentLabel(agent)}</span> · hosted
                cost capped at {`$${template.max_cost_usd.toFixed(2)}`}.
              </p>
            ) : null}
          </form>

          {template && primaryField ? (
            <div className="mt-5 w-full">
              <p className="mb-2 text-center text-xs uppercase tracking-[0.14em] text-[#f4efe6]/35">
                Try one of these
              </p>
              <div className="flex flex-wrap justify-center gap-2">
                {suggestionsFor(template.slug).map((suggestion) => (
                  <button
                    key={suggestion}
                    type="button"
                    onClick={() => updateField(primaryField[0], suggestion)}
                    className="rounded-full border border-[#f4efe6]/12 bg-[#f4efe6]/[0.04] px-3 py-1.5 text-left text-xs text-[#f4efe6]/70 transition hover:border-[#d8a15d]/40 hover:text-[#f4efe6]"
                  >
                    {suggestion.length > 70 ? suggestion.slice(0, 70) + "…" : suggestion}
                  </button>
                ))}
              </div>
            </div>
          ) : null}

          <div className="mt-7 grid w-full gap-3 sm:grid-cols-2">
            <GradientCard
              icon={<FileText className="size-4" />}
              title="Real office tasks"
              text="Meeting minutes, slide decks, spreadsheet extraction, inbox triage."
              tone="amber"
            />
            <GradientCard
              icon={<Gauge className="size-4" />}
              title="Any agent, scored"
              text="Codex, Claude, OpenClaw or Hermes — with a trace and scorecard."
              tone="teal"
            />
          </div>
        </section>

        <TryoutPanel
          tryout={tryout}
          events={events}
          loading={tryoutLoading}
          loginHref={loginHref}
        />
      </div>
    </main>
  );
}

function PillSelect({
  icon,
  value,
  onChange,
  options,
  disabled,
}: {
  icon: React.ReactNode;
  value: string;
  onChange: (value: string) => void;
  options: { value: string; label: string }[];
  disabled?: boolean;
}) {
  return (
    <div className="relative inline-flex items-center gap-1.5 rounded-full border border-[#f4efe6]/12 bg-[#f4efe6]/[0.04] py-1.5 pl-3 pr-7 text-sm text-[#f4efe6]/80 transition hover:border-[#f4efe6]/25">
      <span className="text-[#d8a15d]">{icon}</span>
      <span className="truncate">
        {options.find((option) => option.value === value)?.label ?? "Select"}
      </span>
      <ChevronDown className="pointer-events-none absolute right-2.5 size-3.5 text-[#f4efe6]/40" />
      <select
        value={value}
        disabled={disabled}
        onChange={(event) => onChange(event.target.value)}
        className="absolute inset-0 cursor-pointer opacity-0"
        aria-label="Select"
      >
        {options.map((option) => (
          <option key={option.value || "auto"} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    </div>
  );
}

function CompactField({
  field,
  spec,
  value,
  required,
  onChange,
}: {
  field: string;
  spec: FieldSpec;
  value: string;
  required: boolean;
  onChange: (field: string, value: string) => void;
}) {
  return (
    <label className="flex items-center gap-2 rounded-xl border border-[#f4efe6]/8 bg-[#0e0c0a]/55 px-3 py-1.5">
      <span className="shrink-0 text-xs text-[#f4efe6]/45">
        {fieldLabel(field)}
        {required ? "" : " (opt)"}
      </span>
      <input
        type={spec.type === "string" ? "text" : "number"}
        value={value}
        min={spec.minimum}
        max={spec.maximum}
        onChange={(event) => onChange(field, event.target.value)}
        className="w-full bg-transparent text-sm text-[#f4efe6] outline-none placeholder:text-[#f4efe6]/25"
      />
    </label>
  );
}

function GradientCard({
  icon,
  title,
  text,
  tone,
}: {
  icon: React.ReactNode;
  title: string;
  text: string;
  tone: "amber" | "teal";
}) {
  const toneClass =
    tone === "amber"
      ? "from-[#d8a15d]/18 to-[#d8a15d]/[0.04]"
      : "from-[#3aa6a0]/18 to-[#3aa6a0]/[0.04]";
  return (
    <div
      className={`rounded-2xl border border-[#f4efe6]/8 bg-gradient-to-br ${toneClass} p-4`}
    >
      <span className="text-[#f4efe6]/85">{icon}</span>
      <p className="mt-2.5 font-medium tracking-tight text-[#f4efe6]/90">{title}</p>
      <p className="mt-1 text-sm leading-6 text-[#f4efe6]/55">{text}</p>
    </div>
  );
}

function TryoutPanel({
  tryout,
  events,
  loading,
  loginHref,
}: {
  tryout: AgentTryout | null;
  events: TryoutTimelineEvent[];
  loading: boolean;
  loginHref: string;
}) {
  if (!tryout && !loading) {
    return (
      <section className="mx-auto mb-12 grid w-full max-w-3xl gap-3 rounded-[1.6rem] border border-[#f4efe6]/10 bg-[#f4efe6]/[0.03] p-4 text-sm text-[#f4efe6]/55 sm:grid-cols-3">
        <ProofItem icon={Lock} label="Task gated" text="Four hosted tasks per fingerprint by default." />
        <ProofItem icon={Activity} label="Trace + scorecard" text="Every run exposes a redacted event trail and a scorecard." />
        <ProofItem icon={FileText} label="Exportable" text="Download the artifact, trace, and scorecard." />
      </section>
    );
  }

  if (!tryout) {
    return (
      <section className="mx-auto mb-12 flex w-full max-w-3xl items-center justify-center rounded-[1.6rem] border border-[#f4efe6]/10 bg-[#f4efe6]/[0.03] p-8 text-sm text-[#f4efe6]/55">
        <Loader2 className="mr-2 size-4 animate-spin" />
        Loading tryout
      </section>
    );
  }

  const summary =
    typeof tryout.summary?.message === "string" ? tryout.summary.message : "";
  const outputs = tryoutOutputs(tryout.summary);
  const scorecard = tryoutScorecard(tryout.summary);
  const agentRan = tryout.selected_harness_kind
    ? agentLabel(tryout.selected_harness_kind)
    : "Auto";

  return (
    <section className="mx-auto mb-12 w-full max-w-3xl rounded-[1.6rem] border border-[#f4efe6]/10 bg-[#f4efe6]/[0.035] p-4 shadow-xl shadow-black/20 sm:p-5">
      <div className="flex flex-col gap-4 border-b border-[#f4efe6]/10 pb-4 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="text-xl font-semibold tracking-tight">Result</h2>
            <Badge variant={tryoutStatusVariant(tryout.status)}>{tryout.status}</Badge>
            <Badge variant="outline" className="border-[#d8a15d]/30 text-[#e7c18d]">
              {agentRan}
            </Badge>
          </div>
          <p className="mt-1 text-sm leading-6 text-[#f4efe6]/55">
            {summary || "The agent trace will appear here as the sandbox runs."}
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          {outputs.length > 0 ? (
            <DownloadButton
              label="Artifacts"
              onClick={() => downloadArtifacts(tryout, outputs)}
            />
          ) : null}
          <DownloadButton label="Traces" onClick={() => downloadTrace(tryout, events)} />
          {scorecard ? (
            <DownloadButton
              label="Scorecard"
              onClick={() => downloadScorecard(tryout, scorecard)}
            />
          ) : null}
          <Link
            href={loginHref}
            className="inline-flex h-8 items-center gap-1.5 rounded-full bg-[#e7c18d] px-3 text-sm font-medium text-[#14120f] transition hover:bg-[#f0cf9d]"
          >
            Save and rerun
            <ArrowRight className="size-4" />
          </Link>
        </div>
      </div>

      <dl className="mt-4 grid grid-cols-2 gap-3 text-sm sm:grid-cols-4">
        <Stat label="Task" value={tryout.template_slug} />
        <Stat label="Agent" value={agentRan} />
        <Stat label="Latency" value={formatTryoutLatency(tryout.latency_ms)} />
        <Stat label="Cost" value={formatTryoutCost(tryout)} />
      </dl>

      <TryoutChat tryout={tryout} events={events} />

      {scorecard ? <ScorecardCard scorecard={scorecard} /> : null}

      <div className="mt-5 grid gap-4 lg:grid-cols-[1.1fr_0.9fr]">
        <div>
          {outputs.length > 0 ? (
            <div className="mb-5">
              <h3 className="mb-2 text-sm font-semibold tracking-tight">Preview</h3>
              <div className="space-y-3">
                {outputs.map((output) => (
                  <article
                    key={`${output.key}-${output.relative_path}`}
                    className="rounded-2xl border border-[#f4efe6]/10 bg-[#0e0c0a]/55 p-4"
                  >
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <p className="text-sm font-medium text-[#f4efe6]/88">
                        {output.key || output.relative_path || "Output"}
                      </p>
                      <div className="flex items-center gap-2">
                        {output.relative_path ? (
                          <span className="text-xs text-[#f4efe6]/38">
                            {output.relative_path}
                          </span>
                        ) : null}
                        <button
                          type="button"
                          onClick={() => downloadOutput(output)}
                          className="text-[#f4efe6]/45 transition hover:text-[#f4efe6]"
                          aria-label="Download this artifact"
                        >
                          <Download className="size-3.5" />
                        </button>
                      </div>
                    </div>
                    <pre className="mt-3 max-h-72 overflow-auto whitespace-pre-wrap text-xs leading-5 text-[#f4efe6]/68">
                      {output.preview}
                    </pre>
                    {output.truncated ? (
                      <p className="mt-2 text-xs text-[#f4efe6]/38">
                        Preview truncated. Sign in to keep working with the full
                        artifact.
                      </p>
                    ) : null}
                  </article>
                ))}
              </div>
            </div>
          ) : null}
          <h3 className="mb-2 text-sm font-semibold tracking-tight">Traces</h3>
          {events.length === 0 ? (
            <div className="rounded-2xl border border-[#f4efe6]/10 bg-[#0e0c0a]/55 p-4 text-sm text-[#f4efe6]/50">
              {tryoutIsActive(tryout.status)
                ? "Waiting for the first sandbox event."
                : "No public timeline events were recorded."}
            </div>
          ) : (
            <ol className="overflow-hidden rounded-2xl border border-[#f4efe6]/10 bg-[#0e0c0a]/55">
              {events.map((event) => (
                <li
                  key={event.cursor}
                  className="flex gap-3 border-b border-[#f4efe6]/8 px-4 py-3 text-sm last:border-b-0"
                >
                  <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-[#d8a15d]" />
                  <div className="min-w-0 flex-1">
                    <p className="text-[#f4efe6]/82">{event.summary}</p>
                    <time className="mt-1 block text-xs text-[#f4efe6]/38">
                      {new Date(event.occurred_at).toLocaleTimeString()}
                    </time>
                  </div>
                </li>
              ))}
            </ol>
          )}
        </div>

        <div>
          <h3 className="mb-2 text-sm font-semibold tracking-tight">Input</h3>
          <pre className="max-h-80 overflow-auto rounded-2xl border border-[#f4efe6]/10 bg-[#0e0c0a]/55 p-4 text-xs leading-5 text-[#f4efe6]/62">
            {JSON.stringify(tryout.input_snapshot, null, 2)}
          </pre>
        </div>
      </div>

      <EvalRoiCalculator tryout={tryout} loginHref={loginHref} />
    </section>
  );
}

function ScorecardCard({ scorecard }: { scorecard: TryoutScorecard }) {
  const pct = Math.round(scorecard.score * 100);
  return (
    <div className="mt-4 rounded-2xl border border-[#f4efe6]/10 bg-[#0e0c0a]/55 p-4">
      <div className="flex items-center justify-between gap-3">
        <h3 className="text-sm font-semibold tracking-tight">Scorecard</h3>
        <span
          className={`text-sm font-semibold ${
            scorecard.passed ? "text-[#7fd1a0]" : "text-[#e7c18d]"
          }`}
        >
          {pct}% · {scorecard.passed_validators}/{scorecard.total_validators} checks
        </span>
      </div>
      <div className="mt-3 h-1.5 w-full overflow-hidden rounded-full bg-[#f4efe6]/10">
        <div
          className="h-full rounded-full bg-[#d8a15d]"
          style={{ width: `${pct}%` }}
        />
      </div>
      {scorecard.checks.length > 0 ? (
        <ul className="mt-3 space-y-1.5">
          {scorecard.checks.map((check) => (
            <li key={check.key} className="flex items-center gap-2 text-sm">
              {check.status === "passed" ? (
                <CheckCircle2 className="size-4 shrink-0 text-[#7fd1a0]" />
              ) : check.status === "failed" ? (
                <XCircle className="size-4 shrink-0 text-[#e07a7a]" />
              ) : (
                <span className="size-4 shrink-0 rounded-full border border-[#f4efe6]/25" />
              )}
              <span className="text-[#f4efe6]/75">{fieldLabel(check.key)}</span>
              <span className="ml-auto text-xs text-[#f4efe6]/35">{check.status}</span>
            </li>
          ))}
        </ul>
      ) : null}
      {scorecard.dimensions.length > 0 ? (
        <div className="mt-3 flex flex-wrap gap-1.5">
          {scorecard.dimensions.map((dimension) => (
            <span
              key={dimension}
              className="rounded-full border border-[#f4efe6]/12 px-2 py-0.5 text-xs text-[#f4efe6]/50"
            >
              {dimension}
            </span>
          ))}
        </div>
      ) : null}
    </div>
  );
}

function DownloadButton({ label, onClick }: { label: string; onClick: () => void }) {
  return (
    <Button
      type="button"
      variant="outline"
      onClick={onClick}
      className="h-8 rounded-full border-[#f4efe6]/15 bg-transparent px-3 text-sm text-[#f4efe6] hover:bg-[#f4efe6]/10"
    >
      <Download className="size-3.5" />
      {label}
    </Button>
  );
}

function ProofItem({
  icon: Icon,
  label,
  text,
}: {
  icon: typeof Lock;
  label: string;
  text: string;
}) {
  return (
    <div className="rounded-2xl border border-[#f4efe6]/8 bg-[#0e0c0a]/45 p-4">
      <Icon className="size-4 text-[#d8a15d]" />
      <p className="mt-3 font-medium text-[#f4efe6]/85">{label}</p>
      <p className="mt-1 leading-6">{text}</p>
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-[#f4efe6]/10 bg-[#0e0c0a]/45 px-3 py-2.5">
      <dt className="text-xs text-[#f4efe6]/38">{label}</dt>
      <dd className="mt-1 truncate font-medium">{value}</dd>
    </div>
  );
}

function getFields(template: AgentTryoutTemplate | null): [string, FieldSpec][] {
  if (!template?.input_schema.properties) return [];
  return Object.entries(template.input_schema.properties) as [string, FieldSpec][];
}

function buildInput(
  fields: [string, FieldSpec][],
  required: Set<string>,
  values: Record<string, string>,
): { value: Record<string, unknown> } | { error: string } {
  const input: Record<string, unknown> = {};
  for (const [field, spec] of fields) {
    const raw = (values[field] ?? "").trim();
    if (!raw) {
      if (required.has(field)) {
        return { error: `${fieldLabel(field)} is required.` };
      }
      continue;
    }
    if (spec.type === "integer" || spec.type === "number") {
      const value = Number(raw);
      if (!Number.isFinite(value)) {
        return { error: `${fieldLabel(field)} must be a number.` };
      }
      if (spec.minimum !== undefined && value < spec.minimum) {
        return { error: `${fieldLabel(field)} must be at least ${spec.minimum}.` };
      }
      if (spec.maximum !== undefined && value > spec.maximum) {
        return { error: `${fieldLabel(field)} must be at most ${spec.maximum}.` };
      }
      input[field] = spec.type === "integer" ? Math.trunc(value) : value;
    } else {
      input[field] = raw;
    }
  }
  return { value: input };
}

function fieldLabel(field: string): string {
  const spaced = field.replaceAll("_", " ");
  return spaced.charAt(0).toUpperCase() + spaced.slice(1);
}

function triggerDownload(filename: string, contents: string, mime: string) {
  const blob = new Blob([contents], { type: mime });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

function downloadTrace(tryout: AgentTryout, events: TryoutTimelineEvent[]) {
  triggerDownload(
    `agentclash-tryout-${tryout.id}-traces.json`,
    JSON.stringify(
      {
        source: "agentclash_public_tryout",
        exported_at: new Date().toISOString(),
        tryout,
        events,
      },
      null,
      2,
    ),
    "application/json",
  );
}

function downloadScorecard(tryout: AgentTryout, scorecard: TryoutScorecard) {
  triggerDownload(
    `agentclash-tryout-${tryout.id}-scorecard.json`,
    JSON.stringify(
      { source: "agentclash_public_tryout", tryout_id: tryout.id, scorecard },
      null,
      2,
    ),
    "application/json",
  );
}

function downloadOutput(output: TryoutOutputPreview) {
  const name =
    output.relative_path || `${output.key || "artifact"}.${extForType(output.type)}`;
  triggerDownload(name.split("/").pop() || name, output.preview, "text/plain");
}

function downloadArtifacts(tryout: AgentTryout, outputs: TryoutOutputPreview[]) {
  if (outputs.length === 1) {
    downloadOutput(outputs[0]);
    return;
  }
  triggerDownload(
    `agentclash-tryout-${tryout.id}-artifacts.json`,
    JSON.stringify({ tryout_id: tryout.id, outputs }, null, 2),
    "application/json",
  );
}

function extForType(type: string): string {
  switch (type) {
    case "json":
      return "json";
    case "markdown":
      return "md";
    case "csv":
      return "csv";
    default:
      return "txt";
  }
}

type TryoutOutputPreview = {
  key: string;
  type: string;
  relative_path: string;
  preview: string;
  truncated: boolean;
};

function tryoutOutputs(summary: unknown): TryoutOutputPreview[] {
  if (!summary || typeof summary !== "object") return [];
  const outputs = (summary as { outputs?: unknown }).outputs;
  if (!Array.isArray(outputs)) return [];
  return outputs
    .map((item) => {
      if (!item || typeof item !== "object") return null;
      const output = item as Partial<TryoutOutputPreview>;
      if (typeof output.preview !== "string" || output.preview.trim() === "") {
        return null;
      }
      return {
        key: typeof output.key === "string" ? output.key : "",
        type: typeof output.type === "string" ? output.type : "",
        relative_path:
          typeof output.relative_path === "string" ? output.relative_path : "",
        preview: output.preview,
        truncated: output.truncated === true,
      };
    })
    .filter((item): item is TryoutOutputPreview => item !== null);
}

type TryoutScorecardCheck = {
  key: string;
  type: string;
  status: "passed" | "failed" | "skipped";
};

type TryoutScorecard = {
  passed_validators: number;
  total_validators: number;
  score: number;
  passed: boolean;
  dimensions: string[];
  checks: TryoutScorecardCheck[];
};

function tryoutScorecard(summary: unknown): TryoutScorecard | null {
  if (!summary || typeof summary !== "object") return null;
  const raw = (summary as { scorecard?: unknown }).scorecard;
  if (!raw || typeof raw !== "object") return null;
  const card = raw as Record<string, unknown>;
  const total = typeof card.total_validators === "number" ? card.total_validators : 0;
  if (total <= 0 && !Array.isArray(card.checks)) return null;
  const checks: TryoutScorecardCheck[] = Array.isArray(card.checks)
    ? card.checks
        .map((item) => {
          if (!item || typeof item !== "object") return null;
          const check = item as Record<string, unknown>;
          const status =
            check.status === "passed" || check.status === "failed"
              ? check.status
              : "skipped";
          return {
            key: typeof check.key === "string" ? check.key : "check",
            type: typeof check.type === "string" ? check.type : "",
            status,
          } as TryoutScorecardCheck;
        })
        .filter((item): item is TryoutScorecardCheck => item !== null)
    : [];
  return {
    passed_validators:
      typeof card.passed_validators === "number" ? card.passed_validators : 0,
    total_validators: total,
    score: typeof card.score === "number" ? card.score : 0,
    passed: card.passed === true,
    dimensions: Array.isArray(card.dimensions)
      ? card.dimensions.filter((d): d is string => typeof d === "string")
      : [],
    checks,
  };
}

// ---------------------------------------------------------------------------
// Eval ROI calculator — the conversion centerpiece. After the agent runs on
// the buyer's own task, we quantify the bet they're about to make: shipping an
// agent WITHOUT evaluating it vs WITH AgentClash. Anchors are from 2026
// enterprise benchmarks (MIT NANDA 95% pilot-failure, per-task human cost,
// hallucination rates, cost-of-error). All numbers are editable by the buyer.
// ---------------------------------------------------------------------------

type RoiAnchor = {
  label: string;
  humanCostPerTask: number; // fully-loaded human cost to do the task once
  aiCostPerTask: number; // model + sandbox cost per task
  errorRate: number; // share of outputs that go wrong if unevaluated
  costPerError: number; // downstream cost of one bad output (rework/churn/exposure)
};

const ROI_ANCHORS: Record<string, RoiAnchor> = {
  "support-ticket-resolution": { label: "support ticket", humanCostPerTask: 7, aiCostPerTask: 0.03, errorRate: 0.05, costPerError: 50 },
  "document-extraction": { label: "document", humanCostPerTask: 12.88, aiCostPerTask: 0.02, errorRate: 0.03, costPerError: 75 },
  "contract-review": { label: "contract", humanCostPerTask: 150, aiCostPerTask: 0.3, errorRate: 0.064, costPerError: 5000 },
  "sdr-outreach": { label: "outreach email", humanCostPerTask: 40, aiCostPerTask: 0.01, errorRate: 0.05, costPerError: 100 },
  "meeting-minutes": { label: "meeting", humanCostPerTask: 25, aiCostPerTask: 0.01, errorRate: 0.04, costPerError: 30 },
  "slide-deck": { label: "deck", humanCostPerTask: 60, aiCostPerTask: 0.03, errorRate: 0.05, costPerError: 40 },
  "spreadsheet-builder": { label: "spreadsheet", humanCostPerTask: 40, aiCostPerTask: 0.02, errorRate: 0.04, costPerError: 50 },
  "structured-data": { label: "extraction", humanCostPerTask: 12, aiCostPerTask: 0.02, errorRate: 0.03, costPerError: 50 },
  "status-report": { label: "report", humanCostPerTask: 30, aiCostPerTask: 0.01, errorRate: 0.04, costPerError: 30 },
  "inbox-triage": { label: "inbox batch", humanCostPerTask: 15, aiCostPerTask: 0.02, errorRate: 0.05, costPerError: 25 },
};

const DEFAULT_ANCHOR: RoiAnchor = {
  label: "task",
  humanCostPerTask: 10,
  aiCostPerTask: 0.02,
  errorRate: 0.05,
  costPerError: 50,
};

// Share of un-evaluated enterprise GenAI pilots that fail to reach production
// ROI (MIT NANDA, 2025). Evaluated programs reach production ~6x more often.
const UNEVALUATED_FAILURE_RATE = 0.95;
const EVAL_ANNUAL_COST = 12000; // representative platform cost to evaluate this workflow

function usd(value: number): string {
  if (!Number.isFinite(value)) return "$0";
  if (Math.abs(value) >= 1000) {
    return `$${Math.round(value).toLocaleString()}`;
  }
  return `$${value.toFixed(2)}`;
}

function EvalRoiCalculator({
  tryout,
  loginHref,
}: {
  tryout: AgentTryout;
  loginHref: string;
}) {
  const anchor = ROI_ANCHORS[tryout.template_slug] ?? DEFAULT_ANCHOR;
  const [company, setCompany] = useState("");
  const [email, setEmail] = useState("");
  const [volume, setVolume] = useState("5000");
  const [humanCost, setHumanCost] = useState(String(anchor.humanCostPerTask));

  const monthlyVolume = Math.max(0, Number(volume) || 0);
  const humanCostPerTask = Math.max(0, Number(humanCost) || 0);
  const annualVolume = monthlyVolume * 12;

  // Gross automation prize: what moving these tasks to an agent is worth/yr.
  const automationSavings =
    annualVolume * (humanCostPerTask - anchor.aiCostPerTask);

  // Ship WITHOUT evals: errors run unchecked, AND most such pilots never reach
  // production — so you eat the error cost and forfeit the savings.
  const annualErrorCost = annualVolume * anchor.errorRate * anchor.costPerError;
  const forfeitedSavings = automationSavings * UNEVALUATED_FAILURE_RATE;
  const costWithoutEvals = annualErrorCost + forfeitedSavings;

  // Ship WITH evals: catch failures before prod, actually capture the savings,
  // for the price of evaluating.
  const capturedSavings = automationSavings - EVAL_ANNUAL_COST;
  const netUpside = capturedSavings + costWithoutEvals;

  return (
    <div className="mt-6 rounded-2xl border border-[#d8a15d]/25 bg-gradient-to-br from-[#d8a15d]/[0.08] to-transparent p-5">
      <div className="flex items-center gap-2">
        <Calculator className="size-4 text-[#d8a15d]" />
        <h3 className="text-sm font-semibold tracking-tight">
          What this is worth at your scale
        </h3>
      </div>
      <p className="mt-1.5 text-sm leading-6 text-[#f4efe6]/55">
        You just watched an agent do one {anchor.label}. Here is the business
        case for evaluating it before you wire it into production —{" "}
        <span className="text-[#f4efe6]/75">
          95% of enterprise AI pilots fail to deliver ROI
        </span>{" "}
        when they skip this step.
      </p>

      <div className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <RoiInput label="Company" value={company} onChange={setCompany} placeholder="Acme Inc" />
        <RoiInput label="Work email" value={email} onChange={setEmail} placeholder="you@acme.com" />
        <RoiInput label={`${anchor.label}s / month`} value={volume} onChange={setVolume} numeric />
        <RoiInput label="Human $ / task" value={humanCost} onChange={setHumanCost} numeric />
      </div>

      <div className="mt-4 grid gap-3 sm:grid-cols-2">
        <div className="rounded-2xl border border-[#e07a7a]/25 bg-[#e07a7a]/[0.07] p-4">
          <div className="flex items-center gap-2 text-[#f0a8a8]">
            <ShieldAlert className="size-4" />
            <p className="text-sm font-medium">Integrate without evals</p>
          </div>
          <p className="mt-2 text-3xl font-semibold tracking-tight text-[#f4efe6]">
            {usd(costWithoutEvals)}
            <span className="text-base font-normal text-[#f4efe6]/45"> /yr at risk</span>
          </p>
          <ul className="mt-3 space-y-1 text-sm text-[#f4efe6]/55">
            <li>{usd(annualErrorCost)} from unchecked errors ({Math.round(anchor.errorRate * 100)}% bad outputs × {usd(anchor.costPerError)})</li>
            <li>{usd(forfeitedSavings)} in savings forfeited if the pilot stalls (95% do)</li>
          </ul>
        </div>

        <div className="rounded-2xl border border-[#7fd1a0]/25 bg-[#7fd1a0]/[0.07] p-4">
          <div className="flex items-center gap-2 text-[#8fe0b0]">
            <TrendingUp className="size-4" />
            <p className="text-sm font-medium">Evaluate with AgentClash</p>
          </div>
          <p className="mt-2 text-3xl font-semibold tracking-tight text-[#f4efe6]">
            {usd(capturedSavings)}
            <span className="text-base font-normal text-[#f4efe6]/45"> /yr captured</span>
          </p>
          <ul className="mt-3 space-y-1 text-sm text-[#f4efe6]/55">
            <li>{usd(automationSavings)} automation upside, de-risked into production</li>
            <li>− {usd(EVAL_ANNUAL_COST)} to evaluate before you ship</li>
          </ul>
        </div>
      </div>

      <div className="mt-4 flex flex-col items-start justify-between gap-3 rounded-2xl border border-[#f4efe6]/10 bg-[#0e0c0a]/55 p-4 sm:flex-row sm:items-center">
        <p className="text-sm text-[#f4efe6]/70">
          Evaluating first is worth{" "}
          <span className="font-semibold text-[#e7c18d]">{usd(netUpside)}/yr</span>{" "}
          to {company.trim() || "your team"} on this workflow alone.
        </p>
        <Link
          href={`/enterprise?from=tryout&task=${encodeURIComponent(tryout.template_slug)}${email.trim() ? `&email=${encodeURIComponent(email.trim())}` : ""}`}
          className="inline-flex h-9 shrink-0 items-center gap-1.5 rounded-full bg-[#e7c18d] px-4 text-sm font-medium text-[#14120f] transition hover:bg-[#f0cf9d]"
        >
          Talk to us about integrating
          <ArrowRight className="size-4" />
        </Link>
      </div>
      <p className="mt-2 text-xs text-[#f4efe6]/35">
        Estimates use 2026 enterprise benchmarks (MIT NANDA, per-task labor cost,
        hallucination rates). Adjust the inputs to match your numbers.{" "}
        <Link href={loginHref} className="text-[#f4efe6]/55 hover:underline">
          Save this analysis →
        </Link>
      </p>
    </div>
  );
}

function RoiInput({
  label,
  value,
  onChange,
  placeholder,
  numeric,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  numeric?: boolean;
}) {
  return (
    <label className="block">
      <span className="mb-1 block text-xs text-[#f4efe6]/45">{label}</span>
      <input
        type={numeric ? "number" : "text"}
        inputMode={numeric ? "decimal" : undefined}
        min={numeric ? 0 : undefined}
        value={value}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
        className="h-9 w-full rounded-xl border border-[#f4efe6]/12 bg-[#0e0c0a]/65 px-3 text-sm text-[#f4efe6] outline-none placeholder:text-[#f4efe6]/25 focus:border-[#d8a15d]/50 focus:ring-2 focus:ring-[#d8a15d]/20"
      />
    </label>
  );
}

// ---------------------------------------------------------------------------
// TryoutChat — the live conversation. The visitor talks to the agent turn by
// turn while the session is active; agent reasoning/tool steps stream in from
// the event log. User messages are tracked locally (the page sent them) and
// interleaved with agent steps.
// ---------------------------------------------------------------------------

type ChatUserMessage = { id: string; text: string; at: number };

function TryoutChat({
  tryout,
  events,
}: {
  tryout: AgentTryout;
  events: TryoutTimelineEvent[];
}) {
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  const [ending, setEnding] = useState(false);
  const [userMessages, setUserMessages] = useState<ChatUserMessage[]>([]);
  const [error, setError] = useState<string | null>(null);

  const active = tryoutIsActive(tryout.status);

  // Reset local chat when switching to a different tryout.
  useEffect(() => {
    setUserMessages([]);
    setDraft("");
    setError(null);
  }, [tryout.id]);

  const agentSteps = useMemo(
    () =>
      events
        .filter(
          (event) =>
            event.type === "planning" ||
            event.type === "tool_call" ||
            event.type === "finished",
        )
        .map((event) => ({
          kind: "agent" as const,
          id: `e${event.cursor}`,
          text: event.summary,
          at: new Date(event.occurred_at).getTime(),
          stepType: event.type,
        })),
    [events],
  );

  const timeline = useMemo(() => {
    const user = userMessages.map((m) => ({
      kind: "user" as const,
      id: m.id,
      text: m.text,
      at: m.at,
      stepType: "user" as const,
    }));
    return [...user, ...agentSteps].sort((a, b) => a.at - b.at);
  }, [userMessages, agentSteps]);

  async function send() {
    const text = draft.trim();
    if (!text || sending) return;
    setSending(true);
    setError(null);
    try {
      await submitAgentTryoutTurn(api, tryout.id, { message: text });
      setUserMessages((current) => [
        ...current,
        { id: `u${Date.now()}`, text, at: Date.now() },
      ]);
      setDraft("");
    } catch (err) {
      setError(
        err instanceof ApiError ? err.message : "Could not send your message.",
      );
    } finally {
      setSending(false);
    }
  }

  async function endSession() {
    if (ending) return;
    setEnding(true);
    try {
      await submitAgentTryoutTurn(api, tryout.id, { end: true });
    } catch {
      // best-effort; the session also ends on idle / cap
    } finally {
      setEnding(false);
    }
  }

  return (
    <div className="mt-4 rounded-2xl border border-[#f4efe6]/10 bg-[#0e0c0a]/55 p-4">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold tracking-tight">Conversation</h3>
        {active ? (
          <button
            type="button"
            onClick={endSession}
            disabled={ending}
            className="text-xs text-[#f4efe6]/45 transition hover:text-[#f4efe6]/80"
          >
            End session
          </button>
        ) : null}
      </div>

      <div className="mt-3 max-h-96 space-y-2 overflow-auto">
        {timeline.length === 0 ? (
          <p className="text-sm text-[#f4efe6]/45">
            {active
              ? "The agent is starting. Watch its steps, then reply below."
              : "No conversation was recorded."}
          </p>
        ) : (
          timeline.map((item) =>
            item.kind === "user" ? (
              <div key={item.id} className="flex justify-end">
                <div className="max-w-[80%] rounded-2xl rounded-br-sm bg-[#e7c18d] px-3 py-2 text-sm text-[#14120f]">
                  {item.text}
                </div>
              </div>
            ) : (
              <div key={item.id} className="flex justify-start">
                <div className="flex max-w-[85%] items-start gap-2 rounded-2xl rounded-bl-sm border border-[#f4efe6]/10 bg-[#f4efe6]/[0.04] px-3 py-2 text-sm text-[#f4efe6]/80">
                  {item.stepType === "tool_call" ? (
                    <Activity className="mt-0.5 size-3.5 shrink-0 text-[#7fd1a0]" />
                  ) : (
                    <Gauge className="mt-0.5 size-3.5 shrink-0 text-[#d8a15d]" />
                  )}
                  <span className="min-w-0">{item.text}</span>
                </div>
              </div>
            ),
          )
        )}
      </div>

      {active ? (
        <form
          onSubmit={(event) => {
            event.preventDefault();
            void send();
          }}
          className="mt-3 flex items-end gap-2 rounded-2xl border border-[#f4efe6]/12 bg-[#16140f]/80 p-2 focus-within:border-[#d8a15d]/40"
        >
          <textarea
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter" && !event.shiftKey) {
                event.preventDefault();
                void send();
              }
            }}
            rows={1}
            placeholder="Reply to the agent…"
            className="block max-h-32 w-full resize-none bg-transparent px-2 py-1.5 text-sm text-[#f4efe6] outline-none placeholder:text-[#f4efe6]/30"
          />
          <button
            type="submit"
            disabled={!draft.trim() || sending}
            aria-label="Send message"
            className="flex size-8 shrink-0 items-center justify-center rounded-full bg-[#e7c18d] text-[#14120f] transition hover:bg-[#f0cf9d] disabled:opacity-40"
          >
            {sending ? (
              <Loader2 className="size-4 animate-spin" />
            ) : (
              <ArrowUp className="size-4" />
            )}
          </button>
        </form>
      ) : (
        <p className="mt-3 text-xs text-[#f4efe6]/40">
          This session has ended. Sign in to start a fresh conversation and save it.
        </p>
      )}
      {error ? <p className="mt-2 text-xs text-red-300">{error}</p> : null}
    </div>
  );
}
