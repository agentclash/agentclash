"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import type { FormEvent } from "react";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Activity,
  ArrowRight,
  CheckCircle2,
  Download,
  FileText,
  Loader2,
  Lock,
  Play,
  Sparkles,
} from "lucide-react";

import {
  createAnonymousAgentTryout,
  getPublicAgentTryout,
  getPublicAgentTryoutEvents,
  listAgentTryoutTemplates,
} from "@/lib/api/agent-tryouts";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type {
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

const api = createApiClient();

export function PublicTryoutsClient() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const urlTryoutId = searchParams.get("tryout") ?? "";

  const [templates, setTemplates] = useState<AgentTryoutTemplate[]>([]);
  const [templateSlug, setTemplateSlug] = useState("");
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

  return (
    <main className="min-h-screen overflow-hidden bg-[#14120f] text-[#f4efe6]">
      <div className="pointer-events-none fixed inset-0 bg-[radial-gradient(circle_at_50%_0%,rgba(216,161,93,0.20),transparent_34%),linear-gradient(180deg,rgba(244,239,230,0.06),transparent_30%)]" />
      <div className="relative mx-auto flex min-h-screen w-full max-w-6xl flex-col px-4 py-6 sm:px-6 lg:px-8">
        <header className="flex items-center justify-between gap-4">
          <Link
            href="/"
            className="font-sans text-sm font-semibold tracking-tight text-[#f4efe6]"
          >
            AgentClash
          </Link>
          <div className="flex items-center gap-3">
            <Link
              href="/pricing"
              className="hidden text-sm text-[#f4efe6]/60 transition hover:text-[#f4efe6] sm:inline"
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

        <section className="mx-auto flex w-full max-w-4xl flex-1 flex-col items-center justify-center py-12 text-center sm:py-16">
          <div className="mb-4 inline-flex items-center gap-2 rounded-full border border-[#d8a15d]/25 bg-[#d8a15d]/10 px-3 py-1 text-xs font-medium uppercase tracking-[0.14em] text-[#e7c18d]">
            <Sparkles className="size-3.5" />
            Public agent tryouts
          </div>
          <h1 className="font-sans text-[clamp(2.6rem,7vw,5.8rem)] font-semibold leading-[0.92] tracking-tight">
            Hand an agent real office work.
          </h1>
          <p className="mt-5 max-w-2xl text-base leading-7 text-[#f4efe6]/62 sm:text-lg">
            Paste meeting notes, spreadsheet requirements, inbox context, or a
            status update. AgentClash runs the task in a hosted sandbox, shows
            the trace, and lets teams save the result when they need reruns.
          </p>

          <div className="mt-8 grid w-full gap-4 rounded-[2rem] border border-[#f4efe6]/12 bg-[#1b1814]/80 p-3 text-left shadow-2xl shadow-black/25 backdrop-blur sm:p-4 lg:grid-cols-[0.9fr_1.1fr]">
            <TaskPicker
              templates={templates}
              selectedSlug={templateSlug}
              loading={templatesLoading}
              onSelect={setTemplateSlug}
            />

            <form
              onSubmit={handleLaunch}
              className="rounded-[1.45rem] border border-[#f4efe6]/10 bg-[#f4efe6]/[0.035] p-4 sm:p-5"
            >
              <div className="flex items-center justify-between gap-3">
                <div>
                  <p className="text-xs font-medium uppercase tracking-[0.14em] text-[#f4efe6]/45">
                    Task brief
                  </p>
                  <h2 className="mt-1 text-lg font-semibold tracking-tight">
                    {template?.name ?? "Choose a task"}
                  </h2>
                </div>
                {template ? (
                  <Badge variant="outline" className="border-[#d8a15d]/30 text-[#e7c18d]">
                    {Math.round(template.max_duration_seconds / 60)} min
                  </Badge>
                ) : null}
              </div>

              {template ? (
                <p className="mt-2 text-sm leading-6 text-[#f4efe6]/55">
                  {template.description} Hosted cost is capped at{" "}
                  {`$${template.max_cost_usd.toFixed(2)}`} for this task.
                </p>
              ) : null}

              <div className="mt-5 space-y-4">
                {primaryField ? (
                  <LabeledField
                    field={primaryField[0]}
                    spec={primaryField[1]}
                    value={fieldValues[primaryField[0]] ?? ""}
                    required={required.has(primaryField[0])}
                    large
                    label="Tell the agent what to do"
                    onChange={updateField}
                  />
                ) : null}

                {secondaryFields.length > 0 ? (
                  <div className="grid gap-3 sm:grid-cols-2">
                    {secondaryFields.map(([field, spec]) => (
                      <LabeledField
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
              </div>

              {message ? (
                <div className="mt-4 rounded-2xl border border-red-400/25 bg-red-400/10 p-3 text-sm text-red-100">
                  {message}
                </div>
              ) : null}
              {quotaMessage ? (
                <div className="mt-4 rounded-2xl border border-[#d8a15d]/25 bg-[#d8a15d]/10 p-3 text-sm text-[#f2d6ad]">
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

              <div className="mt-5 flex flex-col gap-3 sm:flex-row sm:items-center">
                <Button
                  type="submit"
                  size="lg"
                  disabled={!template || templatesLoading || launching}
                  className="h-11 rounded-full bg-[#e7c18d] px-5 text-[#14120f] hover:bg-[#f0cf9d]"
                >
                  {launching ? (
                    <Loader2 className="size-4 animate-spin" />
                  ) : (
                    <Play className="size-4" />
                  )}
                  Run public tryout
                </Button>
                <p className="text-xs leading-5 text-[#f4efe6]/45">
                  No payment gate. Public runs are limited by task count,
                  fingerprint, and hosted spend caps.
                </p>
              </div>
            </form>
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

function TaskPicker({
  templates,
  selectedSlug,
  loading,
  onSelect,
}: {
  templates: AgentTryoutTemplate[];
  selectedSlug: string;
  loading: boolean;
  onSelect: (slug: string) => void;
}) {
  return (
    <aside className="rounded-[1.45rem] border border-[#f4efe6]/10 bg-[#110f0d]/65 p-4 sm:p-5">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-xs font-medium uppercase tracking-[0.14em] text-[#f4efe6]/45">
            Choose work
          </p>
          <h2 className="mt-1 text-lg font-semibold tracking-tight">
            Office tasks
          </h2>
        </div>
        <Badge variant="outline" className="border-[#f4efe6]/15 text-[#f4efe6]/60">
          4 free tasks
        </Badge>
      </div>

      <div className="mt-5 space-y-2">
        {loading ? (
          <div className="flex items-center gap-2 rounded-2xl border border-[#f4efe6]/10 p-3 text-sm text-[#f4efe6]/55">
            <Loader2 className="size-4 animate-spin" />
            Loading task templates
          </div>
        ) : null}
        {!loading && templates.length === 0 ? (
          <div className="rounded-2xl border border-[#f4efe6]/10 p-3 text-sm text-[#f4efe6]/55">
            Public tasks are not configured yet.
          </div>
        ) : null}
        {templates.map((template) => {
          const selected = template.slug === selectedSlug;
          return (
            <button
              key={template.slug}
              type="button"
              onClick={() => onSelect(template.slug)}
              className={[
                "w-full rounded-2xl border p-3 text-left transition",
                selected
                  ? "border-[#d8a15d]/45 bg-[#d8a15d]/12"
                  : "border-[#f4efe6]/10 bg-[#f4efe6]/[0.025] hover:border-[#f4efe6]/22",
              ].join(" ")}
            >
              <div className="flex items-center justify-between gap-3">
                <span className="font-medium tracking-tight">{template.name}</span>
                <span className="text-xs text-[#f4efe6]/42">
                  {`$${template.max_cost_usd.toFixed(2)} cap`}
                </span>
              </div>
              <p className="mt-1 line-clamp-2 text-sm leading-5 text-[#f4efe6]/50">
                {template.description}
              </p>
            </button>
          );
        })}
      </div>
    </aside>
  );
}

function LabeledField({
  field,
  spec,
  value,
  required,
  large = false,
  label,
  onChange,
}: {
  field: string;
  spec: FieldSpec;
  value: string;
  required: boolean;
  large?: boolean;
  label?: string;
  onChange: (field: string, value: string) => void;
}) {
  const displayLabel = label ?? fieldLabel(field);
  return (
    <label className="block">
      <span className="mb-1.5 flex items-center justify-between text-sm font-medium">
        <span>{displayLabel}</span>
        {required ? null : (
          <span className="text-xs font-normal text-[#f4efe6]/35">Optional</span>
        )}
      </span>
      {spec.type === "string" ? (
        <textarea
          value={value}
          onChange={(event) => onChange(field, event.target.value)}
          rows={large ? 8 : 3}
          className="block w-full resize-y rounded-2xl border border-[#f4efe6]/12 bg-[#0e0c0a]/75 px-4 py-3 text-sm leading-6 text-[#f4efe6] outline-none placeholder:text-[#f4efe6]/25 focus:border-[#d8a15d]/55 focus:ring-2 focus:ring-[#d8a15d]/20"
          placeholder={large ? "Paste the work, context, or instructions here." : ""}
        />
      ) : (
        <input
          type="number"
          value={value}
          min={spec.minimum}
          max={spec.maximum}
          onChange={(event) => onChange(field, event.target.value)}
          className="block h-11 w-full rounded-2xl border border-[#f4efe6]/12 bg-[#0e0c0a]/75 px-4 text-sm text-[#f4efe6] outline-none focus:border-[#d8a15d]/55 focus:ring-2 focus:ring-[#d8a15d]/20"
        />
      )}
    </label>
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
      <section className="mx-auto mb-12 grid w-full max-w-4xl gap-3 rounded-[1.6rem] border border-[#f4efe6]/10 bg-[#f4efe6]/[0.03] p-4 text-sm text-[#f4efe6]/55 sm:grid-cols-3">
        <ProofItem icon={Lock} label="Task gated" text="Four hosted tasks per fingerprint by default." />
        <ProofItem icon={Activity} label="Trace first" text="Every run exposes a redacted event trail." />
        <ProofItem icon={FileText} label="Exportable" text="Download the public trace as JSON." />
      </section>
    );
  }

  if (!tryout) {
    return (
      <section className="mx-auto mb-12 flex w-full max-w-4xl items-center justify-center rounded-[1.6rem] border border-[#f4efe6]/10 bg-[#f4efe6]/[0.03] p-8 text-sm text-[#f4efe6]/55">
        <Loader2 className="mr-2 size-4 animate-spin" />
        Loading tryout
      </section>
    );
  }

  const summary =
    typeof tryout.summary?.message === "string" ? tryout.summary.message : "";
  const outputs = tryoutOutputs(tryout.summary);

  return (
    <section className="mx-auto mb-12 w-full max-w-4xl rounded-[1.6rem] border border-[#f4efe6]/10 bg-[#f4efe6]/[0.035] p-4 shadow-xl shadow-black/20 sm:p-5">
      <div className="flex flex-col gap-4 border-b border-[#f4efe6]/10 pb-4 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="text-xl font-semibold tracking-tight">Live trace</h2>
            <Badge variant={tryoutStatusVariant(tryout.status)}>{tryout.status}</Badge>
          </div>
          <p className="mt-1 text-sm leading-6 text-[#f4efe6]/55">
            {summary || "The agent trace will appear here as the sandbox runs."}
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button
            type="button"
            variant="outline"
            onClick={() => downloadTrace(tryout, events)}
            className="rounded-full border-[#f4efe6]/15 bg-transparent text-[#f4efe6] hover:bg-[#f4efe6]/10"
          >
            <Download className="size-4" />
            Export trace
          </Button>
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
        <Stat label="Cost" value={formatTryoutCost(tryout)} />
        <Stat label="Latency" value={formatTryoutLatency(tryout.latency_ms)} />
        <Stat label="Expires" value={formatExpiry(tryout.expires_at)} />
      </dl>

      <div className="mt-5 grid gap-4 lg:grid-cols-[1.1fr_0.9fr]">
        <div>
          {outputs.length > 0 ? (
            <div className="mb-5">
              <h3 className="mb-2 text-sm font-semibold tracking-tight">Result</h3>
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
                      {output.relative_path ? (
                        <span className="text-xs text-[#f4efe6]/38">
                          {output.relative_path}
                        </span>
                      ) : null}
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
          <h3 className="mb-2 text-sm font-semibold tracking-tight">Timeline</h3>
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
    </section>
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

function formatExpiry(expiresAt: string | undefined): string {
  if (!expiresAt) return "Saved";
  return new Date(expiresAt).toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
  });
}

function downloadTrace(tryout: AgentTryout, events: TryoutTimelineEvent[]) {
  const payload = {
    source: "agentclash_public_tryout",
    exported_at: new Date().toISOString(),
    tryout,
    events,
  };
  const blob = new Blob([JSON.stringify(payload, null, 2)], {
    type: "application/json",
  });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = `agentclash-tryout-${tryout.id}.json`;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
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
