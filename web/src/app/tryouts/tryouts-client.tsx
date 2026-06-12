"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import type { FormEvent, ReactNode } from "react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  ArrowRight,
  ArrowUp,
  ChevronDown,
  Download,
  FileText,
  Gauge,
  ListChecks,
  Loader2,
  PanelRight,
  Scale,
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
  AgentTryoutJudgeStrictness,
  AgentTryoutModelPolicy,
  AgentTryoutTemplate,
  TryoutTimelineEvent,
} from "@/lib/api/types";
import {
  formatTryoutCost,
  formatTryoutLatency,
  tryoutIsActive,
} from "@/lib/agent-tryout-status";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";
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

// Judge models must match the backend's hosted allowlist
// (defaultPublicJudgeModels). Judges always run on platform keys.
const JUDGE_OPTIONS = [
  { value: "gpt-5-mini", label: "GPT-5 mini" },
  { value: "claude-haiku-4-5", label: "Claude Haiku" },
  { value: "gemini-2.5-flash", label: "Gemini Flash" },
];

const STRICTNESS_OPTIONS: { value: AgentTryoutJudgeStrictness; label: string }[] = [
  { value: "lenient", label: "Lenient" },
  { value: "standard", label: "Standard" },
  { value: "harsh", label: "Harsh" },
];

function judgeModelLabel(model: string): string {
  return JUDGE_OPTIONS.find((option) => option.value === model)?.label ?? model;
}

// Cross-page handoff for the "run it again" funnel loop: the verdict screen
// stashes the brief + eval answers here, and the welcome screen prefills from
// it so a rerun takes one click instead of re-typing everything.
const RERUN_STORAGE_KEY = "agentclash.tryout.rerun";

type RerunPrefill = {
  templateSlug?: string;
  fieldValues?: Record<string, string>;
  evalSetup?: Partial<EvalSetupValues>;
  judgeModel?: string;
  judgeStrictness?: AgentTryoutJudgeStrictness;
  selectedModelKey?: string;
};

function readRerunPrefill(): RerunPrefill | null {
  if (typeof window === "undefined") return null;
  try {
    const raw = window.sessionStorage.getItem(RERUN_STORAGE_KEY);
    if (!raw) return null;
    window.sessionStorage.removeItem(RERUN_STORAGE_KEY);
    return JSON.parse(raw) as RerunPrefill;
  } catch {
    return null;
  }
}

function writeRerunPrefill(prefill: RerunPrefill) {
  try {
    window.sessionStorage.setItem(RERUN_STORAGE_KEY, JSON.stringify(prefill));
  } catch {
    // best-effort
  }
}

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
    "Weekly product update for leadership: wins, slips, and what needs a decision.",
  ],
  "meeting-minutes": [
    "Summarize these notes into minutes with owners and due dates.",
    "Extract action items and render them as a one-page PDF checklist.",
    "Board meeting notes: decisions, risks, and follow-ups for the exec team.",
  ],
  "inbox-triage": [
    "Prioritize these 8 emails and draft replies for the urgent ones.",
    "Sort this inbox by urgency, flag compliance risks, and draft holds for anything ambiguous.",
    "Customer escalation thread plus three routine billing questions: triage and draft responses.",
  ],
};

const GENERIC_SUGGESTIONS = [
  "Generate a PDF report from this data with a chart.",
  "Build a spreadsheet with formulas and a summary tab.",
  "Turn this into a labeled bar chart and explain the trend.",
  "Summarize this for an executive who has two minutes to read.",
  "Flag anything that would fail a compliance review before we ship.",
];

type TryoutShowcaseItem = {
  slug: string;
  tag: string;
  headline: string;
  example: string;
  primaryField: string;
};

const TRYOUT_SHOWCASE: TryoutShowcaseItem[] = [
  {
    slug: "support-ticket-resolution",
    tag: "Customer support",
    headline: "Resolve support tickets",
    example:
      "Customer says their invoice was charged twice and wants a refund today. Draft a reply and decide whether to escalate.",
    primaryField: "ticket",
  },
  {
    slug: "document-extraction",
    tag: "Finance / AP",
    headline: "Extract invoice data",
    example:
      "Extract line items, totals, and vendor from this invoice, then flag any missing fields for human review.",
    primaryField: "document",
  },
  {
    slug: "contract-review",
    tag: "Legal ops",
    headline: "Review contract clauses",
    example:
      "Review this NDA for one-sided indemnity and unlimited liability. List risks with severity and quote the clause.",
    primaryField: "contract",
  },
  {
    slug: "sdr-outreach",
    tag: "Sales",
    headline: "Qualify leads & draft outreach",
    example:
      "VP Engineering at a 200-person SaaS company, hiring 3 platform engineers. Draft a 3-sentence cold email for our eval platform.",
    primaryField: "prospect",
  },
  {
    slug: "inbox-triage",
    tag: "Ops",
    headline: "Triage inbox batches",
    example:
      "Prioritize these 8 emails, draft replies for urgent ones, and flag anything that needs a human before we respond.",
    primaryField: "emails",
  },
  {
    slug: "meeting-minutes",
    tag: "Product / PM",
    headline: "Minutes to action plan",
    example:
      "Summarize these notes into minutes with owners, due dates, and risks. Export a one-page action plan.",
    primaryField: "notes",
  },
  {
    slug: "status-report",
    tag: "Leadership",
    headline: "Status reports",
    example:
      "Turn these scattered sprint updates into a polished weekly status with highlights, risks, and next steps.",
    primaryField: "updates",
  },
  {
    slug: "spreadsheet-builder",
    tag: "Analytics",
    headline: "Data to spreadsheet",
    example:
      "Turn this raw sales data into a spreadsheet with a pivot summary and a bar chart PNG.",
    primaryField: "data",
  },
  {
    slug: "slide-deck",
    tag: "Marketing",
    headline: "Brief to slide deck",
    example:
      "Make a 6-slide deck explaining our AI evaluation platform for a VP of Engineering, with one chart slide.",
    primaryField: "brief",
  },
];

function suggestionsFor(slug: string): string[] {
  const showcase = TRYOUT_SHOWCASE.find((item) => item.slug === slug);
  const curated = TASK_SUGGESTIONS[slug] ?? GENERIC_SUGGESTIONS;
  if (showcase && !curated.includes(showcase.example)) {
    return [showcase.example, ...curated];
  }
  return curated;
}

function showcaseForTemplates(templates: AgentTryoutTemplate[]): TryoutShowcaseItem[] {
  const slugs = new Set(templates.map((template) => template.slug));
  return TRYOUT_SHOWCASE.filter((item) => slugs.has(item.slug));
}

const api = createApiClient();

const SERIF = "[font-family:var(--font-race-display)]";
const MICRO = "font-mono text-2xs uppercase tracking-[0.22em]";

