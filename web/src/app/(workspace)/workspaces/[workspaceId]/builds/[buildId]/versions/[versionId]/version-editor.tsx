"use client";

import { useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type {
  AgentBuildVersion,
  AgentKind,
  ValidationResult,
} from "@/lib/api/types";
import { AGENT_KINDS } from "@/lib/api/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { JsonField } from "@/components/ui/json-field";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { toast } from "sonner";
import {
  CheckCircle,
  Loader2,
  Save,
  ShieldCheck,
  Sparkles,
} from "lucide-react";

import {
  type EditableSpecs,
  type GuidedAuthoringState,
  guidedStateFromVersion,
  guidedTemplates,
  specsFromGuidedState,
  versionPayloadFromTemplate,
} from "./guided-authoring";

interface VersionEditorProps {
  version: AgentBuildVersion;
}

const specFields = [
  {
    key: "policy_spec",
    label: "Policy Spec",
    description: 'Must contain an "instructions" field.',
    required: true as const,
  },
  {
    key: "interface_spec",
    label: "Interface Spec",
    description: "How the agent expects the primary task input to be shaped.",
    required: true as const,
  },
  {
    key: "model_spec",
    label: "Model Spec",
    description: "Optional model-time hints that stay with the build version.",
    required: false as const,
  },
  {
    key: "reasoning_spec",
    label: "Reasoning Spec",
    description: "Optional reasoning preferences for advanced runs.",
    required: false as const,
  },
  {
    key: "memory_spec",
    label: "Memory Spec",
    description: "How much context the agent should try to carry forward.",
    required: false as const,
  },
  {
    key: "workflow_spec",
    label: "Workflow Spec",
    description: "Execution preferences such as tool strategy or orchestration hints.",
    required: false as const,
  },
  {
    key: "guardrail_spec",
    label: "Guardrail Spec",
    description: "Optional safety and escalation rules.",
    required: false as const,
  },
  {
    key: "output_schema",
    label: "Output Schema",
    description: "The final answer contract shown to the agent at run time.",
    required: false as const,
  },
  {
    key: "trace_contract",
    label: "Trace Contract",
    description: "Optional run telemetry contract.",
    required: false as const,
  },
  {
    key: "publication_spec",
    label: "Publication Spec",
    description: "Optional publishing or surfacing metadata.",
    required: false as const,
  },
] as const;

type SpecKey = (typeof specFields)[number]["key"];

function jsonStr(val: unknown): string {
  if (val === null || val === undefined) return "";
  if (typeof val === "string") return val;
  return JSON.stringify(val, null, 2);
}

function safeParseSpec(raw: string): unknown {
  const trimmed = raw.trim();
  if (!trimmed) return {};
  try {
    return JSON.parse(trimmed);
  } catch {
    return {};
  }
}

function versionToEditableSpecs(version: AgentBuildVersion): EditableSpecs {
  return {
    agent_kind: version.agent_kind,
    interface_spec: version.interface_spec,
    policy_spec: version.policy_spec,
    reasoning_spec: version.reasoning_spec,
    memory_spec: version.memory_spec,
    workflow_spec: version.workflow_spec,
    guardrail_spec: version.guardrail_spec,
    model_spec: version.model_spec,
    output_schema: version.output_schema,
    trace_contract: version.trace_contract,
    publication_spec: version.publication_spec,
  };
}

function specsFromStrings(
  agentKind: AgentKind,
  specs: Record<SpecKey, string>,
): EditableSpecs {
  return {
    agent_kind: agentKind,
    interface_spec: safeParseSpec(specs.interface_spec),
    policy_spec: safeParseSpec(specs.policy_spec),
    reasoning_spec: safeParseSpec(specs.reasoning_spec),
    memory_spec: safeParseSpec(specs.memory_spec),
    workflow_spec: safeParseSpec(specs.workflow_spec),
    guardrail_spec: safeParseSpec(specs.guardrail_spec),
    model_spec: safeParseSpec(specs.model_spec),
    output_schema: safeParseSpec(specs.output_schema),
    trace_contract: safeParseSpec(specs.trace_contract),
    publication_spec: safeParseSpec(specs.publication_spec),
  };
}

function guidedFieldTextAreaClassName(error?: string): string {
  return [
    "block w-full rounded-lg border bg-transparent px-3 py-2 text-sm leading-relaxed placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring/50 resize-y disabled:opacity-50 disabled:cursor-not-allowed",
    error
      ? "border-destructive focus:border-destructive focus:ring-destructive/50"
      : "border-input focus:border-ring",
  ].join(" ");
}

interface GuidedTextAreaProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  description: string;
  disabled?: boolean;
  rows?: number;
  error?: string;
}

