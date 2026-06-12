"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import type { FormEvent, ReactNode } from "react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
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
  ListChecks,
  Loader2,
  Lock,
  PanelRight,
  ShieldAlert,
  Terminal,
  TrendingUp,
  Wrench,
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
  AgentTryoutModelPolicy,
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
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import { cn } from "@/lib/utils";

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

type ModelOption = AgentOption & {
  provider?: "openai" | "anthropic" | "openrouter";
  model?: string;
};

type EvalPriority = "accuracy" | "polish" | "speed" | "cost" | "compliance";
type EvalStyle = "consistent" | "balanced" | "creative";

type EvalSetupValues = {
  unacceptableMistakes: string;
  reviewer: string;
  priority: EvalPriority;
  style: EvalStyle;
  monthlyVolume: string;
};

type EvalRubricItem = {
  key: string;
  label: string;
  checks: string[];
};

type DerivedEvalSetup = {
  version: string;
  unacceptable_mistakes: string;
  human_reviewer: string;
  business_priority: EvalPriority;
  output_style: EvalStyle;
  monthly_volume: string;
  derived_rubric: EvalRubricItem[];
  suggested_generation_settings: {
    temperature: string;
    reason: string;
  };
};

const DEFAULT_EVAL_SETUP: EvalSetupValues = {
  unacceptableMistakes: "",
  reviewer: "Operations owner",
  priority: "accuracy",
  style: "consistent",
  monthlyVolume: "",
};

const PRIORITY_OPTIONS: { value: EvalPriority; label: string }[] = [
  { value: "accuracy", label: "Correct facts" },
  { value: "polish", label: "Client-ready" },
  { value: "speed", label: "Fast enough" },
  { value: "cost", label: "Cheap at scale" },
  { value: "compliance", label: "Policy-safe" },
];

const STYLE_OPTIONS: { value: EvalStyle; label: string }[] = [
  { value: "consistent", label: "Same every time" },
  { value: "balanced", label: "Balanced" },
  { value: "creative", label: "More creative" },
];

const MODEL_OPTIONS: ModelOption[] = [
  { value: "", label: "Auto", hint: "Hosted default agent and model" },
  {
    value: "codex_e2b",
    label: "GPT-5",
    hint: "OpenAI via Codex",
    provider: "openai",
    model: "gpt-5",
  },
  {
    value: "codex_e2b",
    label: "GPT-5 mini",
    hint: "OpenAI via Codex",
    provider: "openai",
    model: "gpt-5-mini",
  },
  {
    value: "claude_e2b",
    label: "Claude Sonnet 4.5",
    hint: "Anthropic via Claude Code",
    provider: "anthropic",
    model: "claude-sonnet-4-5",
  },
  {
    value: "claude_e2b",
    label: "Claude Opus 4.1",
    hint: "Anthropic via Claude Code",
    provider: "anthropic",
    model: "claude-opus-4-1",
  },
  {
    value: "openclaw_e2b",
    label: "Gemini 2.5 Pro",
    hint: "Google Gemini via OpenRouter",
    provider: "openrouter",
    model: "google/gemini-2.5-pro",
  },
  {
    value: "openclaw_e2b",
    label: "Gemini 2.5 Flash",
    hint: "Google Gemini via OpenRouter",
    provider: "openrouter",
    model: "google/gemini-2.5-flash",
  },
];

function agentLabel(value: string): string {
  switch (value) {
    case "codex_e2b":
      return "Codex";
    case "claude_e2b":
      return "Claude";
    case "openclaw_e2b":
      return "OpenClaw";
    default:
      return "Auto";
  }
}

function modelOptionKey(option: ModelOption): string {
  return option.model ? `${option.provider}:${option.model}` : "auto";
}

function modelPolicyFor(option: ModelOption): AgentTryoutModelPolicy | undefined {
  if (!option.provider || !option.model) return undefined;
  return {
    mode: "explicit",
    max_models: 1,
    models: [{ provider: option.provider, model: option.model }],
  };
}

function modelLabelFromPolicy(policy: unknown): string {
  if (!policy || typeof policy !== "object") return "Auto";
  const models = (policy as { models?: unknown }).models;
  if (!Array.isArray(models) || models.length === 0) return "Auto";
  const first = models[0] as { provider?: unknown; model?: unknown };
  if (typeof first.model !== "string" || first.model.trim() === "") return "Auto";
  const match = MODEL_OPTIONS.find(
    (option) => option.provider === first.provider && option.model === first.model,
  );
  return match?.label ?? first.model;
}

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
    "Make a 6-slide deck for 10 year olds explaining what AI is, with a simple chart.",
    "Turn this product brief into a PowerPoint with speaker notes and one diagram.",
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

function ThinkingIndicator({ label = "Thinking" }: { label?: string }) {
  return (
    <div className="flex justify-start animate-in fade-in duration-500">
      <div className="flex items-center gap-3 rounded-2xl border border-white/10 bg-white/[0.03] px-4 py-3">
        <div className="flex items-center gap-1" aria-hidden>
          {[0, 1, 2].map((index) => (
            <span
              key={index}
              className="size-1.5 rounded-full bg-white/35 animate-pulse"
              style={{ animationDelay: `${index * 180}ms` }}
            />
          ))}
        </div>
        <span className="text-sm text-white/45">{label}</span>
      </div>
    </div>
  );
}