export function PublicTryoutsClient() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const urlTryoutId = searchParams.get("tryout") ?? "";

  const [templates, setTemplates] = useState<AgentTryoutTemplate[]>([]);
  const [templateSlug, setTemplateSlug] = useState("");
  const [selectedModelKey, setSelectedModelKey] = useState("auto");
  const [judgeModel, setJudgeModel] = useState(JUDGE_OPTIONS[0].value);
  const [judgeStrictness, setJudgeStrictness] =
    useState<AgentTryoutJudgeStrictness>("standard");
  const [fieldValues, setFieldValues] = useState<Record<string, string>>({});
  const [evalSetup, setEvalSetup] = useState<EvalSetupValues>(DEFAULT_EVAL_SETUP);
  const prefillRef = useRef<RerunPrefill | null>(null);
  const showcasePrefillRef = useRef<{
    slug: string;
    values: Record<string, string>;
  } | null>(null);

  // Apply a rerun handoff (same brief, different agent/judge) exactly once.
  useEffect(() => {
    if (urlTryoutId) return;
    const prefill = readRerunPrefill();
    if (!prefill) return;
    prefillRef.current = prefill;
    if (prefill.templateSlug) setTemplateSlug(prefill.templateSlug);
    if (prefill.evalSetup) {
      setEvalSetup((current) => ({ ...current, ...prefill.evalSetup }));
    }
    if (prefill.judgeModel) setJudgeModel(prefill.judgeModel);
    if (prefill.judgeStrictness) setJudgeStrictness(prefill.judgeStrictness);
    if (prefill.selectedModelKey) setSelectedModelKey(prefill.selectedModelKey);
    if (prefill.fieldValues) setFieldValues(prefill.fieldValues);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
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
    const pending = prefillRef.current;
    if (
      pending?.fieldValues &&
      (!pending.templateSlug || pending.templateSlug === templateSlug)
    ) {
      setFieldValues(pending.fieldValues);
      prefillRef.current = null;
      return;
    }
    const showcasePending = showcasePrefillRef.current;
    if (showcasePending && showcasePending.slug === templateSlug) {
      setFieldValues(showcasePending.values);
      showcasePrefillRef.current = null;
      return;
    }
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

  const evalReady =
    evalSetup.unacceptableMistakes.trim().length > 0 &&
    evalSetup.monthlyVolume.trim().length > 0;

  async function launchTryout() {
    if (!template || launching || !evalReady) return;

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
      const payload = {
        template_slug: template.slug,
        input: input.value,
        ...(selectedModel.value ? { selected_harness_kind: selectedModel.value } : {}),
        ...(selectedPolicy ? { selected_model_policy: selectedPolicy } : {}),
        judge: { model: judgeModel, strictness: judgeStrictness },
      };
      let nextTryout: AgentTryout;
      try {
        nextTryout = await createAnonymousAgentTryout(api, payload);
      } catch (err) {
        // Backends that predate judge selection reject unknown fields; fall
        // back to a judgeless run instead of breaking the page.
        if (
          err instanceof ApiError &&
          err.status === 400 &&
          err.message.toLowerCase().includes("judge")
        ) {
          const withoutJudge = { ...payload, judge: undefined };
          nextTryout = await createAnonymousAgentTryout(api, withoutJudge);
        } else {
          throw err;
        }
      }
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
  const showcaseItems = useMemo(() => showcaseForTemplates(templates), [templates]);

  const selectShowcase = useCallback((item: TryoutShowcaseItem) => {
    showcasePrefillRef.current = {
      slug: item.slug,
      values: { [item.primaryField]: item.example },
    };
    setTemplateSlug(item.slug);
  }, []);

  return (
    <main className="flex h-[100dvh] flex-col overflow-hidden bg-[#131312] text-white">
      <header className="relative z-10 flex shrink-0 items-center justify-between gap-4 border-b border-white/[0.07] px-4 py-3 sm:px-6">
        <div className="flex items-baseline gap-4">
          <Link
            href="/"
            className="text-sm font-semibold tracking-tight text-white/90"
          >
            AgentClash
          </Link>
          {tryout ? (
            <span className={cn(MICRO, "hidden text-white/35 sm:inline")}>
              {tryout.status}
            </span>
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
            className="rounded-sm border border-white/15 px-3 py-1.5 text-sm text-white/80 transition hover:border-white/40 hover:text-white"
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
          judgeModel={judgeModel}
          setJudgeModel={setJudgeModel}
          judgeStrictness={judgeStrictness}
          setJudgeStrictness={setJudgeStrictness}
          primaryField={primaryField}
          secondaryFields={secondaryFields}
          fieldValues={fieldValues}
          updateField={updateField}
          evalSetup={evalSetup}
          updateEvalSetup={updateEvalSetup}
          evalReady={evalReady}
          primaryValue={primaryValue}
          canRun={canRun}
          launching={launching}
          templatesLoading={templatesLoading}
          message={message}
          quotaMessage={quotaMessage}
          loginHref={loginHref}
          showcaseItems={showcaseItems}
          onSelectShowcase={selectShowcase}
          onLaunch={() => void launchTryout()}
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
  judgeModel,
  setJudgeModel,
  judgeStrictness,
  setJudgeStrictness,
  primaryField,
  secondaryFields,
  fieldValues,
  updateField,
  evalSetup,
  updateEvalSetup,
  evalReady,
  primaryValue,
  canRun,
  launching,
  templatesLoading,
  message,
  quotaMessage,
  loginHref,
  showcaseItems,
  onSelectShowcase,
  onLaunch,
}: {
  template: AgentTryoutTemplate | null;
  templates: AgentTryoutTemplate[];
  templateSlug: string;
  setTemplateSlug: (value: string) => void;
  selectedModelKey: string;
  setSelectedModelKey: (value: string) => void;
  judgeModel: string;
  setJudgeModel: (value: string) => void;
  judgeStrictness: AgentTryoutJudgeStrictness;
  setJudgeStrictness: (value: AgentTryoutJudgeStrictness) => void;
  primaryField: [string, FieldSpec] | null;
  secondaryFields: [string, FieldSpec][];
  fieldValues: Record<string, string>;
  updateField: (field: string, value: string) => void;
  evalSetup: EvalSetupValues;
  updateEvalSetup: <Key extends keyof EvalSetupValues>(
    field: Key,
    value: EvalSetupValues[Key],
  ) => void;
  evalReady: boolean;
  primaryValue: string;
  canRun: boolean;
  launching: boolean;
  templatesLoading: boolean;
  message: string | null;
  quotaMessage: string | null;
  loginHref: string;
  showcaseItems: TryoutShowcaseItem[];
  onSelectShowcase: (item: TryoutShowcaseItem) => void;
  onLaunch: () => void;
}) {
  const [barOpen, setBarOpen] = useState(false);
  // True when the bar dialog was opened by a send attempt: confirming the bar
  // should immediately launch the run instead of dropping back to the page.
  const [launchAfterBar, setLaunchAfterBar] = useState(false);

  const enter =
    "animate-in fade-in slide-in-from-bottom-3 fill-mode-both duration-700 motion-reduce:animate-none";

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!canRun) return;
    if (!evalReady) {
      setLaunchAfterBar(true);
      setBarOpen(true);
      return;
    }
    onLaunch();
  }

  return (
    <div className="relative flex min-h-0 flex-1 flex-col overflow-y-auto">
      <div className="mx-auto flex min-h-0 w-full max-w-2xl flex-1 flex-col justify-center px-5 py-10 sm:px-6 sm:py-12">
        <p
          className={cn(MICRO, "text-center text-white/35", enter)}
          style={{ animationDelay: "0ms" }}
        >
          Free AI agent evaluation · Sandboxed · No signup
        </p>
        <h1
          className={cn(
            "mx-auto mt-5 max-w-[28ch] text-center font-sans text-[clamp(2rem,4.8vw,3.25rem)] font-semibold leading-[1.08] tracking-tight text-white/95",
            enter,
          )}
          style={{ animationDelay: "90ms" }}
        >
          Test AI agents on real business work before you integrate them
        </h1>
        <p
          className={cn(
            "mx-auto mt-4 max-w-xl text-center text-base leading-7 text-white/50",
            enter,
          )}
          style={{ animationDelay: "180ms" }}
        >
          Run a free sandboxed tryout on customer support, finance, legal, sales,
          and ops workflows. Set the quality bar your team would enforce in
          production, pick a judge, and get a scored verdict with artifacts you
          can share before an AI pilot or automation rollout.
        </p>
        <ul className="sr-only">
          {showcaseItems.map((item) => (
            <li key={item.slug}>
              {item.headline}: {item.tag} AI workflow automation example
            </li>
          ))}
        </ul>

        <form
          onSubmit={handleSubmit}
          className={cn("mt-8", enter)}
          style={{ animationDelay: "270ms" }}
        >
          <ComposerShell
            value={primaryValue}
            onChange={(value) => primaryField && updateField(primaryField[0], value)}
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
                <BarPill ready={evalReady} onClick={() => setBarOpen(true)} />
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
                <AnimatedPillSelect
                  icon={<Scale className="size-3.5" />}
                  value={judgeModel}
                  onChange={setJudgeModel}
                  options={JUDGE_OPTIONS.map((option) => ({
                    value: option.value,
                    label: `${option.label} judges`,
                  }))}
                />
              </>
            }
          />

          {secondaryFields.length > 0 ? (
            <div className="mt-4 grid gap-x-10 gap-y-5 sm:grid-cols-2">
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

          {message ? <Alert text={message} /> : null}
          {quotaMessage ? (
            <div className="mt-6 border border-white/25 p-5">
              <p className={cn(MICRO, "text-white/45")}>Free runs used</p>
              <p className="mt-2 text-sm leading-6 text-white/55">{quotaMessage}</p>
              <p className="mt-2 text-sm leading-6 text-white/70">
                Your bar is already written. Sign up free to keep grading agents
                against it, and to save every verdict.
              </p>
              <Link
                href={loginHref}
                className="mt-4 inline-flex h-9 items-center gap-1.5 rounded-sm bg-white px-4 text-sm font-medium text-black transition hover:bg-white/90"
              >
                Keep grading
                <ArrowRight className="size-4" />
              </Link>
            </div>
          ) : null}
        </form>

        {template && primaryField ? (
          <div className={cn("mt-9", enter)} style={{ animationDelay: "360ms" }}>
            <p className={cn(MICRO, "text-center text-white/30")}>
              Or steal one of these prompts
            </p>
            <ul className="mt-3 divide-y divide-white/[0.07] border-y border-white/[0.07]">
              {suggestionsFor(template.slug).slice(0, 5).map((suggestion) => (
                <li key={suggestion}>
                  <button
                    type="button"
                    onClick={() => updateField(primaryField[0], suggestion)}
                    className="group flex w-full items-baseline gap-3 py-2.5 text-left text-sm leading-6 text-white/50 transition hover:text-white"
                  >
                    <span className="font-mono text-2xs text-white/25 transition group-hover:text-white/60">
                      →
                    </span>
                    {suggestion}
                  </button>
                </li>
              ))}
            </ul>
          </div>
        ) : null}
      </div>

      {showcaseItems.length > 0 ? (
        <TryoutTemplateMarquee
          items={showcaseItems}
          activeSlug={templateSlug}
          onSelect={onSelectShowcase}
        />
      ) : null}

      <BarDialog
        open={barOpen}
        onOpenChange={(open) => {
          setBarOpen(open);
          if (!open) setLaunchAfterBar(false);
        }}
        values={evalSetup}
        onChange={updateEvalSetup}
        strictness={judgeStrictness}
        setStrictness={setJudgeStrictness}
        judgeModel={judgeModel}
        evalReady={evalReady}
        willLaunch={launchAfterBar}
        onConfirm={() => {
          if (!evalReady) return;
          setBarOpen(false);
          if (launchAfterBar) {
            setLaunchAfterBar(false);
            onLaunch();
          }
        }}
      />
    </div>
  );
}

function TryoutTemplateMarquee({
  items,
  activeSlug,
  onSelect,
}: {
  items: TryoutShowcaseItem[];
  activeSlug: string;
  onSelect: (item: TryoutShowcaseItem) => void;
}) {
  const loop = [...items, ...items];

  return (
    <section
      className="shrink-0 border-t border-white/[0.07] bg-[#0f0f0e]/80 pb-6 pt-5"
      aria-label="AI workflow templates to try"
    >
      <div className="mx-auto max-w-6xl px-5 sm:px-6">
        <p className={cn(MICRO, "text-center text-white/30")}>
          Business workflows you can test today
        </p>
      </div>
      <div className="relative mt-4 overflow-hidden">
        <div
          className="pointer-events-none absolute inset-y-0 left-0 z-10 w-12 bg-gradient-to-r from-[#0f0f0e] to-transparent sm:w-20"
          aria-hidden
        />
        <div
          className="pointer-events-none absolute inset-y-0 right-0 z-10 w-12 bg-gradient-to-l from-[#0f0f0e] to-transparent sm:w-20"
          aria-hidden
        />
        <div
          className="flex w-max gap-3 motion-reduce:overflow-x-auto motion-reduce:px-5 motion-reduce:pb-1 motion-safe:animate-[tryout-marquee_55s_linear_infinite]"
        >
          {loop.map((item, index) => (
            <button
              key={`${item.slug}-${index}`}
              type="button"
              onClick={() => onSelect(item)}
              className={cn(
                "group w-[17rem] shrink-0 rounded-sm border px-4 py-3 text-left transition sm:w-[19rem]",
                item.slug === activeSlug
                  ? "border-white/35 bg-white/[0.06]"
                  : "border-white/10 bg-white/[0.02] hover:border-white/25 hover:bg-white/[0.04]",
              )}
            >
              <span className={cn(MICRO, "text-white/35")}>{item.tag}</span>
              <p className="mt-2 text-sm font-medium leading-snug text-white/90">
                {item.headline}
              </p>
              <p className="mt-2 line-clamp-2 text-xs leading-5 text-white/45 transition group-hover:text-white/60">
                {item.example}
              </p>
            </button>
          ))}
        </div>
      </div>
    </section>
  );
}

// BarPill is the composer's entry to the eval gate: it shows whether the bar
// is set and opens the dialog to write or edit it.
function BarPill({ ready, onClick }: { ready: boolean; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "inline-flex items-center gap-1.5 rounded-sm border py-1.5 pl-2.5 pr-2.5 font-mono text-2xs transition",
        ready
          ? "border-white/12 text-white/60 hover:border-white/30 hover:text-white/90"
          : "border-white/35 text-white/90 hover:border-white/60",
      )}
    >
      <ListChecks className="size-3.5" />
      {ready ? "Bar set" : "Set the bar"}
      {!ready ? (
        <span
          aria-hidden
          className="size-1 rounded-full bg-white/80 animate-pulse motion-reduce:animate-none"
        />
      ) : null}
    </button>
  );
}

// BarDialog is the eval gate: a single modal where the visitor writes the bar
// the judge will grade against. It blocks the first run until the two
// required answers exist, then gets out of the way.
function BarDialog({
  open,
  onOpenChange,
  values,
  onChange,
  strictness,
  setStrictness,
  judgeModel,
  evalReady,
  willLaunch,
  onConfirm,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  values: EvalSetupValues;
  onChange: <Key extends keyof EvalSetupValues>(
    field: Key,
    value: EvalSetupValues[Key],
  ) => void;
  strictness: AgentTryoutJudgeStrictness;
  setStrictness: (value: AgentTryoutJudgeStrictness) => void;
  judgeModel: string;
  evalReady: boolean;
  willLaunch: boolean;
  onConfirm: () => void;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        showCloseButton={false}
        className="max-h-[85dvh] gap-0 overflow-y-auto rounded-sm border border-white/12 bg-[#1a1a19] p-6 text-white ring-0 sm:max-w-lg"
      >
        <p className={cn(MICRO, "text-white/40")}>Before the agent runs</p>
        <DialogTitle
          className={cn(SERIF, "mt-3 text-3xl font-light tracking-tight text-white/95")}
        >
          What would make you{" "}
          <em className="italic text-white/55">reject</em> this work?
        </DialogTitle>
        <DialogDescription className="mt-3 text-sm leading-6 text-white/50">
          Your answers become the judge&apos;s instructions, word for word.
          That is an eval — and {judgeModelLabel(judgeModel)} will enforce it.
        </DialogDescription>

        <div className="mt-6 space-y-6">
          <UnderlineField
            label="The mistake you would not forgive"
            required
            value={values.unacceptableMistakes}
            onChange={(value) => onChange("unacceptableMistakes", value)}
            placeholder="Invented numbers, missing citations, off-brand tone"
          />
          <div className="grid gap-x-8 gap-y-6 sm:grid-cols-2">
            <UnderlineField
              label="The person who signs off"
              value={values.reviewer}
              onChange={(value) => onChange("reviewer", value)}
              placeholder="Support lead, CFO, sales manager"
            />
            <UnderlineField
              label="Runs per month"
              required
              value={values.monthlyVolume}
              onChange={(value) => onChange("monthlyVolume", value)}
              placeholder="50, 500, 10k"
            />
          </div>
          <div className="grid gap-x-8 gap-y-6 sm:grid-cols-2">
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
          <SegmentedControl
            label="How harshly the judge grades"
            value={strictness}
            options={STRICTNESS_OPTIONS}
            onChange={setStrictness}
          />
        </div>

        <div className="mt-7 flex items-center justify-between gap-4 border-t border-white/[0.08] pt-4">
          <button
            type="button"
            onClick={() => onOpenChange(false)}
            className="font-mono text-2xs uppercase tracking-[0.12em] text-white/35 transition hover:text-white/70"
          >
            Not yet
          </button>
          <button
            type="button"
            onClick={onConfirm}
            disabled={!evalReady}
            className="inline-flex h-9 items-center gap-1.5 rounded-sm bg-white px-4 text-sm font-medium text-black transition hover:bg-white/90 disabled:cursor-not-allowed disabled:opacity-40"
          >
            {willLaunch ? "Lock the bar and run" : "Lock the bar"}
            <ArrowRight className="size-4" />
          </button>
        </div>
      </DialogContent>
    </Dialog>
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
      <aside className="hidden w-80 shrink-0 flex-col border-r border-white/10 lg:flex">
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
            className="h-8 rounded-sm border-white/15 bg-transparent text-white hover:bg-white/10 lg:hidden"
          />
        }
      >
        <PanelRight className="size-3.5" />
        Trace
      </SheetTrigger>
      <SheetContent
        side="right"
        className="w-full border-white/10 bg-[#131312] text-white sm:max-w-md"
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
      <div className="space-y-2 px-1">
        <p className={cn(MICRO, "text-white/55")}>{tryout.template_slug}</p>
        <p className="text-xs text-white/40">
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
          <VerdictCard scorecard={scorecard} compact />
        </div>
      ) : null}

      {evalPlan ? (
        <div className="mt-4 px-1">
          <EvalPlanCard setup={evalPlan} compact />
        </div>
      ) : null}

      {outputs.length > 0 ? (
        <div className="mt-5 min-h-0 px-1">
          <p className={cn(MICRO, "mb-3 text-white/35")}>Artifacts</p>
          <div className="space-y-2">
            {outputs.map((output) => (
              <ArtifactPreviewCard key={`${output.key}-${output.relative_path}`} output={output} />
            ))}
          </div>
        </div>
      ) : null}

      <div className="mt-5 flex min-h-0 flex-1 flex-col px-1">
        <p className={cn(MICRO, "mb-3 text-white/35")}>Raw event log</p>
        <ol className="ml-1 min-h-0 flex-1 space-y-3 overflow-y-auto border-l border-white/10 pl-4">
          {events.length === 0 ? (
            <li className="text-xs text-white/40">
              {tryoutIsActive(tryout.status)
                ? "Waiting for events…"
                : "No events recorded."}
            </li>
          ) : (
            events.map((event) => (
              <li key={event.cursor} className="relative">
                <span
                  aria-hidden
                  className="absolute -left-[19px] top-[5px] size-[5px] rounded-full bg-white/25"
                />
                <p className="text-xs leading-5 text-white/60">{event.summary}</p>
                <time className="mt-0.5 block font-mono text-2xs uppercase tracking-[0.1em] text-white/25">
                  {new Date(event.occurred_at).toLocaleTimeString()}
                </time>
              </li>
            ))
          )}
        </ol>
      </div>

      <Link
        href={loginHref}
        className="mt-5 inline-flex h-9 items-center justify-center gap-1.5 rounded-sm bg-white px-4 text-sm font-medium text-black transition hover:bg-white/90"
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
      });
    }

    return items.sort((a, b) => a.at - b.at);
  }, [initialUserText, followUps, events, tryout.created_at]);

  const blocks = useMemo(() => {
    const out: ChatBlock[] = [];
    for (const item of timeline) {
      const last = out[out.length - 1];
      if (item.kind === "user") {
        out.push({ kind: "user", item });
      } else if (last?.kind === "steps") {
        last.items.push(item);
      } else {
        out.push({ kind: "steps", id: item.id, items: [item] });
      }
    }
    return out;
  }, [timeline]);

  const lastEvent = events[events.length - 1];
  const judging =
    active && lastEvent?.type === "scoring" && /grading/i.test(lastEvent.summary);
  const thinkingLabel = !active
    ? null
    : judging
      ? "The judge is reading"
      : outputs.length > 0
        ? null
        : events.length === 0
          ? "Starting"
          : timeline[timeline.length - 1]?.kind === "agent"
            ? "Working"
            : null;

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
        <div className="mx-auto flex max-w-3xl flex-col gap-5">
          {blocks.map((block, index) =>
            block.kind === "user" ? (
              <UserBubble
                key={block.item.id}
                text={block.item.text}
                animate={index === blocks.length - 1}
              />
            ) : (
              <TraceBlock
                key={block.id}
                items={block.items}
                thinking={index === blocks.length - 1 ? thinkingLabel : null}
              />
            ),
          )}

          {thinkingLabel && blocks[blocks.length - 1]?.kind !== "steps" ? (
            <TraceBlock items={[]} thinking={thinkingLabel} />
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
              <VerdictCard scorecard={scorecard} />
            </div>
          ) : null}

          {scorecard?.judge && finished ? (
            <div className="animate-in fade-in slide-in-from-bottom-2 duration-500">
              <RerunStrip tryout={tryout} judge={scorecard.judge} loginHref={loginHref} />
            </div>
          ) : null}

          {finished ? (
            <div className="animate-in fade-in slide-in-from-bottom-3 duration-700">
              <EvalRoiCalculator
                tryout={tryout}
                loginHref={loginHref}
                initialVolume={evalPlan?.monthly_volume}
              />
            </div>
          ) : null}

          <div ref={bottomRef} />
        </div>
      </div>

      <div className="shrink-0 border-t border-white/[0.07] bg-[#131312]/95 px-4 py-3 backdrop-blur sm:px-8">
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
};