function GuidedTextArea({
  label,
  value,
  onChange,
  description,
  disabled,
  rows = 4,
  error,
}: GuidedTextAreaProps) {
  return (
    <div className="space-y-1.5">
      <label className="block text-sm font-medium">{label}</label>
      <p className="text-xs text-muted-foreground">{description}</p>
      <textarea
        value={value}
        onChange={(event) => onChange(event.target.value)}
        disabled={disabled}
        rows={rows}
        className={guidedFieldTextAreaClassName(error)}
      />
      {error ? <p className="text-xs text-destructive">{error}</p> : null}
    </div>
  );
}

interface GuidedSelectProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  description: string;
  disabled?: boolean;
  options: Array<{ value: string; label: string }>;
}

function GuidedSelect({
  label,
  value,
  onChange,
  description,
  disabled,
  options,
}: GuidedSelectProps) {
  return (
    <div className="space-y-1.5">
      <label className="block text-sm font-medium">{label}</label>
      <p className="text-xs text-muted-foreground">{description}</p>
      <select
        value={value}
        onChange={(event) => onChange(event.target.value)}
        disabled={disabled}
        className="block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50"
      >
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    </div>
  );
}

export function VersionEditor({ version }: VersionEditorProps) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();

  const isLocked = version.version_status === "ready";
  const originalSpecs = useMemo(() => versionToEditableSpecs(version), [version]);

  const [agentKind, setAgentKind] = useState<AgentKind>(
    AGENT_KINDS.includes(version.agent_kind as AgentKind)
      ? (version.agent_kind as AgentKind)
      : "llm_agent",
  );
  const [specs, setSpecs] = useState<Record<SpecKey, string>>(() => {
    const initial: Record<string, string> = {};
    for (const field of specFields) {
      initial[field.key] = jsonStr(version[field.key as keyof AgentBuildVersion]);
    }
    return initial as Record<SpecKey, string>;
  });
  const [guided, setGuided] = useState<GuidedAuthoringState>(() =>
    guidedStateFromVersion(originalSpecs),
  );
  const [activeTab, setActiveTab] = useState<"guided" | "json">("guided");

  const [saving, setSaving] = useState(false);
  const [validating, setValidating] = useState(false);
  const [markingReady, setMarkingReady] = useState(false);
  const [validationErrors, setValidationErrors] = useState<Record<string, string>>(
    {},
  );

  function syncSpecsFromEditable(next: EditableSpecs) {
    setAgentKind(
      AGENT_KINDS.includes(next.agent_kind as AgentKind)
        ? (next.agent_kind as AgentKind)
        : "llm_agent",
    );
    setSpecs({
      policy_spec: jsonStr(next.policy_spec),
      interface_spec: jsonStr(next.interface_spec),
      model_spec: jsonStr(next.model_spec),
      reasoning_spec: jsonStr(next.reasoning_spec),
      memory_spec: jsonStr(next.memory_spec),
      workflow_spec: jsonStr(next.workflow_spec),
      guardrail_spec: jsonStr(next.guardrail_spec),
      output_schema: jsonStr(next.output_schema),
      trace_contract: jsonStr(next.trace_contract),
      publication_spec: jsonStr(next.publication_spec),
    });
    setGuided(guidedStateFromVersion(next));
  }

  function clearValidationError(field: string) {
    if (!validationErrors[field]) return;
    setValidationErrors((prev) => {
      const next = { ...prev };
      delete next[field];
      return next;
    });
  }

  function updateSpec(key: SpecKey, value: string) {
    setSpecs((prev) => ({ ...prev, [key]: value }));
    clearValidationError(key);
  }

  function updateGuided(
    updates: Partial<GuidedAuthoringState>,
    nextAgentKind?: AgentKind,
  ) {
    const merged: GuidedAuthoringState = {
      ...guided,
      ...updates,
      agentKind: nextAgentKind ?? guided.agentKind,
    };
    const base = specsFromStrings(nextAgentKind ?? agentKind, specs);
    const nextSpecs = specsFromGuidedState(merged, {
      ...originalSpecs,
      ...base,
    });
    syncSpecsFromEditable(nextSpecs);
    clearValidationError("policy_spec");
    clearValidationError("interface_spec");
  }

  function applyTemplate(templateID: string) {
    const templateSpecs = versionPayloadFromTemplate(templateID);
    syncSpecsFromEditable(templateSpecs);
    toast.success("Applied guided starter");
  }

  function buildRequestBody() {
    const body: Record<string, unknown> = { agent_kind: agentKind };
    for (const field of specFields) {
      const raw = specs[field.key].trim();
      if (raw) {
        try {
          body[field.key] = JSON.parse(raw);
        } catch {
          toast.error(`Invalid JSON in ${field.label}`);
          setActiveTab("json");
          return null;
        }
      } else if (field.required) {
        body[field.key] = {};
      }
    }
    return body;
  }

  async function handleSave() {
    const body = buildRequestBody();
    if (!body) return;

    setSaving(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      await api.patch(`/v1/agent-build-versions/${version.id}`, body);
      toast.success("Saved");
      router.refresh();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to save");
    } finally {
      setSaving(false);
    }
  }

  async function handleValidate() {
    setValidating(true);
    setValidationErrors({});
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const result = await api.post<ValidationResult>(
        `/v1/agent-build-versions/${version.id}/validate`,
      );
      if (result.valid) {
        toast.success("Validation passed");
      } else {
        const errorMap: Record<string, string> = {};
        for (const error of result.errors) {
          errorMap[error.field] = error.message;
        }
        setValidationErrors(errorMap);
        toast.error(`${result.errors.length} validation error(s)`);
      }
    } catch (err) {
      toast.error(
        err instanceof ApiError ? err.message : "Validation failed",
      );
    } finally {
      setValidating(false);
    }
  }

  async function handleMarkReady() {
    setMarkingReady(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      await api.post(`/v1/agent-build-versions/${version.id}/ready`);
      toast.success("Version marked as ready");
      router.refresh();
    } catch (err) {
      if (err instanceof ApiError && err.code === "validation_failed") {
        toast.error("Cannot mark ready — fix validation errors first");
        void handleValidate();
      } else {
        toast.error(
          err instanceof ApiError ? err.message : "Failed to mark ready",
        );
      }
    } finally {
      setMarkingReady(false);
    }
  }

  const outputModeHelp =
    guided.outputMode === "custom"
      ? "This version already uses a custom output schema. Switch to JSON to fine-tune it, or pick a guided preset below to replace it."
      : "Pick the shape of the final answer contract without hand-authoring JSON.";

  return (
    <div className="max-w-4xl space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <h2 className="text-sm font-semibold">Version {version.version_number}</h2>
          <Badge
            variant={
              version.version_status === "ready"
                ? "default"
                : version.version_status === "draft"
                  ? "outline"
                  : "secondary"
            }
          >
            {version.version_status}
          </Badge>
        </div>
        {!isLocked ? (
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={handleValidate}
              disabled={validating}
            >
              {validating ? (
                <Loader2 className="size-4 animate-spin" />
              ) : (
                <>
                  <ShieldCheck data-icon="inline-start" className="size-4" />
                  Validate
                </>
              )}
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={handleSave}
              disabled={saving}
            >
              {saving ? (
                <Loader2 className="size-4 animate-spin" />
              ) : (
                <>
                  <Save data-icon="inline-start" className="size-4" />
                  Save Draft
                </>
              )}
            </Button>
            <Button size="sm" onClick={handleMarkReady} disabled={markingReady}>
              {markingReady ? (
                <Loader2 className="size-4 animate-spin" />
              ) : (
                <>
                  <CheckCircle data-icon="inline-start" className="size-4" />
                  Mark Ready
                </>
              )}
            </Button>
          </div>
        ) : null}
      </div>

      {isLocked ? (
        <div className="rounded-lg border border-white/[0.06] bg-white/[0.02] p-3 text-sm text-muted-foreground">
          This version is locked. Fields cannot be edited after marking as ready.
        </div>
      ) : null}

      <div className="rounded-2xl border border-white/[0.06] bg-white/[0.02] p-5">
        <div className="mb-4 flex items-start justify-between gap-4">
          <div>
            <p className="text-sm font-semibold">Authoring Mode</p>
            <p className="mt-1 text-sm text-muted-foreground">
              Guided mode is the beginner-first surface. JSON mode stays available
              as the advanced escape hatch and both stay in sync.
            </p>
          </div>
          <div className="rounded-full border border-white/[0.08] bg-white/[0.03] px-3 py-1 text-xs text-muted-foreground">
            Same API payloads, less manual JSON
          </div>
        </div>

        <Tabs
          value={activeTab}
          onValueChange={(value) => setActiveTab(value as "guided" | "json")}
          className="gap-4"
        >
          <TabsList variant="default">
            <TabsTrigger value="guided">
              <Sparkles className="size-4" />
              Guided
            </TabsTrigger>
            <TabsTrigger value="json">Advanced JSON</TabsTrigger>
          </TabsList>

          <TabsContent value="guided" className="space-y-6">
            <div className="rounded-xl border border-white/[0.06] bg-black/[0.12] p-4">
              <div className="mb-3">
                <p className="text-sm font-semibold">Starter Templates</p>
                <p className="mt-1 text-xs text-muted-foreground">
                  Borrow the strongest beginner pattern from tools like Dify and
                  Flowise: start from a template, then tune it in plain language.
                </p>
              </div>
              <div className="grid gap-3 md:grid-cols-2">
                {guidedTemplates.map((template) => (
                  <button
                    key={template.id}
                    type="button"
                    onClick={() => applyTemplate(template.id)}
                    disabled={isLocked}
                    className="rounded-xl border border-white/[0.08] bg-white/[0.02] p-4 text-left transition hover:border-white/[0.16] hover:bg-white/[0.04] disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    <div className="flex items-center justify-between gap-3">
                      <span className="text-sm font-medium">{template.name}</span>
                      <span className="text-[11px] uppercase tracking-[0.12em] text-muted-foreground">
                        Apply
                      </span>
                    </div>
                    <p className="mt-2 text-sm text-muted-foreground">
                      {template.summary}
                    </p>
                  </button>
                ))}
              </div>
            </div>

            <div className="grid gap-4 md:grid-cols-2">
              <div className="space-y-1.5">
                <label className="block text-sm font-medium">Agent Kind</label>
                <p className="text-xs text-muted-foreground">
                  Pick the closest execution style, then tune the rest in plain
                  language below.
                </p>
                <select
                  value={agentKind}
                  onChange={(event) => {
                    const next = event.target.value as AgentKind;
                    updateGuided({ agentKind: next }, next);
                    clearValidationError("agent_kind");
                  }}
                  disabled={isLocked}
                  className="block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {AGENT_KINDS.map((kind) => (
                    <option key={kind} value={kind}>
                      {kind}
                    </option>
                  ))}
                </select>
                {validationErrors.agent_kind ? (
                  <p className="text-xs text-destructive">
                    {validationErrors.agent_kind}
                  </p>
                ) : null}
              </div>

              <div className="space-y-1.5">
                <label className="block text-sm font-medium">
                  Primary Input Field
                </label>
                <p className="text-xs text-muted-foreground">
                  Name the main thing this agent receives, like
                  <code className="mx-1 rounded bg-white/[0.06] px-1.5 py-0.5">
                    user_request
                  </code>
                  or
                  <code className="mx-1 rounded bg-white/[0.06] px-1.5 py-0.5">
                    support_ticket
                  </code>
                  .
                </p>
                <Input
                  value={guided.primaryInput}
                  onChange={(event) =>
                    updateGuided({ primaryInput: event.target.value })
                  }
                  disabled={isLocked}
                />
                {validationErrors.interface_spec ? (
                  <p className="text-xs text-destructive">
                    {validationErrors.interface_spec}
                  </p>
                ) : null}
              </div>
            </div>

            <GuidedTextArea
              label="Role"
              value={guided.role}
              onChange={(value) => updateGuided({ role: value })}
              description="Who the agent is and the lens it should bring to the task."
              disabled={isLocked}
              rows={3}
            />

            <GuidedTextArea
              label="Mission / Instructions"
              value={guided.instructions}
              onChange={(value) => updateGuided({ instructions: value })}
              description="The core job to do. This writes directly into policy_spec.instructions, which the backend already validates."
              disabled={isLocked}
              rows={5}
              error={validationErrors.policy_spec}
            />

            <div className="grid gap-4 md:grid-cols-2">
              <GuidedTextArea
                label="System Prompt Add-On"
                value={guided.systemPrompt}
                onChange={(value) => updateGuided({ systemPrompt: value })}
                description="Extra steering that should always sit around the mission."
                disabled={isLocked}
                rows={4}
              />
              <GuidedTextArea
                label="Success Criteria"
                value={guided.successConditions}
                onChange={(value) => updateGuided({ successConditions: value })}
                description="What a great result must include before the agent is done."
                disabled={isLocked}
                rows={4}
              />
            </div>

            <div className="grid gap-4 md:grid-cols-2">
              <GuidedSelect
                label="Tool Strategy"
                value={guided.toolStrategy}
                onChange={(value) =>
                  updateGuided({
                    toolStrategy: value as GuidedAuthoringState["toolStrategy"],
                  })
                }
                description="Tell the runtime whether this agent should stay manual, use tools opportunistically, or reach for tools first."
                disabled={isLocked}
                options={[
                  { value: "manual_only", label: "Manual only" },
                  { value: "use_when_helpful", label: "Use tools when helpful" },
                  { value: "prefer_tools_first", label: "Prefer tools first" },
                ]}
              />

              <GuidedSelect
                label="Memory Style"
                value={guided.memoryMode}
                onChange={(value) =>
                  updateGuided({
                    memoryMode: value as GuidedAuthoringState["memoryMode"],
                  })
                }
                description="A small hint for how much context the build version expects to keep around."
                disabled={isLocked}
                options={[
                  { value: "none", label: "Fresh every run" },
                  { value: "session", label: "Remember this session" },
                  { value: "extended", label: "Keep a longer working memory" },
                ]}
              />
            </div>

            <div className="rounded-xl border border-white/[0.06] bg-black/[0.12] p-4">
              <div className="mb-3">
                <p className="text-sm font-semibold">Output Contract</p>
                <p className="mt-1 text-xs text-muted-foreground">
                  {outputModeHelp}
                </p>
              </div>
              <div className="grid gap-3 md:grid-cols-3">
                {[
                  {
                    value: "freeform_text",
                    label: "Freeform Text",
                    body: "No schema. Best for conversational or exploratory outputs.",
                  },
                  {
                    value: "answer_object",
                    label: "Structured Answer",
                    body: "A simple JSON object with a required answer field.",
                  },
                  {
                    value: "answer_summary_citations",
                    label: "Answer + Evidence",
                    body: "Adds a summary and citations array for evidence-heavy tasks.",
                  },
                ].map((option) => {
                  const active = guided.outputMode === option.value;
                  return (
                    <button
                      key={option.value}
                      type="button"
                      onClick={() =>
                        updateGuided({
                          outputMode: option.value as GuidedAuthoringState["outputMode"],
                        })
                      }
                      disabled={isLocked}
                      className={[
                        "rounded-xl border p-4 text-left transition disabled:cursor-not-allowed disabled:opacity-50",
                        active
                          ? "border-foreground/40 bg-white/[0.06]"
                          : "border-white/[0.08] bg-white/[0.02] hover:border-white/[0.16] hover:bg-white/[0.04]",
                      ].join(" ")}
                    >
                      <p className="text-sm font-medium">{option.label}</p>
                      <p className="mt-2 text-sm text-muted-foreground">
                        {option.body}
                      </p>
                    </button>
                  );
                })}
              </div>
              {guided.outputMode === "custom" ? (
                <p className="mt-3 text-xs text-amber-300/90">
                  This draft currently has a custom output schema. Guided presets
                  will replace it once you choose one.
                </p>
              ) : null}
            </div>
          </TabsContent>

          <TabsContent value="json" className="space-y-5">
            <div className="rounded-xl border border-white/[0.06] bg-black/[0.12] p-4 text-sm text-muted-foreground">
              Advanced mode keeps every raw spec editable for power users, CLI
              parity, and edge cases. It is still fully supported; it just is not
              the first thing a new user sees anymore.
            </div>

            <div className="space-y-5">
              {specFields.map((field) => (
                <JsonField
                  key={field.key}
                  label={field.label + (field.required ? "" : " (optional)")}
                  description={field.description}
                  value={specs[field.key]}
                  onChange={(value) => updateSpec(field.key, value)}
                  error={validationErrors[field.key]}
                  disabled={isLocked}
                />
              ))}
            </div>
          </TabsContent>
        </Tabs>
      </div>
    </div>
  );
}