export function PublicTryoutsClient() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const urlTryoutId = searchParams.get("tryout") ?? "";

  const [templates, setTemplates] = useState<AgentTryoutTemplate[]>([]);
  const [templateSlug, setTemplateSlug] = useState("");
  const [selectedModelKey, setSelectedModelKey] = useState("auto");
  const [fieldValues, setFieldValues] = useState<Record<string, string>>({});
  const [evalSetup, setEvalSetup] = useState<EvalSetupValues>(DEFAULT_EVAL_SETUP);
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

  const updateEvalSetup = useCallback(
    <Key extends keyof EvalSetupValues>(field: Key, value: EvalSetupValues[Key]) => {
      setEvalSetup((current) => ({ ...current, [field]: value }));
    },
    [],
  );

  async function handleLaunch(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!template || launching) return;

    const input = buildInput(fields, required, fieldValues);
    if ("error" in input) {
      setMessage(input.error);
      return;
    }
    input.value.eval_setup = buildEvalSetup(evalSetup, template?.name ?? "agent task");

    const selectedModel =
      MODEL_OPTIONS.find((option) => modelOptionKey(option) === selectedModelKey) ??
      MODEL_OPTIONS[0];
    const selectedPolicy = modelPolicyFor(selectedModel);

    setLaunching(true);
    setMessage(null);
    setQuotaMessage(null);
    try {
      const nextTryout = await createAnonymousAgentTryout(api, {
        template_slug: template.slug,
        input: input.value,
        ...(selectedModel.value ? { selected_harness_kind: selectedModel.value } : {}),
        ...(selectedPolicy ? { selected_model_policy: selectedPolicy } : {}),
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
  const inSession = Boolean(urlTryoutId);

  return (
    <main className="flex h-[100dvh] flex-col overflow-hidden bg-black text-white">
      <div className="pointer-events-none fixed inset-0 bg-[radial-gradient(circle_at_50%_0%,rgba(255,255,255,0.05),transparent_55%)]" />

      <header className="relative z-10 flex shrink-0 items-center justify-between gap-4 border-b border-white/10 px-4 py-3 sm:px-6">
        <div className="flex items-center gap-3">
          <Link
            href="/"
            className="text-sm font-semibold tracking-tight text-white/90"
          >
            AgentClash
          </Link>
          {tryout ? (
            <Badge variant={tryoutStatusVariant(tryout.status)} className="hidden sm:inline-flex">
              {tryout.status}
            </Badge>
          ) : null}
        </div>
        <div className="flex items-center gap-2 sm:gap-3">
          {inSession && tryout ? (
            <TryoutSidebarMobile
              tryout={tryout}
              events={events}
              loginHref={loginHref}
            />
          ) : null}
          {inSession ? (
            <Link
              href="/tryouts"
              className="hidden text-sm text-white/55 transition hover:text-white sm:inline"
            >
              New tryout
            </Link>
          ) : (
            <Link
              href="/pricing"
              className="hidden text-sm text-white/55 transition hover:text-white sm:inline"
            >
              For teams
            </Link>
          )}
          <Link
            href={loginHref}
            className="rounded-full border border-white/15 px-3 py-1.5 text-sm text-white/80 transition hover:border-white/30 hover:text-white"
          >
            Sign in
          </Link>
        </div>
      </header>

      {inSession ? (
        <TryoutSession
          tryout={tryout}
          events={events}
          loading={tryoutLoading}
          loginHref={loginHref}
          message={message}
        />
      ) : (
        <TryoutWelcome
          template={template}
          templates={templates}
          templateSlug={templateSlug}
          setTemplateSlug={setTemplateSlug}
          selectedModelKey={selectedModelKey}
          setSelectedModelKey={setSelectedModelKey}
          primaryField={primaryField}
          secondaryFields={secondaryFields}
          fieldValues={fieldValues}
          updateField={updateField}
          evalSetup={evalSetup}
          updateEvalSetup={updateEvalSetup}
          primaryValue={primaryValue}
          canRun={canRun}
          launching={launching}
          templatesLoading={templatesLoading}
          message={message}
          quotaMessage={quotaMessage}
          loginHref={loginHref}
          onSubmit={handleLaunch}
        />
      )}
    </main>
  );
}

function TryoutWelcome({
  template,
  templates,
  templateSlug,
  setTemplateSlug,
  selectedModelKey,
  setSelectedModelKey,
  primaryField,
  secondaryFields,
  fieldValues,
  updateField,
  evalSetup,
  updateEvalSetup,
  primaryValue,
  canRun,
  launching,
  templatesLoading,
  message,
  quotaMessage,
  loginHref,
  onSubmit,
}: {
  template: AgentTryoutTemplate | null;
  templates: AgentTryoutTemplate[];
  templateSlug: string;
  setTemplateSlug: (value: string) => void;
  selectedModelKey: string;
  setSelectedModelKey: (value: string) => void;
  primaryField: [string, FieldSpec] | null;
  secondaryFields: [string, FieldSpec][];
  fieldValues: Record<string, string>;
  updateField: (field: string, value: string) => void;
  evalSetup: EvalSetupValues;
  updateEvalSetup: <Key extends keyof EvalSetupValues>(
    field: Key,
    value: EvalSetupValues[Key],
  ) => void;
  primaryValue: string;
  canRun: boolean;
  launching: boolean;
  templatesLoading: boolean;
  message: string | null;
  quotaMessage: string | null;
  loginHref: string;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
}) {
  return (
    <div className="relative flex flex-1 flex-col items-center justify-center overflow-y-auto px-4 py-8 sm:px-6">
      <h1 className="text-center text-[clamp(2rem,7vw,3.5rem)] font-semibold leading-none tracking-tight">
        Try an agent on real work
      </h1>
      <p className="mt-3 max-w-lg text-center text-base text-white/55">
        Chat with a sandboxed agent. When it finishes, see the eval scorecard and
        what shipping it is worth at your scale.
      </p>

      <form onSubmit={onSubmit} className="mt-8 w-full max-w-2xl">
        <ComposerShell
          value={primaryValue}
          onChange={(value) =>
            primaryField && updateField(primaryField[0], value)
          }
          disabled={!template}
          placeholder={
            template
              ? `Describe the work for "${template.name}"…`
              : "Loading tasks…"
          }
          canSubmit={canRun}
          submitting={launching}
          footer={
            <>
              <AnimatedPillSelect
                icon={<FileText className="size-3.5" />}
                value={templateSlug}
                onChange={setTemplateSlug}
                disabled={templatesLoading}
                options={templates.map((t) => ({ value: t.slug, label: t.name }))}
              />
              <AnimatedPillSelect
                icon={<Gauge className="size-3.5" />}
                value={selectedModelKey}
                onChange={setSelectedModelKey}
                options={MODEL_OPTIONS.map((option) => ({
                  value: modelOptionKey(option),
                  label: option.label,
                }))}
              />
            </>
          }
        />

        {secondaryFields.length > 0 ? (
          <div className="mt-2 grid gap-2 sm:grid-cols-2">
            {secondaryFields.map(([field, spec]) => (
              <CompactField
                key={field}
                field={field}
                spec={spec}
                value={fieldValues[field] ?? ""}
                required={false}
                onChange={updateField}
              />
            ))}
          </div>
        ) : null}

        <EvalSetupPanel values={evalSetup} onChange={updateEvalSetup} />

        {message ? <Alert text={message} /> : null}
        {quotaMessage ? (
          <div className="mt-3 rounded-2xl border border-white/15 bg-white/[0.04] p-3 text-sm text-white/70">
            <p>{quotaMessage}</p>
            <Link
              href={loginHref}
              className="mt-2 inline-flex items-center gap-1 font-medium text-white hover:underline"
            >
              Save this tryout in a workspace
              <ArrowRight className="size-3.5" />
            </Link>
          </div>
        ) : null}

        {template ? (
          <p className="mt-3 text-center text-xs text-white/40">
            {template.description} ·{" "}
            {MODEL_OPTIONS.find((option) => modelOptionKey(option) === selectedModelKey)?.hint ??
              "Hosted default agent and model"}{" "}
            · cap{" "}
            {`$${template.max_cost_usd.toFixed(2)}`}
          </p>
        ) : null}
      </form>

      {template && primaryField ? (
        <div className="mt-6 w-full max-w-2xl">
          <p className="mb-2 text-center text-xs uppercase tracking-[0.14em] text-white/35">
            Try one of these
          </p>
          <div className="flex flex-wrap justify-center gap-2">
            {suggestionsFor(template.slug).map((suggestion) => (
              <button
                key={suggestion}
                type="button"
                onClick={() => updateField(primaryField[0], suggestion)}
                className="rounded-full border border-white/10 bg-white/[0.03] px-3 py-1.5 text-left text-xs text-white/60 transition hover:border-white/25 hover:text-white"
              >
                {suggestion.length > 70 ? suggestion.slice(0, 70) + "…" : suggestion}
              </button>
            ))}
          </div>
        </div>
      ) : null}

      <div className="mt-8 hidden w-full max-w-2xl gap-3 sm:grid-cols-3">
        <ProofItem icon={Lock} label="Sandboxed" text="Real tools, capped cost." />
        <ProofItem icon={Activity} label="Live trace" text="Every step in the sidebar." />
        <ProofItem icon={FileText} label="Evals built in" text="Scorecard when the run ends." />
      </div>
    </div>
  );
}

function TryoutSession({
  tryout,
  events,
  loading,
  loginHref,
  message,
}: {
  tryout: AgentTryout | null;
  events: TryoutTimelineEvent[];
  loading: boolean;
  loginHref: string;
  message: string | null;
}) {
  if (!tryout && loading) {
    return (
      <div className="flex flex-1 items-center justify-center text-sm text-white/55">
        <Loader2 className="mr-2 size-4 animate-spin" />
        Starting your session…
      </div>
    );
  }

  if (!tryout) {
    return (
      <div className="flex flex-1 items-center justify-center px-4 text-sm text-white/55">
        {message ?? "Could not load this tryout."}
      </div>
    );
  }

  const outputs = tryoutOutputs(tryout.summary);
  const scorecard = tryoutScorecard(tryout.summary);

  return (
    <div className="relative flex min-h-0 flex-1">
      <aside className="hidden w-80 shrink-0 flex-col border-r border-white/10 bg-zinc-950/80/60 lg:flex">
        <TryoutSidebar
          tryout={tryout}
          events={events}
          outputs={outputs}
          scorecard={scorecard}
          loginHref={loginHref}
        />
      </aside>

      <div className="flex min-h-0 min-w-0 flex-1 flex-col">
        <TryoutChatThread
          tryout={tryout}
          events={events}
          outputs={outputs}
          scorecard={scorecard}
          loginHref={loginHref}
        />
      </div>
    </div>
  );
}

function TryoutSidebarMobile({
  tryout,
  events,
  loginHref,
}: {
  tryout: AgentTryout;
  events: TryoutTimelineEvent[];
  loginHref: string;
}) {
  const outputs = tryoutOutputs(tryout.summary);
  const scorecard = tryoutScorecard(tryout.summary);

  return (
    <Sheet>
      <SheetTrigger
        render={
          <Button
            variant="outline"
            size="sm"
            className="h-8 rounded-full border-white/15 bg-transparent text-white hover:bg-white/10 lg:hidden"
          />
        }
      >
        <PanelRight className="size-3.5" />
        Trace
      </SheetTrigger>
      <SheetContent
        side="right"
        className="w-full border-white/10 bg-zinc-950/80 text-white sm:max-w-md"
      >
        <SheetHeader>
          <SheetTitle className="text-white">Trace & downloads</SheetTitle>
        </SheetHeader>
        <TryoutSidebar
          tryout={tryout}
          events={events}
          outputs={outputs}
          scorecard={scorecard}
          loginHref={loginHref}
          compact
        />
      </SheetContent>
    </Sheet>
  );
}

function TryoutSidebar({
  tryout,
  events,
  outputs,
  scorecard,
  loginHref,
  compact,
}: {
  tryout: AgentTryout;
  events: TryoutTimelineEvent[];
  outputs: TryoutOutputPreview[];
  scorecard: TryoutScorecard | null;
  loginHref: string;
  compact?: boolean;
}) {
  const modelRan = modelLabelFromPolicy(tryout.selected_model_policy);
  const agentRan = tryout.selected_harness_kind
    ? `${modelRan} · ${agentLabel(tryout.selected_harness_kind)}`
    : modelRan;
  const evalPlan = evalSetupFromInput(tryout.input_snapshot);

  return (
    <div className={cn("flex min-h-0 flex-1 flex-col", compact ? "pt-2" : "p-4")}>
      <div className="space-y-1 px-1">
        <p className="text-sm font-medium text-white/90">{tryout.template_slug}</p>
        <p className="text-xs text-white/45">
          {agentRan} · {formatTryoutLatency(tryout.latency_ms)} ·{" "}
          {formatTryoutCost(tryout)}
        </p>
      </div>

      <div className="mt-3 flex flex-wrap gap-2 px-1">
        <DownloadButton label="Trace" onClick={() => downloadTrace(tryout, events)} />
        {scorecard ? (
          <DownloadButton
            label="Scorecard"
            onClick={() => downloadScorecard(tryout, scorecard)}
          />
        ) : null}
        {outputs.length > 0 ? (
          <DownloadButton
            label="Artifacts"
            onClick={() => downloadArtifacts(tryout, outputs)}
          />
        ) : null}
      </div>

      {scorecard ? (
        <div className="mt-4 px-1">
          <ScorecardCard scorecard={scorecard} compact />
        </div>
      ) : null}

      {evalPlan ? (
        <div className="mt-4 px-1">
          <EvalPlanCard setup={evalPlan} compact />
        </div>
      ) : null}

      {outputs.length > 0 ? (
        <div className="mt-4 min-h-0 px-1">
          <p className="mb-2 text-xs font-medium uppercase tracking-[0.12em] text-white/40">
            Artifacts
          </p>
          <div className="space-y-2">
            {outputs.map((output) => (
              <ArtifactPreviewCard key={`${output.key}-${output.relative_path}`} output={output} />
            ))}
          </div>
        </div>
      ) : null}

      <div className="mt-4 flex min-h-0 flex-1 flex-col px-1">
        <p className="mb-2 text-xs font-medium uppercase tracking-[0.12em] text-white/40">
          Event log
        </p>
        <ol className="min-h-0 flex-1 space-y-0 overflow-y-auto rounded-xl border border-white/10 bg-black/80">
          {events.length === 0 ? (
            <li className="p-3 text-xs text-white/45">
              {tryoutIsActive(tryout.status)
                ? "Waiting for events…"
                : "No events recorded."}
            </li>
          ) : (
            events.map((event) => (
              <li
                key={event.cursor}
                className="border-b border-white/6 px-3 py-2 text-xs last:border-b-0"
              >
                <p className="text-white/75">{event.summary}</p>
                <time className="mt-0.5 block text-[10px] text-white/35">
                  {new Date(event.occurred_at).toLocaleTimeString()}
                </time>
              </li>
            ))
          )}
        </ol>
      </div>

      <Link
        href={loginHref}
        className="mt-4 inline-flex h-9 items-center justify-center gap-1.5 rounded-full bg-white px-4 text-sm font-medium text-black transition hover:bg-white/90"
      >
        Save and rerun
        <ArrowRight className="size-4" />
      </Link>
    </div>
  );
}

function TryoutChatThread({
  tryout,
  events,
  outputs,
  scorecard,
  loginHref,
}: {
  tryout: AgentTryout;
  events: TryoutTimelineEvent[];
  outputs: TryoutOutputPreview[];
  scorecard: TryoutScorecard | null;
  loginHref: string;
}) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const bottomRef = useRef<HTMLDivElement>(null);
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  const [ending, setEnding] = useState(false);
  const [followUps, setFollowUps] = useState<{ id: string; text: string; at: number }[]>([]);
  const [error, setError] = useState<string | null>(null);

  const active = tryoutIsActive(tryout.status);
  const finished = !active;
  const evalPlan = useMemo(
    () => evalSetupFromInput(tryout.input_snapshot),
    [tryout.input_snapshot],
  );

  useEffect(() => {
    setFollowUps([]);
    setDraft("");
    setError(null);
  }, [tryout.id]);

  const initialUserText = useMemo(
    () => formatInputSnapshot(tryout.input_snapshot),
    [tryout.input_snapshot],
  );

  const timeline = useMemo(() => {
    const items: ChatItem[] = [];

    if (initialUserText) {
      items.push({
        kind: "user",
        id: "initial",
        text: initialUserText,
        at: new Date(tryout.created_at).getTime(),
      });
    }

    for (const msg of followUps) {
      items.push({
        kind: "user",
        id: msg.id,
        text: msg.text,
        at: msg.at,
      });
    }

    for (const event of events) {
      if (event.type === "started") continue;
      items.push({
        kind: "agent",
        id: `e${event.cursor}`,
        text: friendlyTraceSummary(event),
        at: new Date(event.occurred_at).getTime(),
        eventType: event.type,
      });
    }

    return items.sort((a, b) => a.at - b.at);
  }, [initialUserText, followUps, events, tryout.created_at]);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [timeline.length, outputs.length, scorecard, finished]);

  async function send() {
    const text = draft.trim();
    if (!text || sending) return;
    setSending(true);
    setError(null);
    try {
      await submitAgentTryoutTurn(api, tryout.id, { message: text });
      setFollowUps((current) => [
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
      // best-effort
    } finally {
      setEnding(false);
    }
  }

  return (
    <>
      <div ref={scrollRef} className="min-h-0 flex-1 overflow-y-auto px-4 py-6 sm:px-8">
        <div className="mx-auto flex max-w-3xl flex-col gap-4">
          {timeline.map((item, index) =>
            item.kind === "user" ? (
              <UserBubble key={item.id} text={item.text} animate={index === timeline.length - 1} />
            ) : (
              <AgentStepBubble
                key={item.id}
                text={item.text}
                eventType={item.eventType}
                animate={index === timeline.length - 1}
              />
            ),
          )}

          {active && events.length === 0 ? <ThinkingIndicator label="Starting" /> : null}

          {active && outputs.length === 0 && timeline.length > 0 && timeline[timeline.length - 1]?.kind === "agent" ? (
            <ThinkingIndicator />
          ) : null}

          {outputs
            .filter((output) => output.type !== "json")
            .map((output) => (
            <ArtifactChatCard key={`${output.key}-${output.relative_path}`} output={output} />
          ))}

          {evalPlan && (outputs.length > 0 || finished) ? (
            <div className="animate-in fade-in slide-in-from-bottom-2 duration-500">
              <EvalPlanCard setup={evalPlan} />
            </div>
          ) : null}

          {scorecard && finished ? (
            <div className="animate-in fade-in slide-in-from-bottom-2 duration-500">
              <ScorecardCard scorecard={scorecard} />
            </div>
          ) : null}

          {finished ? (
            <div className="animate-in fade-in slide-in-from-bottom-3 duration-700">
              <EvalRoiCalculator tryout={tryout} loginHref={loginHref} />
            </div>
          ) : null}

          <div ref={bottomRef} />
        </div>
      </div>

      <div className="shrink-0 border-t border-white/10 bg-black/95 px-4 py-3 backdrop-blur sm:px-8">
        <div className="mx-auto max-w-3xl">
          {active ? (
            <>
              <div className="mb-2 flex items-center justify-between">
                <p className="text-xs text-white/40">
                  {outputs.length > 0
                    ? "Outputs are ready. Reply to request edits, or end the session to finalize scoring."
                    : "Reply to steer the agent, or let it finish on its own."}
                </p>
                <button
                  type="button"
                  onClick={endSession}
                  disabled={ending}
                  className="text-xs text-white/45 transition hover:text-white/80"
                >
                  End session
                </button>
              </div>
              <ComposerShell
                value={draft}
                onChange={setDraft}
                disabled={false}
                placeholder="Reply to the agent…"
                canSubmit={Boolean(draft.trim()) && !sending}
                submitting={sending}
                onSubmit={() => void send()}
                compact
              />
            </>
          ) : (
            <p className="text-center text-sm text-white/45">
              Session complete.{" "}
              <Link href={loginHref} className="text-white underline-offset-4 hover:underline">
                Sign in
              </Link>{" "}
              to save this run and wire it into evals.
            </p>
          )}
          {error ? <p className="mt-2 text-xs text-white/50">{error}</p> : null}
        </div>
      </div>
    </>
  );
}

type ChatItem = {
  kind: "user" | "agent";
  id: string;
  text: string;
  at: number;
  eventType?: TryoutTimelineEvent["type"];
};

function UserBubble({ text, animate }: { text: string; animate?: boolean }) {
  return (
    <div
      className={cn(
        "flex justify-end",
        animate && "animate-in fade-in slide-in-from-bottom-2 duration-300",
      )}
    >
      <div className="max-w-[85%] rounded-2xl rounded-br-md bg-white px-4 py-2.5 text-[15px] leading-7 text-black">
        {text}
      </div>
    </div>
  );
}

function AgentStepBubble({
  text,
  eventType,
  animate,
}: {
  text: string;
  eventType?: TryoutTimelineEvent["type"];
  animate?: boolean;
}) {
  return (
    <div
      className={cn(
        "flex justify-start",
        animate && "animate-in fade-in slide-in-from-bottom-2 duration-300",
      )}
    >
      <div className="flex max-w-[90%] items-start gap-2.5 rounded-2xl rounded-bl-md border border-white/10 bg-white/[0.03] px-3.5 py-2.5 text-sm leading-6 text-white/75">
        <EventStepIcon type={eventType} />
        <span className="min-w-0 whitespace-pre-wrap">{text}</span>
      </div>
    </div>
  );
}

function EventStepIcon({ type }: { type?: TryoutTimelineEvent["type"] }) {
  const className = "mt-0.5 size-4 shrink-0 text-white/35";
  switch (type) {
    case "tool_call":
      return <Wrench className={className} />;
    case "sandbox_command":
      return <Terminal className={className} />;
    case "file_written":
    case "file_activity":
      return <FileText className={className} />;
    case "validation":
    case "scoring":
      return <CheckCircle2 className={className} />;
    case "planning":
      return <Gauge className={className} />;
    default:
      return <Activity className={className} />;
  }
}

function ArtifactChatCard({ output }: { output: TryoutOutputPreview }) {
  const label = output.relative_path || output.key || "Artifact";
  const sizeLabel =
    typeof output.size_bytes === "number"
      ? `${Math.max(1, Math.round(output.size_bytes / 1024))} KB`
      : null;

  return (
    <div className="animate-in fade-in slide-in-from-bottom-2 duration-500">
      <div className="overflow-hidden rounded-xl border border-white/10 bg-white/[0.02]">
        <div className="flex items-center justify-between gap-2 border-b border-white/8 px-4 py-2.5">
          <div className="flex min-w-0 items-center gap-2">
            <FileText className="size-4 shrink-0 text-white/40" />
            <div className="min-w-0">
              <p className="truncate text-sm font-medium text-white/90">{label}</p>
              {sizeLabel ? (
                <p className="text-[11px] text-white/40">{artifactKindLabel(output)} · {sizeLabel}</p>
              ) : null}
            </div>
          </div>
          <button
            type="button"
            onClick={() => downloadOutput(output)}
            className="inline-flex shrink-0 items-center gap-1 rounded-full border border-white/10 px-2.5 py-1 text-xs text-white/60 transition hover:border-white/25 hover:text-white"
          >
            <Download className="size-3" />
            Download
          </button>
        </div>
        <ArtifactPreviewBody output={output} />
        {output.truncated ? (
          <p className="border-t border-white/8 px-4 py-2 text-xs text-white/38">
            Preview truncated. Download for the full file.
          </p>
        ) : null}
      </div>
    </div>
  );
}

function ArtifactPreviewBody({ output }: { output: TryoutOutputPreview }) {
  const previewUrl = artifactDataUrl(output);

  if (isPdfArtifact(output) && previewUrl) {
    return (
      <div className="bg-black p-2">
        <iframe
          title={output.relative_path || "PDF preview"}
          src={previewUrl}
          className="h-[28rem] w-full rounded-lg border border-white/10 bg-white"
        />
      </div>
    );
  }

  if (isImageArtifact(output) && previewUrl) {
    return (
      <div className="p-4">
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img
          src={previewUrl}
          alt={output.relative_path || "Generated chart"}
          className="max-h-80 w-full rounded-lg object-contain"
        />
      </div>
    );
  }

  if (isBinaryArtifact(output)) {
    return (
      <div className="px-4 py-6 text-sm leading-7 text-white/60">
        <p>
          {artifactKindLabel(output)} ready. Download to open it in PowerPoint, Keynote, or
          Preview.
        </p>
      </div>
    );
  }

  return (
    <div className="max-h-80 overflow-auto p-4">
      <pre className="whitespace-pre-wrap font-sans text-sm leading-7 text-white/75">
        {output.preview}
      </pre>
    </div>
  );
}

function ArtifactPreviewCard({ output }: { output: TryoutOutputPreview }) {
  return (
    <div className="rounded-xl border border-white/10 bg-[#0c0c0a]/80 p-3">
      <div className="flex items-center justify-between gap-2">
        <div className="min-w-0">
          <p className="truncate text-xs font-medium text-white/80">
            {output.relative_path || output.key || "Output"}
          </p>
          <p className="text-[10px] text-white/35">{artifactKindLabel(output)}</p>
        </div>
        <button
          type="button"
          onClick={() => downloadOutput(output)}
          className="text-white/45 transition hover:text-white"
          aria-label="Download"
        >
          <Download className="size-3.5" />
        </button>
      </div>
      {!isBinaryArtifact(output) ? (
        <pre className="mt-2 max-h-32 overflow-auto whitespace-pre-wrap text-[11px] leading-5 text-white/55">
          {output.preview.slice(0, 400)}
          {output.preview.length > 400 ? "…" : ""}
        </pre>
      ) : null}
    </div>
  );
}

function ComposerShell({
  value,
  onChange,
  disabled,
  placeholder,
  canSubmit,
  submitting,
  footer,
  compact,
  onSubmit,
}: {
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
  placeholder: string;
  canSubmit: boolean;
  submitting: boolean;
  footer?: ReactNode;
  compact?: boolean;
  onSubmit?: () => void;
}) {
  return (
    <div
      className={cn(
        "rounded-2xl border border-white/10 bg-white/[0.02] p-2 transition focus-within:border-white/25",
        compact && "rounded-xl shadow-none",
      )}
    >
      <textarea
        value={value}
        onChange={(event) => onChange(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === "Enter" && !event.shiftKey) {
            event.preventDefault();
            if (canSubmit && !submitting) {
              onSubmit?.();
            }
          }
        }}
        rows={compact ? 1 : 3}
        disabled={disabled}
        placeholder={placeholder}
        className="block w-full resize-none bg-transparent px-3 pt-2 text-[15px] leading-7 text-white outline-none placeholder:text-white/30"
      />
      <div className="flex items-center justify-between gap-2 px-1 pb-0.5 pt-1">
        {footer ? <div className="flex flex-wrap items-center gap-2">{footer}</div> : <span />}
        <button
          type={onSubmit ? "button" : "submit"}
          onClick={onSubmit}
          disabled={!canSubmit || submitting}
          aria-label="Send"
          className="flex size-9 shrink-0 items-center justify-center rounded-full bg-white text-black transition hover:bg-white/90 disabled:cursor-not-allowed disabled:opacity-40"
        >
          {submitting ? (
            <Loader2 className="size-4 animate-spin" />
          ) : (
            <ArrowUp className="size-4" />
          )}
        </button>
      </div>
    </div>
  );
}

function AnimatedPillSelect({
  icon,
  value,
  onChange,
  options,
  disabled,
}: {
  icon: ReactNode;
  value: string;
  onChange: (value: string) => void;
  options: { value: string; label: string }[];
  disabled?: boolean;
}) {
  const selected = options.find((option) => option.value === value);

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        disabled={disabled}
        className="inline-flex items-center gap-1.5 rounded-full border border-white/10 bg-white/[0.03] py-1.5 pl-3 pr-2.5 text-sm text-white/70 transition hover:border-white/20 data-popup-open:border-white/30 data-popup-open:bg-white/[0.06] disabled:opacity-50"
      >
        <span className="text-white/45">{icon}</span>
        <span className="max-w-[9rem] truncate">{selected?.label ?? "Select"}</span>
        <ChevronDown className="size-3.5 text-white/40 transition-transform duration-200 group-data-popup-open:rotate-180" />
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="start"
        className="min-w-[12rem] border-white/10 bg-zinc-950 text-white"
      >
        {options.map((option) => (
          <DropdownMenuItem
            key={option.value || "auto"}
            onClick={() => onChange(option.value)}
            className={cn(
              "cursor-pointer text-white/75 focus:bg-white/10 focus:text-white",
              option.value === value && "bg-white/10 text-white",
            )}
          >
            {option.label}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function EvalSetupPanel({
  values,
  onChange,
}: {
  values: EvalSetupValues;
  onChange: <Key extends keyof EvalSetupValues>(
    field: Key,
    value: EvalSetupValues[Key],
  ) => void;
}) {
  return (
    <div className="mt-3 rounded-2xl border border-white/10 bg-white/[0.025] p-3">
      <div className="flex items-center gap-2">
        <ListChecks className="size-4 text-white/45" />
        <div>
          <p className="text-sm font-medium text-white/85">Eval setup</p>
          <p className="text-xs text-white/45">A short rubric is generated from these answers.</p>
        </div>
      </div>

      <div className="mt-3 grid gap-2 sm:grid-cols-2">
        <label className="rounded-xl border border-white/8 bg-black/40 px-3 py-2">
          <span className="block text-xs text-white/45">What mistake would fail this?</span>
          <input
            value={values.unacceptableMistakes}
            onChange={(event) => onChange("unacceptableMistakes", event.target.value)}
            placeholder="Inventing numbers, missing citations, off-brand tone"
            className="mt-1 w-full bg-transparent text-sm text-white outline-none placeholder:text-white/25"
          />
        </label>
        <label className="rounded-xl border border-white/8 bg-black/40 px-3 py-2">
          <span className="block text-xs text-white/45">Who would approve it?</span>
          <input
            value={values.reviewer}
            onChange={(event) => onChange("reviewer", event.target.value)}
            placeholder="Support lead, CFO, sales manager"
            className="mt-1 w-full bg-transparent text-sm text-white outline-none placeholder:text-white/25"
          />
        </label>
      </div>

      <div className="mt-3 grid gap-3 sm:grid-cols-2">
        <SegmentedControl
          label="Optimize for"
          value={values.priority}
          options={PRIORITY_OPTIONS}
          onChange={(value) => onChange("priority", value)}
        />
        <SegmentedControl
          label="Output behavior"
          value={values.style}
          options={STYLE_OPTIONS}
          onChange={(value) => onChange("style", value)}
        />
      </div>

      <label className="mt-3 flex items-center gap-2 rounded-xl border border-white/8 bg-black/40 px-3 py-2">
        <span className="shrink-0 text-xs text-white/45">Monthly volume</span>
        <input
          value={values.monthlyVolume}
          onChange={(event) => onChange("monthlyVolume", event.target.value)}
          placeholder="50, 500, 10k"
          className="w-full bg-transparent text-sm text-white outline-none placeholder:text-white/25"
        />
      </label>
    </div>
  );
}

function SegmentedControl<TValue extends string>({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: TValue;
  options: { value: TValue; label: string }[];
  onChange: (value: TValue) => void;
}) {
  return (
    <div>
      <p className="mb-1.5 text-xs text-white/45">{label}</p>
      <div className="flex flex-wrap gap-1.5">
        {options.map((option) => (
          <button
            key={option.value}
            type="button"
            onClick={() => onChange(option.value)}
            className={cn(
              "rounded-full border px-2.5 py-1 text-xs transition",
              option.value === value
                ? "border-white/35 bg-white text-black"
                : "border-white/10 bg-black/35 text-white/55 hover:border-white/25 hover:text-white",
            )}
          >
            {option.label}
          </button>
        ))}
      </div>
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
    <label className="flex items-center gap-2 rounded-xl border border-white/8 bg-zinc-950/80/55 px-3 py-1.5">
      <span className="shrink-0 text-xs text-white/45">
        {fieldLabel(field)}
        {required ? "" : " (opt)"}
      </span>
      <input
        type={spec.type === "string" ? "text" : "number"}
        value={value}
        min={spec.minimum}
        max={spec.maximum}
        onChange={(event) => onChange(field, event.target.value)}
        className="w-full bg-transparent text-sm text-white outline-none placeholder:text-white/25"
      />
    </label>
  );
}

function Alert({ text }: { text: string }) {
  return (
    <div className="mt-3 rounded-xl border border-white/15 bg-white/[0.03] p-3 text-sm text-white/70">
      {text}
    </div>
  );
}

function ScorecardCard({
  scorecard,
  compact,
}: {
  scorecard: TryoutScorecard;
  compact?: boolean;
}) {
  const pct = Math.round(scorecard.score * 100);
  return (
    <div
      className={cn(
        "rounded-xl border border-white/10 bg-white/[0.02]",
        compact ? "p-3" : "p-4",
      )}
    >
      <div className="flex items-center justify-between gap-3">
        <h3 className="text-sm font-semibold tracking-tight">Eval scorecard</h3>
        <span className="text-sm font-medium text-white/80">
          {pct}% · {scorecard.passed_validators}/{scorecard.total_validators}
        </span>
      </div>
      <div className="mt-3 h-px w-full overflow-hidden rounded-full bg-white/10">
        <div
          className="h-full rounded-full bg-white/70 transition-all duration-700"
          style={{ width: `${pct}%` }}
        />
      </div>
      {!compact && scorecard.checks.length > 0 ? (
        <ul className="mt-3 space-y-1.5">
          {scorecard.checks.map((check) => (
            <li key={check.key} className="flex items-center gap-2 text-sm">
              {check.status === "passed" ? (
                <CheckCircle2 className="size-4 shrink-0 text-white/70" />
              ) : check.status === "failed" ? (
                <XCircle className="size-4 shrink-0 text-white/35" />
              ) : (
                <span className="size-4 shrink-0 rounded-full border border-white/25" />
              )}
              <span className="text-white/75">{fieldLabel(check.key)}</span>
              <span className="ml-auto text-xs text-white/35">{check.status}</span>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}

function EvalPlanCard({
  setup,
  compact,
}: {
  setup: DerivedEvalSetup;
  compact?: boolean;
}) {
  return (
    <div
      className={cn(
        "rounded-xl border border-white/10 bg-white/[0.02]",
        compact ? "p-3" : "p-4",
      )}
    >
      <div className="flex items-center justify-between gap-3">
        <h3 className="text-sm font-semibold tracking-tight">Generated eval</h3>
        <span className="text-xs text-white/38">{priorityLabel(setup.business_priority)}</span>
      </div>
      <p className="mt-2 text-sm leading-6 text-white/58">
        Reviewer: {setup.human_reviewer}. Behavior: {styleLabel(setup.output_style)}.
      </p>
      {setup.unacceptable_mistakes ? (
        <p className="mt-1 text-sm leading-6 text-white/58">
          Fail condition: {setup.unacceptable_mistakes}
        </p>
      ) : null}
      {!compact ? (
        <ul className="mt-3 space-y-2">
          {setup.derived_rubric.slice(0, 4).map((item) => (
            <li key={item.key} className="rounded-lg border border-white/8 bg-black/35 p-2.5">
              <p className="text-sm font-medium text-white/82">{item.label}</p>
              <p className="mt-1 text-xs leading-5 text-white/48">{item.checks.join(" · ")}</p>
            </li>
          ))}
        </ul>
      ) : null}
      <p className="mt-3 text-xs leading-5 text-white/38">
        Suggested setting: {setup.suggested_generation_settings.temperature} temperature.
      </p>
    </div>
  );
}

function DownloadButton({ label, onClick }: { label: string; onClick: () => void }) {
  return (
    <Button
      type="button"
      variant="outline"
      onClick={onClick}
      className="h-7 rounded-full border-white/15 bg-transparent px-2.5 text-xs text-white hover:bg-white/10"
    >
      <Download className="size-3" />
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
    <div className="rounded-2xl border border-white/8 bg-zinc-950/80/45 p-4">
      <Icon className="size-4 text-white/35" />
      <p className="mt-3 font-medium text-white/85">{label}</p>
      <p className="mt-1 text-sm leading-6 text-white/55">{text}</p>
    </div>
  );
}

function formatInputSnapshot(input: Record<string, unknown>): string {
  if (!input || typeof input !== "object") return "";
  const preferred = ["brief", "task", "prompt", "message", "content", "description"];
  for (const key of preferred) {
    const value = input[key];
    if (typeof value === "string" && value.trim()) return value.trim();
  }
  const strings = Object.entries(input)
    .filter(([key, value]) => key !== "eval_setup" && typeof value === "string" && value.trim())
    .map(([key, value]) => `${fieldLabel(key)}: ${value}`);
  if (strings.length === 1) return strings[0].split(": ").slice(1).join(": ");
  if (strings.length > 1) return strings.join("\n\n");
  return JSON.stringify(input, null, 2);
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

function buildEvalSetup(values: EvalSetupValues, taskName: string): DerivedEvalSetup {
  const unacceptable = values.unacceptableMistakes.trim();
  const reviewer = values.reviewer.trim() || DEFAULT_EVAL_SETUP.reviewer;
  const volume = values.monthlyVolume.trim();
  const priority = values.priority;
  const style = values.style;

  const rubric: EvalRubricItem[] = [
    {
      key: "business_fit",
      label: `${reviewer} would accept it`,
      checks: [
        `Solves the ${taskName} request`,
        "Uses the supplied context instead of generic filler",
        "Makes the next human review step obvious",
      ],
    },
    {
      key: "priority_match",
      label: priorityRubricLabel(priority),
      checks: priorityRubricChecks(priority),
    },
    {
      key: "failure_mode",
      label: unacceptable ? "Avoids the named failure mode" : "Avoids obvious failure modes",
      checks: [
        unacceptable || "Does not invent facts, omit required sections, or leave placeholders",
        "Flags uncertainty instead of hiding it",
      ],
    },
    {
      key: "operational_readiness",
      label: "Ready to repeat in production",
      checks: [
        volume ? `Suitable for about ${volume} runs per month` : "Clear enough to scale beyond a demo",
        "Outputs are structured enough to validate automatically",
      ],
    },
  ];

  return {
    version: "public_tryouts_eval_setup_v1",
    unacceptable_mistakes: unacceptable,
    human_reviewer: reviewer,
    business_priority: priority,
    output_style: style,
    monthly_volume: volume,
    derived_rubric: rubric,
    suggested_generation_settings: generationSettingsFor(style, priority),
  };
}

function evalSetupFromInput(input: Record<string, unknown>): DerivedEvalSetup | null {
  const value = input?.eval_setup;
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  const setup = value as Partial<DerivedEvalSetup>;
  if (!Array.isArray(setup.derived_rubric) || setup.derived_rubric.length === 0) {
    return null;
  }
  return {
    version: typeof setup.version === "string" ? setup.version : "public_tryouts_eval_setup_v1",
    unacceptable_mistakes:
      typeof setup.unacceptable_mistakes === "string" ? setup.unacceptable_mistakes : "",
    human_reviewer:
      typeof setup.human_reviewer === "string" && setup.human_reviewer.trim()
        ? setup.human_reviewer
        : DEFAULT_EVAL_SETUP.reviewer,
    business_priority: isEvalPriority(setup.business_priority)
      ? setup.business_priority
      : DEFAULT_EVAL_SETUP.priority,
    output_style: isEvalStyle(setup.output_style)
      ? setup.output_style
      : DEFAULT_EVAL_SETUP.style,
    monthly_volume: typeof setup.monthly_volume === "string" ? setup.monthly_volume : "",
    derived_rubric: setup.derived_rubric.filter(isEvalRubricItem),
    suggested_generation_settings:
      setup.suggested_generation_settings &&
      typeof setup.suggested_generation_settings === "object" &&
      typeof setup.suggested_generation_settings.temperature === "string" &&
      typeof setup.suggested_generation_settings.reason === "string"
        ? setup.suggested_generation_settings
        : generationSettingsFor(DEFAULT_EVAL_SETUP.style, DEFAULT_EVAL_SETUP.priority),
  };
}

function isEvalPriority(value: unknown): value is EvalPriority {
  return PRIORITY_OPTIONS.some((option) => option.value === value);
}

function isEvalStyle(value: unknown): value is EvalStyle {
  return STYLE_OPTIONS.some((option) => option.value === value);
}

function isEvalRubricItem(value: unknown): value is EvalRubricItem {
  if (!value || typeof value !== "object" || Array.isArray(value)) return false;
  const item = value as Partial<EvalRubricItem>;
  return (
    typeof item.key === "string" &&
    typeof item.label === "string" &&
    Array.isArray(item.checks) &&
    item.checks.every((check) => typeof check === "string")
  );
}

function generationSettingsFor(
  style: EvalStyle,
  priority: EvalPriority,
): DerivedEvalSetup["suggested_generation_settings"] {
  if (style === "creative" && priority !== "compliance") {
    return {
      temperature: "medium",
      reason: "Allow more variation because the user values creative alternatives.",
    };
  }
  if (priority === "speed" || priority === "cost") {
    return {
      temperature: "low",
      reason: "Keep outputs predictable so cheaper or faster models can be compared fairly.",
    };
  }
  return {
    temperature: "low",
    reason: "Prefer repeatable outputs because the rubric depends on correctness and reviewer trust.",
  };
}

function priorityRubricLabel(priority: EvalPriority): string {
  switch (priority) {
    case "polish":
      return "Looks client-ready";
    case "speed":
      return "Finishes quickly enough";
    case "cost":
      return "Makes economic sense";
    case "compliance":
      return "Stays inside policy";
    default:
      return "Gets the facts right";
  }
}

function priorityRubricChecks(priority: EvalPriority): string[] {
  switch (priority) {
    case "polish":
      return ["Clear structure", "Tone fits the audience", "No rough-draft leftovers"];
    case "speed":
      return ["Completes without extra back-and-forth", "Avoids unnecessary tool calls"];
    case "cost":
      return ["Avoids wasteful retries", "Output quality justifies model cost"];
    case "compliance":
      return ["Follows supplied policies", "Calls out risky or missing information"];
    default:
      return ["Grounded in user input", "No fabricated claims", "Important details preserved"];
  }
}

function priorityLabel(priority: EvalPriority): string {
  return PRIORITY_OPTIONS.find((option) => option.value === priority)?.label ?? "Correct facts";
}

function styleLabel(style: EvalStyle): string {
  return STYLE_OPTIONS.find((option) => option.value === style)?.label ?? "Same every time";
}

function friendlyTraceSummary(event: TryoutTimelineEvent): string {
  const summary = event.summary.trim();
  const lower = summary.toLowerCase();
  switch (event.type) {
    case "planning":
      return "Planned the next step";
    case "tool_call":
      return lower.includes("complete") ? "Finished a tool step" : "Used a tool";
    case "sandbox_command":
      return lower.includes("soffice") || lower.includes("libreoffice")
        ? "Exported the deck preview"
        : "Ran a sandbox command";
    case "file_written":
    case "file_activity":
      return summary.replace(/^wrote file:?/i, "Created").replace(/^file written:?/i, "Created");
    case "validation":
      return "Checked the output against validators";
    case "scoring":
      return "Updated the eval scorecard";
    default:
      return summary || "Working";
  }
}

function fieldLabel(field: string): string {
  const spaced = field.replaceAll("_", " ");
  return spaced.charAt(0).toUpperCase() + spaced.slice(1);
}

function triggerDownload(filename: string, contents: string, mime: string) {
  triggerDownloadBlob(filename, new Blob([contents], { type: mime }));
}

function triggerDownloadBlob(filename: string, blob: Blob) {
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
    output.relative_path || `${output.key || "artifact"}.${extForType(output.type, output.relative_path)}`;
  const mime = output.content_type || mimeForType(output.type, output.relative_path);
  triggerDownloadBlob(name.split("/").pop() || name, new Blob([decodeArtifactBytes(output)], { type: mime }));
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

function extForType(type: string, relativePath?: string): string {
  const fromPath = relativePath?.includes(".") ? relativePath.split(".").pop() : "";
  if (fromPath) return fromPath;
  switch (type) {
    case "json":
      return "json";
    case "markdown":
      return "md";
    case "csv":
      return "csv";
    case "pdf":
      return "pdf";
    case "pptx":
      return "pptx";
    case "png":
      return "png";
    default:
      return "txt";
  }
}

function mimeForType(type: string, relativePath?: string): string {
  const ext = extForType(type, relativePath).toLowerCase();
  switch (ext) {
    case "pdf":
      return "application/pdf";
    case "pptx":
      return "application/vnd.openxmlformats-officedocument.presentationml.presentation";
    case "png":
      return "image/png";
    case "jpg":
    case "jpeg":
      return "image/jpeg";
  }
  switch (type) {
    case "json":
      return "application/json";
    case "markdown":
      return "text/markdown";
    case "csv":
      return "text/csv";
    default:
      return "text/plain";
  }
}

type TryoutOutputPreview = {
  key: string;
  type: string;
  relative_path: string;
  preview: string;
  truncated: boolean;
  encoding?: "utf-8" | "base64";
  content_type?: string;
  size_bytes?: number;
};

function isBinaryArtifact(output: TryoutOutputPreview): boolean {
  return output.encoding === "base64" || ["pdf", "pptx", "png", "jpg", "jpeg"].includes(output.type);
}

function isPdfArtifact(output: TryoutOutputPreview): boolean {
  return output.type === "pdf" || output.content_type?.includes("pdf") === true;
}

function isImageArtifact(output: TryoutOutputPreview): boolean {
  return (
    output.type === "png" ||
    output.content_type?.startsWith("image/") === true
  );
}

function artifactKindLabel(output: TryoutOutputPreview): string {
  if (output.type === "pptx" || output.relative_path.endsWith(".pptx")) return "PowerPoint";
  if (isPdfArtifact(output)) return "PDF";
  if (isImageArtifact(output)) return "Image";
  if (output.type === "json") return "Metadata";
  if (output.type === "markdown") return "Markdown";
  return "File";
}

function decodeArtifactBytes(output: TryoutOutputPreview): ArrayBuffer {
  if (output.encoding === "base64") {
    const binary = atob(output.preview);
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i += 1) {
      bytes[i] = binary.charCodeAt(i);
    }
    return bytes.buffer;
  }
  return new TextEncoder().encode(output.preview).buffer;
}

function artifactDataUrl(output: TryoutOutputPreview): string | null {
  if (!isPdfArtifact(output) && !isImageArtifact(output)) {
    return null;
  }
  const mime = output.content_type || mimeForType(output.type, output.relative_path);
  if (output.encoding === "base64") {
    return `data:${mime};base64,${output.preview}`;
  }
  const bytes = new TextEncoder().encode(output.preview);
  let binary = "";
  for (let i = 0; i < bytes.length; i += 1) {
    binary += String.fromCharCode(bytes[i]);
  }
  return `data:${mime};base64,${btoa(binary)}`;
}

function tryoutOutputs(summary: unknown): TryoutOutputPreview[] {
  if (!summary || typeof summary !== "object") return [];
  const outputs = (summary as { outputs?: unknown }).outputs;
  if (!Array.isArray(outputs)) return [];
  return outputs
    .map((item): TryoutOutputPreview | null => {
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
        encoding:
          output.encoding === "base64" || output.encoding === "utf-8"
            ? output.encoding
            : undefined,
        content_type:
          typeof output.content_type === "string" ? output.content_type : undefined,
        size_bytes:
          typeof output.size_bytes === "number" ? output.size_bytes : undefined,
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

type RoiAnchor = {
  label: string;
  humanCostPerTask: number;
  aiCostPerTask: number;
  errorRate: number;
  costPerError: number;
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

const UNEVALUATED_FAILURE_RATE = 0.95;
const EVAL_ANNUAL_COST = 12000;

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

  const automationSavings =
    annualVolume * (humanCostPerTask - anchor.aiCostPerTask);
  const annualErrorCost = annualVolume * anchor.errorRate * anchor.costPerError;
  const forfeitedSavings = automationSavings * UNEVALUATED_FAILURE_RATE;
  const costWithoutEvals = annualErrorCost + forfeitedSavings;
  const capturedSavings = automationSavings - EVAL_ANNUAL_COST;
  const netUpside = capturedSavings + costWithoutEvals;

  return (
    <div className="rounded-xl border border-white/10 bg-white/[0.02] p-5">
      <div className="flex items-center gap-2">
        <Calculator className="size-4 text-white/40" />
        <h3 className="text-sm font-semibold tracking-tight">
          What this is worth at your scale
        </h3>
      </div>
      <p className="mt-1.5 text-sm leading-6 text-white/50">
        You just watched an agent finish one {anchor.label}. Here is the business
        case for evaluating it before you wire it into production.
      </p>

      <div className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <RoiInput label="Company" value={company} onChange={setCompany} placeholder="Acme Inc" />
        <RoiInput label="Work email" value={email} onChange={setEmail} placeholder="you@acme.com" />
        <RoiInput label={`${anchor.label}s / month`} value={volume} onChange={setVolume} numeric />
        <RoiInput label="Human $ / task" value={humanCost} onChange={setHumanCost} numeric />
      </div>

      <div className="mt-4 grid gap-3 sm:grid-cols-2">
        <div className="rounded-xl border border-white/10 bg-white/[0.03] p-4">
          <div className="flex items-center gap-2 text-white/55">
            <ShieldAlert className="size-4" />
            <p className="text-sm font-medium">Integrate without evals</p>
          </div>
          <p className="mt-2 text-3xl font-semibold tracking-tight text-white">
            {usd(costWithoutEvals)}
            <span className="text-base font-normal text-white/40"> /yr at risk</span>
          </p>
        </div>

        <div className="rounded-xl border border-white/15 bg-white/[0.04] p-4">
          <div className="flex items-center gap-2 text-white/70">
            <TrendingUp className="size-4" />
            <p className="text-sm font-medium">Evaluate with AgentClash</p>
          </div>
          <p className="mt-2 text-3xl font-semibold tracking-tight text-white">
            {usd(capturedSavings)}
            <span className="text-base font-normal text-white/40"> /yr captured</span>
          </p>
        </div>
      </div>

      <div className="mt-4 flex flex-col items-start justify-between gap-3 rounded-xl border border-white/10 bg-black/40 p-4 sm:flex-row sm:items-center">
        <p className="text-sm text-white/65">
          Evaluating first is worth{" "}
          <span className="font-semibold text-white">{usd(netUpside)}/yr</span>{" "}
          to {company.trim() || "your team"} on this workflow alone.
        </p>
        <Link
          href={`/enterprise?from=tryout&task=${encodeURIComponent(tryout.template_slug)}${email.trim() ? `&email=${encodeURIComponent(email.trim())}` : ""}`}
          className="inline-flex h-9 shrink-0 items-center gap-1.5 rounded-full bg-white px-4 text-sm font-medium text-black transition hover:bg-white/90"
        >
          Talk to us about integrating
          <ArrowRight className="size-4" />
        </Link>
      </div>
      <p className="mt-2 text-xs text-white/35">
        Adjust the inputs to match your numbers.{" "}
        <Link href={loginHref} className="text-white/55 hover:underline">
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
      <span className="mb-1 block text-xs text-white/45">{label}</span>
      <input
        type={numeric ? "number" : "text"}
        inputMode={numeric ? "decimal" : undefined}
        min={numeric ? 0 : undefined}
        value={value}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
        className="h-9 w-full rounded-lg border border-white/10 bg-white/[0.03] px-3 text-sm text-white outline-none placeholder:text-white/25 focus:border-white/25 focus:ring-1 focus:ring-white/10"
      />
    </label>
  );
}