type ChatBlock =
  | { kind: "user"; item: ChatItem }
  | { kind: "steps"; id: string; items: ChatItem[] };

function UserBubble({ text, animate }: { text: string; animate?: boolean }) {
  return (
    <div
      className={cn(
        "flex justify-end",
        animate &&
          "animate-in fade-in slide-in-from-bottom-2 duration-300 motion-reduce:animate-none",
      )}
    >
      <div className="max-w-[85%] rounded-sm bg-[#e9e9e5] px-4 py-2.5 text-base leading-7 text-[#161614]">
        {text}
      </div>
    </div>
  );
}

function TraceBlock({
  items,
  thinking,
}: {
  items: ChatItem[];
  thinking?: string | null;
}) {
  return (
    <div className="ml-1 border-l border-white/10 py-0.5 pl-5">
      <div className="flex flex-col gap-2.5">
        {items.map((item) => (
          <div
            key={item.id}
            className="relative text-sm leading-6 text-white/55 animate-in fade-in duration-300 motion-reduce:animate-none"
          >
            <span
              aria-hidden
              className="absolute -left-[23px] top-[9.5px] size-[5px] rounded-full bg-white/30"
            />
            <span className="whitespace-pre-wrap">{item.text}</span>
          </div>
        ))}
        {thinking ? (
          <div className="relative flex h-6 items-center animate-in fade-in duration-300 motion-reduce:animate-none">
            <span
              aria-hidden
              className="absolute -left-[24.5px] flex size-2"
            >
              <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-white/25 motion-reduce:animate-none" />
              <span className="relative inline-flex size-2 rounded-full bg-white/45" />
            </span>
            <span className={cn(MICRO, "text-white/35")}>{thinking}</span>
          </div>
        ) : null}
      </div>
    </div>
  );
}

function ArtifactChatCard({ output }: { output: TryoutOutputPreview }) {
  const label = output.relative_path || output.key || "Artifact";
  const sizeLabel =
    typeof output.size_bytes === "number"
      ? `${Math.max(1, Math.round(output.size_bytes / 1024))} KB`
      : null;

  return (
    <div className="animate-in fade-in slide-in-from-bottom-2 duration-500">
      <div className="overflow-hidden rounded-sm border border-white/12">
        <div className="flex items-center justify-between gap-2 border-b border-white/8 px-4 py-2.5">
          <div className="flex min-w-0 items-center gap-2">
            <FileText className="size-4 shrink-0 text-white/40" />
            <div className="min-w-0">
              <p className="truncate text-sm font-medium text-white/90">{label}</p>
              {sizeLabel ? (
                <p className="text-2xs text-white/40">{artifactKindLabel(output)} · {sizeLabel}</p>
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
    <div className="rounded-sm border border-white/10 p-3">
      <div className="flex items-center justify-between gap-2">
        <div className="min-w-0">
          <p className="truncate text-xs font-medium text-white/80">
            {output.relative_path || output.key || "Output"}
          </p>
          <p className="text-2xs text-white/35">{artifactKindLabel(output)}</p>
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
        <pre className="mt-2 max-h-32 overflow-auto whitespace-pre-wrap text-2xs leading-5 text-white/55">
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
        "rounded-sm border border-white/12 bg-white/[0.015] p-2 transition focus-within:border-white/35",
        compact && "shadow-none",
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
        className="block w-full resize-none bg-transparent px-3 pt-2 text-base leading-7 text-white outline-none placeholder:text-white/30"
      />
      <div className="flex items-center justify-between gap-2 px-1 pb-0.5 pt-1">
        {footer ? <div className="flex flex-wrap items-center gap-2">{footer}</div> : <span />}
        <button
          type={onSubmit ? "button" : "submit"}
          onClick={onSubmit}
          disabled={!canSubmit || submitting}
          aria-label="Send"
          className="flex size-9 shrink-0 items-center justify-center rounded-sm bg-white text-black transition hover:bg-white/90 disabled:cursor-not-allowed disabled:opacity-40"
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
        className="inline-flex items-center gap-1.5 rounded-sm border border-white/12 py-1.5 pl-2.5 pr-2 font-mono text-2xs text-white/60 transition hover:border-white/30 hover:text-white/90 data-popup-open:border-white/40 data-popup-open:text-white disabled:opacity-50"
      >
        <span className="text-white/45">{icon}</span>
        <span className="max-w-[9rem] truncate">{selected?.label ?? "Select"}</span>
        <ChevronDown className="size-3.5 text-white/40 transition-transform duration-200 group-data-popup-open:rotate-180" />
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="start"
        className="min-w-[12rem] rounded-sm border-white/10 bg-[#181817] text-white"
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

function UnderlineField({
  label,
  value,
  onChange,
  placeholder,
  required,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  required?: boolean;
}) {
  return (
    <label className="block">
      <span className={cn(MICRO, "flex items-baseline justify-between tracking-[0.16em] text-white/40")}>
        {label}
        {required ? <span className="text-white/25">req</span> : null}
      </span>
      <input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        className="mt-2.5 w-full border-b border-white/15 bg-transparent pb-2 text-base text-white outline-none transition-colors placeholder:text-white/20 focus:border-white/50"
      />
    </label>
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
      <p className={cn(MICRO, "tracking-[0.16em] text-white/40")}>{label}</p>
      <div className="mt-2.5 flex flex-wrap gap-1.5">
        {options.map((option) => (
          <button
            key={option.value}
            type="button"
            onClick={() => onChange(option.value)}
            className={cn(
              "border px-2.5 py-1 font-mono text-2xs uppercase tracking-[0.08em] transition",
              option.value === value
                ? "border-white bg-white text-black"
                : "border-white/12 text-white/45 hover:border-white/35 hover:text-white/85",
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
    <label className="block">
      <span className={cn(MICRO, "flex items-baseline justify-between tracking-[0.16em] text-white/40")}>
        {fieldLabel(field)}
        {required ? null : <span className="text-white/25">opt</span>}
      </span>
      <input
        type={spec.type === "string" ? "text" : "number"}
        value={value}
        min={spec.minimum}
        max={spec.maximum}
        onChange={(event) => onChange(field, event.target.value)}
        className="mt-2.5 w-full border-b border-white/15 bg-transparent pb-2 text-base text-white outline-none transition-colors placeholder:text-white/20 focus:border-white/50"
      />
    </label>
  );
}

function Alert({ text }: { text: string }) {
  return (
    <div className="mt-5 border-l-2 border-white/40 pl-4 text-sm leading-6 text-white/70">
      {text}
    </div>
  );
}

// modelKeyFromPolicy maps a tryout's stored model policy back to the
// MODEL_OPTIONS key so reruns can rotate to a different agent.
function modelKeyFromPolicy(policy: unknown): string {
  if (!policy || typeof policy !== "object") return "auto";
  const models = (policy as { models?: unknown }).models;
  if (!Array.isArray(models) || models.length === 0) return "auto";
  const first = models[0] as { provider?: unknown; model?: unknown };
  const match = MODEL_OPTIONS.find(
    (option) => option.provider === first.provider && option.model === first.model,
  );
  return match ? modelOptionKey(match) : "auto";
}

// RerunStrip drives the comparison loop: one click reruns the same brief and
// the same bar with a different agent, a harsher judge, or a different judge.
function RerunStrip({
  tryout,
  judge,
  loginHref,
}: {
  tryout: AgentTryout;
  judge: TryoutJudgeSection;
  loginHref: string;
}) {
  const router = useRouter();

  function basePrefill(): RerunPrefill {
    const fieldValues: Record<string, string> = {};
    for (const [key, value] of Object.entries(tryout.input_snapshot ?? {})) {
      if (key === "eval_setup") continue;
      if (typeof value === "string") fieldValues[key] = value;
      else if (typeof value === "number") fieldValues[key] = String(value);
    }
    const setup = evalSetupFromInput(tryout.input_snapshot);
    return {
      templateSlug: tryout.template_slug,
      fieldValues,
      evalSetup: setup
        ? {
            unacceptableMistakes: setup.unacceptable_mistakes,
            reviewer: setup.human_reviewer,
            priority: setup.business_priority,
            style: setup.output_style,
            monthlyVolume: setup.monthly_volume,
          }
        : undefined,
      selectedModelKey: modelKeyFromPolicy(tryout.selected_model_policy),
      judgeModel: judge.model,
      judgeStrictness:
        judge.strictness === "lenient" || judge.strictness === "harsh"
          ? judge.strictness
          : "standard",
    };
  }

  function rerun(overrides: Partial<RerunPrefill>) {
    writeRerunPrefill({ ...basePrefill(), ...overrides });
    router.push("/tryouts");
  }

  const currentModelKey = modelKeyFromPolicy(tryout.selected_model_policy);
  const currentOption = MODEL_OPTIONS.find(
    (option) => modelOptionKey(option) === currentModelKey,
  );
  const nextAgent =
    MODEL_OPTIONS.find(
      (option) =>
        option.model &&
        option.provider !== currentOption?.provider &&
        modelOptionKey(option) !== currentModelKey,
    ) ?? MODEL_OPTIONS.find((option) => option.model && modelOptionKey(option) !== currentModelKey);
  const nextJudge =
    JUDGE_OPTIONS.find((option) => option.value !== judge.model) ?? JUDGE_OPTIONS[0];

  return (
    <div className="border border-white/12 p-5 sm:p-6">
      <p className={cn(MICRO, "text-white/40")}>Do not take one run&apos;s word for it</p>
      <p className="mt-2 max-w-lg text-sm leading-6 text-white/50">
        This is the whole point of an eval: the bar stays fixed, everything else
        can change, and the verdicts stay comparable.
      </p>
      <div className="mt-4 flex flex-wrap gap-2">
        {nextAgent ? (
          <RerunButton
            onClick={() =>
              rerun({ selectedModelKey: modelOptionKey(nextAgent) })
            }
          >
            Same brief, run {nextAgent.label} instead
          </RerunButton>
        ) : null}
        {judge.strictness !== "harsh" ? (
          <RerunButton onClick={() => rerun({ judgeStrictness: "harsh" })}>
            Same brief, harsher judge
          </RerunButton>
        ) : null}
        <RerunButton onClick={() => rerun({ judgeModel: nextJudge.value })}>
          Let {nextJudge.label} judge instead
        </RerunButton>
      </div>
      <p className="mt-4 border-t border-white/[0.07] pt-3 text-xs leading-5 text-white/35">
        The bar you wrote is reusable.{" "}
        <Link href={loginHref} className="text-white/60 underline-offset-4 hover:text-white hover:underline">
          Save this eval
        </Link>{" "}
        and it grades every future run, automatically.
      </p>
    </div>
  );
}

function RerunButton({
  onClick,
  children,
}: {
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="border border-white/15 px-3 py-1.5 text-left font-mono text-2xs text-white/65 transition hover:border-white/40 hover:text-white"
    >
      {children}
    </button>
  );
}

const VERDICT_WORDS: Record<TryoutJudgeSection["verdict"], string> = {
  approved: "Approved",
  needs_edits: "Needs edits",
  rejected: "Rejected",
  not_judged: "Not judged",
};

// VerdictCard is the centerpiece of the funnel: the judge the visitor chose,
// grading the work against the visitor's own words, with its reasoning quoted.
function VerdictCard({
  scorecard,
  compact,
}: {
  scorecard: TryoutScorecard;
  compact?: boolean;
}) {
  const judge = scorecard.judge;
  if (!judge) return <ScorecardCard scorecard={scorecard} compact={compact} />;
  const grade = judge.score != null ? 1 + judge.score * 4 : null;

  return (
    <div
      className={cn(
        "border",
        judge.verdict === "approved" ? "border-white/30" : "border-white/12",
        compact ? "p-4" : "p-5 sm:p-6",
      )}
    >
      <div className="flex items-baseline justify-between gap-3">
        <p className={cn(MICRO, "text-white/40")}>The verdict</p>
        <p className="font-mono text-2xs uppercase tracking-[0.14em] text-white/30">
          {judgeModelLabel(judge.model)}
          {judge.strictness ? ` · ${judge.strictness}` : ""}
        </p>
      </div>

      <div className="mt-4 flex items-end justify-between gap-4">
        <p
          className={cn(
            SERIF,
            "font-light leading-none",
            compact ? "text-3xl" : "text-4xl sm:text-5xl",
            judge.verdict === "rejected" ? "text-white/55" : "text-white/95",
          )}
        >
          {VERDICT_WORDS[judge.verdict]}
        </p>
        {grade != null ? (
          <p className={cn(SERIF, "font-light leading-none text-white/85", compact ? "text-2xl" : "text-3xl sm:text-4xl")}>
            {grade.toFixed(1)}
            <span className="text-lg text-white/35"> / 5</span>
          </p>
        ) : null}
      </div>
      <p className="mt-3 text-xs leading-5 text-white/40">
        Graded by the judge you chose, against the bar you wrote in step 01.
      </p>
      {judge.reason ? (
        <p className="mt-2 text-sm leading-6 text-white/55">{judge.reason}</p>
      ) : null}

      {!compact && judge.criteria.length > 0 ? (
        <ul className="mt-5 divide-y divide-white/[0.07] border-t border-white/[0.07]">
          {judge.criteria.map((criterion) => (
            <li key={criterion.key} className="py-3">
              <div className="flex items-baseline justify-between gap-3">
                <span className="text-sm leading-6 text-white/75">{criterion.label}</span>
                <span
                  className={cn(
                    "font-mono text-2xs uppercase tracking-[0.14em]",
                    criterion.status === "passed"
                      ? "text-white/80"
                      : criterion.status === "failed"
                        ? "text-white/35 line-through"
                        : "text-white/25",
                  )}
                >
                  {criterion.status === "skipped" ? "not graded" : criterion.status}
                </span>
              </div>
              {criterion.reasoning ? (
                <p className="mt-1 max-w-xl text-xs leading-5 text-white/40">
                  “{criterion.reasoning}”
                </p>
              ) : null}
            </li>
          ))}
        </ul>
      ) : null}

      {!compact && scorecard.checks.length > 0 ? (
        <div className="mt-5">
          <p className={cn(MICRO, "text-white/30")}>Automatic checks</p>
          <ul className="mt-2 divide-y divide-white/[0.06]">
            {scorecard.checks.map((check) => (
              <li
                key={check.key}
                className="flex items-baseline justify-between gap-3 py-1.5 text-xs"
              >
                <span className="text-white/50">{fieldLabel(check.key)}</span>
                <span
                  className={cn(
                    "font-mono text-2xs uppercase tracking-[0.14em]",
                    check.status === "passed" ? "text-white/60" : "text-white/25",
                  )}
                >
                  {check.status}
                </span>
              </li>
            ))}
          </ul>
        </div>
      ) : null}
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
    <div className={cn("border border-white/12", compact ? "p-4" : "p-5 sm:p-6")}>
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className={cn(MICRO, "text-white/40")}>Eval scorecard</p>
          <p className="mt-2 text-xs text-white/35">
            {scorecard.passed_validators} of {scorecard.total_validators} checks
            passed
          </p>
        </div>
        <p
          className={cn(
            SERIF,
            "font-light leading-none text-white/90",
            compact ? "text-4xl" : "text-5xl sm:text-6xl",
          )}
        >
          {pct}
          <span className={cn(compact ? "text-xl" : "text-2xl", "text-white/35")}>%</span>
        </p>
      </div>
      <div className="mt-4 h-px w-full bg-white/10">
        <div
          className="h-full bg-white/70 transition-all duration-700"
          style={{ width: `${pct}%` }}
        />
      </div>
      {!compact && scorecard.checks.length > 0 ? (
        <ul className="mt-4 divide-y divide-white/[0.07]">
          {scorecard.checks.map((check) => (
            <li
              key={check.key}
              className="flex items-baseline justify-between gap-3 py-2.5 text-sm"
            >
              <span className="text-white/65">{fieldLabel(check.key)}</span>
              <span
                className={cn(
                  "font-mono text-2xs uppercase tracking-[0.14em]",
                  check.status === "passed"
                    ? "text-white/80"
                    : check.status === "failed"
                      ? "text-white/30 line-through"
                      : "text-white/25",
                )}
              >
                {check.status}
              </span>
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
    <div className={cn("border border-white/12", compact ? "p-4" : "p-5 sm:p-6")}>
      <div className="flex items-baseline justify-between gap-3">
        <p className={cn(MICRO, "text-white/40")}>Your rubric</p>
        <span className="font-mono text-2xs uppercase tracking-[0.14em] text-white/30">
          {priorityLabel(setup.business_priority)}
        </span>
      </div>
      <p className="mt-3 text-sm leading-6 text-white/55">
        Graded as if {setup.human_reviewer} signs off. Behavior:{" "}
        {styleLabel(setup.output_style).toLowerCase()}.
      </p>
      {setup.unacceptable_mistakes ? (
        <p className="mt-1 text-sm leading-6 text-white/55">
          Instant fail: {setup.unacceptable_mistakes}
        </p>
      ) : null}
      {!compact ? (
        <ul className="mt-4 divide-y divide-white/[0.07] border-t border-white/[0.07]">
          {setup.derived_rubric.slice(0, 4).map((item, index) => (
            <li key={item.key} className="flex gap-4 py-3">
              <span className="font-mono text-2xs leading-6 text-white/25">
                0{index + 1}
              </span>
              <div>
                <p className="text-sm leading-6 text-white/80">{item.label}</p>
                <p className="mt-0.5 text-xs leading-5 text-white/40">
                  {item.checks.join(" · ")}
                </p>
              </div>
            </li>
          ))}
        </ul>
      ) : null}
      <p className="mt-3 text-xs leading-5 text-white/35">
        Suggested setting: {setup.suggested_generation_settings.temperature}{" "}
        temperature.
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
      className="h-7 rounded-sm border-white/15 bg-transparent px-2.5 font-mono text-2xs uppercase tracking-[0.1em] text-white/70 hover:bg-white/10 hover:text-white"
    >
      <Download className="size-3" />
      {label}
    </Button>
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
      return summary.startsWith("Judge") ? summary : "Updated the eval scorecard";
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

type TryoutJudgeCriterion = {
  key: string;
  label: string;
  mode: string;
  status: "passed" | "failed" | "skipped";
  score?: number;
  reasoning?: string;
  confidence?: string;
  reason?: string;
};

type TryoutJudgeSection = {
  model: string;
  strictness?: string;
  verdict: "approved" | "needs_edits" | "rejected" | "not_judged";
  score?: number;
  reason?: string;
  criteria: TryoutJudgeCriterion[];
};

type TryoutScorecard = {
  passed_validators: number;
  total_validators: number;
  score: number;
  passed: boolean;
  dimensions: string[];
  checks: TryoutScorecardCheck[];
  judge: TryoutJudgeSection | null;
};

function tryoutJudgeSection(raw: unknown): TryoutJudgeSection | null {
  if (!raw || typeof raw !== "object") return null;
  const section = raw as Record<string, unknown>;
  if (typeof section.model !== "string" || section.model.trim() === "") return null;
  const verdict =
    section.verdict === "approved" ||
    section.verdict === "needs_edits" ||
    section.verdict === "rejected" ||
    section.verdict === "not_judged"
      ? section.verdict
      : "not_judged";
  const criteria: TryoutJudgeCriterion[] = Array.isArray(section.criteria)
    ? section.criteria
        .map((item): TryoutJudgeCriterion | null => {
          if (!item || typeof item !== "object") return null;
          const criterion = item as Record<string, unknown>;
          const status =
            criterion.status === "passed" || criterion.status === "failed"
              ? criterion.status
              : "skipped";
          const key = typeof criterion.key === "string" ? criterion.key : "criterion";
          return {
            key,
            label:
              typeof criterion.label === "string" && criterion.label.trim()
                ? criterion.label
                : fieldLabel(key),
            mode: typeof criterion.mode === "string" ? criterion.mode : "",
            status,
            score: typeof criterion.score === "number" ? criterion.score : undefined,
            reasoning:
              typeof criterion.reasoning === "string" ? criterion.reasoning : undefined,
            confidence:
              typeof criterion.confidence === "string" ? criterion.confidence : undefined,
            reason: typeof criterion.reason === "string" ? criterion.reason : undefined,
          };
        })
        .filter((item): item is TryoutJudgeCriterion => item !== null)
    : [];
  return {
    model: section.model,
    strictness:
      typeof section.strictness === "string" ? section.strictness : undefined,
    verdict,
    score: typeof section.score === "number" ? section.score : undefined,
    reason: typeof section.reason === "string" ? section.reason : undefined,
    criteria,
  };
}

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
    judge: tryoutJudgeSection(card.judge),
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

// parseMonthlyVolume turns freeform answers like "500", "10k", or "2,000 runs"
// into a number for the ROI inputs.
function parseMonthlyVolume(raw: string | undefined): number | null {
  if (!raw) return null;
  const match = raw.replaceAll(",", "").match(/(\d+(?:\.\d+)?)\s*(k|m)?/i);
  if (!match) return null;
  let value = Number(match[1]);
  if (!Number.isFinite(value) || value <= 0) return null;
  const unit = (match[2] ?? "").toLowerCase();
  if (unit === "k") value *= 1_000;
  if (unit === "m") value *= 1_000_000;
  return Math.round(value);
}

function EvalRoiCalculator({
  tryout,
  loginHref,
  initialVolume,
}: {
  tryout: AgentTryout;
  loginHref: string;
  initialVolume?: string;
}) {
  const anchor = ROI_ANCHORS[tryout.template_slug] ?? DEFAULT_ANCHOR;
  const [company, setCompany] = useState("");
  const [email, setEmail] = useState("");
  const [volume, setVolume] = useState(() => {
    const parsed = parseMonthlyVolume(initialVolume);
    return parsed != null ? String(parsed) : "5000";
  });
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
    <div className="border border-white/12 p-5 sm:p-6">
      <p className={cn(MICRO, "text-white/40")}>The business case</p>
      <p className="mt-3 max-w-lg text-sm leading-6 text-white/50">
        You just watched an agent finish one {anchor.label}. This is what grading
        it before production is worth at your scale.
      </p>

      <div className="mt-6 grid gap-x-8 gap-y-6 sm:grid-cols-2 lg:grid-cols-4">
        <RoiInput label="Company" value={company} onChange={setCompany} placeholder="Acme Inc" />
        <RoiInput label="Work email" value={email} onChange={setEmail} placeholder="you@acme.com" />
        <RoiInput label={`${anchor.label}s / month`} value={volume} onChange={setVolume} numeric />
        <RoiInput label="Human $ / task" value={humanCost} onChange={setHumanCost} numeric />
      </div>

      <div className="mt-7 grid divide-y divide-white/[0.08] border-y border-white/[0.08] sm:grid-cols-2 sm:divide-x sm:divide-y-0">
        <div className="py-5 sm:pr-8">
          <p className={cn(MICRO, "text-white/35")}>Ship blind</p>
          <p className={cn(SERIF, "mt-3 text-4xl font-light leading-none text-white/90 sm:text-5xl")}>
            {usd(costWithoutEvals)}
          </p>
          <p className="mt-2 text-xs text-white/35">per year at risk</p>
        </div>
        <div className="py-5 sm:pl-8">
          <p className={cn(MICRO, "text-white/55")}>Ship graded</p>
          <p className={cn(SERIF, "mt-3 text-4xl font-light leading-none text-white sm:text-5xl")}>
            {usd(capturedSavings)}
          </p>
          <p className="mt-2 text-xs text-white/35">per year captured</p>
        </div>
      </div>

      <div className="mt-6 flex flex-col items-start justify-between gap-4 sm:flex-row sm:items-center">
        <p className="text-sm leading-6 text-white/60">
          Evaluating first is worth{" "}
          <span className="font-medium text-white">{usd(netUpside)}/yr</span> to{" "}
          {company.trim() || "your team"} on this workflow alone.
        </p>
        <Link
          href={`/enterprise?from=tryout&task=${encodeURIComponent(tryout.template_slug)}${email.trim() ? `&email=${encodeURIComponent(email.trim())}` : ""}`}
          className="inline-flex h-9 shrink-0 items-center gap-1.5 rounded-sm bg-white px-4 text-sm font-medium text-black transition hover:bg-white/90"
        >
          Talk to us about integrating
          <ArrowRight className="size-4" />
        </Link>
      </div>
      <p className="mt-3 text-xs text-white/35">
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
      <span className={cn(MICRO, "block tracking-[0.16em] text-white/40")}>{label}</span>
      <input
        type={numeric ? "number" : "text"}
        inputMode={numeric ? "decimal" : undefined}
        min={numeric ? 0 : undefined}
        value={value}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
        className="mt-2 w-full border-b border-white/15 bg-transparent pb-1.5 text-sm text-white outline-none transition-colors placeholder:text-white/20 focus:border-white/50"
      />
    </label>
  );
}
