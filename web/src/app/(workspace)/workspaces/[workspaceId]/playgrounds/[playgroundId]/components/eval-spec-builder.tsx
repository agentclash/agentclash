"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Plus, Trash2, Code2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";

// ---------------------------------------------------------------------------
// Types matching backend/internal/scoring/spec.go
// ---------------------------------------------------------------------------

type JudgeMode = "deterministic" | "llm_judge" | "hybrid";

type ValidatorType =
  | "exact_match"
  | "contains"
  | "regex_match"
  | "json_schema"
  | "json_path_match"
  | "boolean_assert";

type MetricType = "numeric" | "text" | "boolean";

type ScorecardDimensionName =
  | "correctness"
  | "reliability"
  | "latency"
  | "cost";

interface ValidatorDeclaration {
  key: string;
  type: ValidatorType;
  target: string;
  expected_from: string;
}

interface MetricDeclaration {
  key: string;
  type: MetricType;
  collector: string;
  unit?: string;
}

// Backend ScorecardDimension is a plain string, not an object.
type ScorecardDimension = ScorecardDimensionName;

interface LatencyNormalization {
  target_ms?: number;
  max_ms?: number;
}

interface CostNormalization {
  target_usd?: number;
  max_usd?: number;
}

interface ScorecardNormalization {
  latency?: LatencyNormalization;
  cost?: CostNormalization;
}

interface ScorecardDeclaration {
  dimensions: ScorecardDimensionName[];
  normalization?: ScorecardNormalization;
}

interface EvaluationSpec {
  name: string;
  version_number: number;
  judge_mode: JudgeMode;
  validators: ValidatorDeclaration[];
  metrics: MetricDeclaration[];
  scorecard: ScorecardDeclaration;
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const VALIDATOR_TYPES: { value: ValidatorType; label: string }[] = [
  { value: "exact_match", label: "Exact Match" },
  { value: "contains", label: "Contains" },
  { value: "regex_match", label: "Regex Match" },
  { value: "json_schema", label: "JSON Schema" },
  { value: "json_path_match", label: "JSON Path Match" },
  { value: "boolean_assert", label: "Boolean Assert" },
];

const TARGET_OPTIONS = [
  { value: "final_output", label: "final_output" },
  { value: "tool_calls", label: "tool_calls" },
  { value: "intermediate_steps", label: "intermediate_steps" },
];

const COLLECTOR_OPTIONS: {
  value: string;
  label: string;
  type: MetricType;
  unit: string;
}[] = [
  {
    value: "run_total_latency_ms",
    label: "Total Latency",
    type: "numeric",
    unit: "ms",
  },
  {
    value: "run_model_cost_usd",
    label: "Model Cost",
    type: "numeric",
    unit: "USD",
  },
  {
    value: "run_input_tokens",
    label: "Input Tokens",
    type: "numeric",
    unit: "tokens",
  },
  {
    value: "run_output_tokens",
    label: "Output Tokens",
    type: "numeric",
    unit: "tokens",
  },
  {
    value: "run_total_tokens",
    label: "Total Tokens",
    type: "numeric",
    unit: "tokens",
  },
];

const ALL_DIMENSIONS: ScorecardDimensionName[] = [
  "correctness",
  "reliability",
  "latency",
  "cost",
];

// ---------------------------------------------------------------------------
// Preset templates
// ---------------------------------------------------------------------------

type PresetKey = "basic" | "strict" | "custom";

function makeBasicPreset(): Omit<EvaluationSpec, "name" | "version_number" | "judge_mode"> {
  return {
    validators: [
      {
        key: "contains-1",
        type: "contains",
        target: "final_output",
        expected_from: "case.expectations.expected_output",
      },
    ],
    metrics: [
      { key: "run_total_latency_ms", type: "numeric", collector: "run_total_latency_ms", unit: "ms" },
      { key: "run_model_cost_usd", type: "numeric", collector: "run_model_cost_usd", unit: "USD" },
    ],
    scorecard: {
      dimensions: ["correctness", "latency", "cost"],
    },
  };
}

function makeStrictPreset(): Omit<EvaluationSpec, "name" | "version_number" | "judge_mode"> {
  return {
    validators: [
      {
        key: "exact_match-1",
        type: "exact_match",
        target: "final_output",
        expected_from: "case.expectations.expected_output",
      },
    ],
    metrics: COLLECTOR_OPTIONS.map((c) => ({
      key: c.value,
      type: c.type,
      collector: c.value,
      unit: c.unit,
    })),
    scorecard: {
      dimensions: [...ALL_DIMENSIONS],
    },
  };
}

// ---------------------------------------------------------------------------
// Internal form state
// ---------------------------------------------------------------------------

interface FormState {
  specName: string;
  validators: ValidatorDeclaration[];
  metrics: MetricDeclaration[];
  enabledDimensions: Set<ScorecardDimensionName>;
  latencyTargetMs: string;
  latencyMaxMs: string;
  costTargetUsd: string;
  costMaxUsd: string;
}

function defaultFormState(): FormState {
  return {
    specName: "",
    validators: [],
    metrics: [],
    enabledDimensions: new Set(),
    latencyTargetMs: "",
    latencyMaxMs: "",
    costTargetUsd: "",
    costMaxUsd: "",
  };
}

function parseIncoming(value: unknown): FormState {
  let spec: Partial<EvaluationSpec> = {};

  if (typeof value === "string") {
    try {
      spec = JSON.parse(value);
    } catch {
      return defaultFormState();
    }
  } else if (value && typeof value === "object") {
    spec = value as Partial<EvaluationSpec>;
  }

  const dims = new Set<ScorecardDimensionName>();
  if (spec.scorecard?.dimensions) {
    for (const d of spec.scorecard.dimensions) {
      const name = (typeof d === "string" ? d : (d as { name: string }).name) as ScorecardDimensionName;
      if (ALL_DIMENSIONS.includes(name)) {
        dims.add(name);
      }
    }
  }

  const latencyNorm = spec.scorecard?.normalization?.latency;
  const costNorm = spec.scorecard?.normalization?.cost;

  return {
    specName: spec.name ?? "",
    validators: spec.validators ?? [],
    metrics: spec.metrics ?? [],
    enabledDimensions: dims,
    latencyTargetMs: latencyNorm?.target_ms != null ? String(latencyNorm.target_ms) : "",
    latencyMaxMs: latencyNorm?.max_ms != null ? String(latencyNorm.max_ms) : "",
    costTargetUsd: costNorm?.target_usd != null ? String(costNorm.target_usd) : "",
    costMaxUsd: costNorm?.max_usd != null ? String(costNorm.max_usd) : "",
  };
}

function formToSpec(state: FormState): EvaluationSpec {
  const dimensions: ScorecardDimensionName[] = ALL_DIMENSIONS.filter((d) =>
    state.enabledDimensions.has(d)
  );

  const normalization: ScorecardNormalization = {};

  if (state.enabledDimensions.has("latency")) {
    const targetMs = parseFloat(state.latencyTargetMs);
    const maxMs = parseFloat(state.latencyMaxMs);
    if (!isNaN(targetMs) || !isNaN(maxMs)) {
      normalization.latency = {};
      if (!isNaN(targetMs)) normalization.latency.target_ms = targetMs;
      if (!isNaN(maxMs)) normalization.latency.max_ms = maxMs;
    }
  }

  if (state.enabledDimensions.has("cost")) {
    const targetUsd = parseFloat(state.costTargetUsd);
    const maxUsd = parseFloat(state.costMaxUsd);
    if (!isNaN(targetUsd) || !isNaN(maxUsd)) {
      normalization.cost = {};
      if (!isNaN(targetUsd)) normalization.cost.target_usd = targetUsd;
      if (!isNaN(maxUsd)) normalization.cost.max_usd = maxUsd;
    }
  }

  const scorecard: ScorecardDeclaration = { dimensions };
  if (Object.keys(normalization).length > 0) {
    scorecard.normalization = normalization;
  }

  return {
    name: state.specName,
    version_number: 1,
    judge_mode: "deterministic",
    validators: state.validators,
    metrics: state.metrics,
    scorecard,
  };
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export interface EvalSpecBuilderProps {
  value: unknown;
  onChange: (spec: unknown) => void;
}

export function EvalSpecBuilder({ value, onChange }: EvalSpecBuilderProps) {
  const [form, setForm] = useState<FormState>(() => parseIncoming(value));
  const [preset, setPreset] = useState<PresetKey>("custom");
  const [showJson, setShowJson] = useState(false);

  // Reparse if the external value changes identity (e.g. server refresh)
  useEffect(() => {
    setForm(parseIncoming(value));
  }, [value]);

  // Propagate changes upstream
  const emit = useCallback(
    (next: FormState) => {
      setForm(next);
      onChange(formToSpec(next));
    },
    [onChange],
  );

  // Preset handling
  function applyPreset(key: PresetKey) {
    setPreset(key);
    if (key === "custom") return;
    const template = key === "basic" ? makeBasicPreset() : makeStrictPreset();
    const next: FormState = {
      ...form,
      validators: template.validators,
      metrics: template.metrics,
      enabledDimensions: new Set(template.scorecard.dimensions),
      latencyTargetMs: "",
      latencyMaxMs: "",
      costTargetUsd: "",
      costMaxUsd: "",
    };
    emit(next);
  }

  // ---- Validator helpers ----

  function addValidator() {
    const idx = form.validators.length + 1;
    const next: FormState = {
      ...form,
      validators: [
        ...form.validators,
        {
          key: `contains-${idx}`,
          type: "contains",
          target: "final_output",
          expected_from: "case.expectations.expected_output",
        },
      ],
    };
    setPreset("custom");
    emit(next);
  }

  function updateValidator(index: number, patch: Partial<ValidatorDeclaration>) {
    const validators = form.validators.map((v, i) => {
      if (i !== index) return v;
      const updated = { ...v, ...patch };
      // Auto-generate key from type + (index+1)
      if (patch.type) {
        updated.key = `${patch.type}-${i + 1}`;
      }
      return updated;
    });
    setPreset("custom");
    emit({ ...form, validators });
  }

  function removeValidator(index: number) {
    const validators = form.validators.filter((_, i) => i !== index);
    setPreset("custom");
    emit({ ...form, validators });
  }

  // ---- Metric helpers ----

  function addMetric() {
    const next: FormState = {
      ...form,
      metrics: [
        ...form.metrics,
        {
          key: "run_total_latency_ms",
          type: "numeric",
          collector: "run_total_latency_ms",
          unit: "ms",
        },
      ],
    };
    setPreset("custom");
    emit(next);
  }

  function updateMetricCollector(index: number, collector: string) {
    const info = COLLECTOR_OPTIONS.find((c) => c.value === collector);
    if (!info) return;
    const metrics = form.metrics.map((m, i) =>
      i === index
        ? { key: info.value, type: info.type, collector: info.value, unit: info.unit }
        : m,
    );
    setPreset("custom");
    emit({ ...form, metrics });
  }

  function removeMetric(index: number) {
    const metrics = form.metrics.filter((_, i) => i !== index);
    setPreset("custom");
    emit({ ...form, metrics });
  }

  // ---- Dimension helpers ----

  function toggleDimension(name: ScorecardDimensionName) {
    const next = new Set(form.enabledDimensions);
    if (next.has(name)) {
      next.delete(name);
    } else {
      next.add(name);
    }
    setPreset("custom");
    emit({ ...form, enabledDimensions: next });
  }

  // ---- Normalization helpers ----

  function setNormField(field: keyof Pick<FormState, "latencyTargetMs" | "latencyMaxMs" | "costTargetUsd" | "costMaxUsd">, val: string) {
    setPreset("custom");
    emit({ ...form, [field]: val });
  }

  // ---- Computed JSON ----

  const specJson = useMemo(() => {
    return JSON.stringify(formToSpec(form), null, 2);
  }, [form]);

  return (
    <div className="space-y-6">
      {/* Header row: name + preset selector */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
        <div className="flex-1 space-y-2">
          <label className="text-sm font-medium">Spec Name</label>
          <Input
            value={form.specName}
            onChange={(e) => emit({ ...form, specName: e.target.value })}
            placeholder="e.g. my-eval-spec"
          />
        </div>
        <div className="space-y-2">
          <label className="text-sm font-medium">Preset Template</label>
          <Select value={preset} onValueChange={(v) => v && applyPreset(v as PresetKey)}>
            <SelectTrigger className="w-full sm:w-44">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="basic">Basic</SelectItem>
              <SelectItem value="strict">Strict</SelectItem>
              <SelectItem value="custom">Custom</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      {/* LLM Judge toggle (disabled / coming-soon) */}
      {/* TODO(#245): Enable LLM-as-judge evaluation mode */}
      {/* See: https://github.com/agentclash/agentclash/issues/XXX */}
      <div className="flex items-center gap-3 rounded-lg border border-border bg-muted/30 px-4 py-3">
        <div className="flex items-center gap-2 opacity-50">
          <input
            type="checkbox"
            disabled
            className="size-4 rounded border-border"
          />
          <span className="text-sm font-medium">LLM-as-Judge Mode</span>
        </div>
        <Badge variant="secondary">Coming soon</Badge>
        <span className="text-xs text-muted-foreground">
          Currently using deterministic evaluation only.
        </span>
      </div>

      {/* Validators */}
      <section className="rounded-lg border border-border p-5 space-y-4">
        <div className="flex items-center justify-between">
          <div>
            <h3 className="text-sm font-semibold">Validators</h3>
            <p className="text-xs text-muted-foreground">
              Rules that check agent output against expected values.
            </p>
          </div>
          <Button variant="outline" size="sm" onClick={addValidator}>
            <Plus data-icon="inline-start" className="size-3.5" />
            Add Validator
          </Button>
        </div>

        {form.validators.length === 0 && (
          <p className="py-4 text-center text-sm text-muted-foreground">
            No validators configured. Click &quot;Add Validator&quot; to get started.
          </p>
        )}

        {form.validators.length > 0 && (
          <div className="hidden sm:flex items-center gap-3 px-3 text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
            <span className="shrink-0 w-20">Key</span>
            <span className="w-40">Type</span>
            <span className="w-40">Checks</span>
            <span className="flex-1">Compare Against (test case expectation field)</span>
          </div>
        )}
        <div className="space-y-3">
          {form.validators.map((v, i) => (
            <div
              key={i}
              className="flex flex-col gap-3 rounded-md border border-border p-3 sm:flex-row sm:items-center"
            >
              <div className="flex-1 space-y-2 sm:space-y-0 sm:flex sm:items-center sm:gap-3">
                <TooltipProvider>
                  <Tooltip>
                    <TooltipTrigger>
                      <Badge variant="outline" className="shrink-0">
                        {v.key}
                      </Badge>
                    </TooltipTrigger>
                    <TooltipContent>Auto-generated key</TooltipContent>
                  </Tooltip>
                </TooltipProvider>

                <Select
                  value={v.type}
                  onValueChange={(val) => val && updateValidator(i, { type: val as ValidatorType })}
                >
                  <SelectTrigger className="w-full sm:w-40">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {VALIDATOR_TYPES.map((vt) => (
                      <SelectItem key={vt.value} value={vt.value}>
                        {vt.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>

                <Select
                  value={v.target}
                  onValueChange={(val) => val && updateValidator(i, { target: val })}
                >
                  <SelectTrigger className="w-full sm:w-40">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {TARGET_OPTIONS.map((t) => (
                      <SelectItem key={t.value} value={t.value}>
                        {t.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>

                <div className="flex-1">
                  <Input
                    value={v.expected_from}
                    onChange={(e) => updateValidator(i, { expected_from: e.target.value })}
                    placeholder="case.expectations.expected_output"
                    className="text-xs"
                  />
                  <p className="mt-1 text-[10px] text-muted-foreground">
                    Path to expected value in your test case, e.g. <code className="font-mono">case.expectations.your_field</code>
                  </p>
                </div>
              </div>

              <Button
                variant="ghost"
                size="icon-sm"
                onClick={() => removeValidator(i)}
                aria-label="Remove validator"
              >
                <Trash2 className="size-3.5 text-muted-foreground" />
              </Button>
            </div>
          ))}
        </div>
      </section>

      {/* Metrics */}
      <section className="rounded-lg border border-border p-5 space-y-4">
        <div className="flex items-center justify-between">
          <div>
            <h3 className="text-sm font-semibold">Metrics</h3>
            <p className="text-xs text-muted-foreground">
              Numeric collectors attached to each run.
            </p>
          </div>
          <Button variant="outline" size="sm" onClick={addMetric}>
            <Plus data-icon="inline-start" className="size-3.5" />
            Add Metric
          </Button>
        </div>

        {form.metrics.length === 0 && (
          <p className="py-4 text-center text-sm text-muted-foreground">
            No metrics configured. Click &quot;Add Metric&quot; to get started.
          </p>
        )}

        <div className="space-y-3">
          {form.metrics.map((m, i) => (
            <div
              key={i}
              className="flex flex-col gap-3 rounded-md border border-border p-3 sm:flex-row sm:items-center"
            >
              <div className="flex-1 sm:flex sm:items-center sm:gap-3">
                <Badge variant="outline" className="shrink-0">
                  {m.key}
                </Badge>

                <Select
                  value={m.collector}
                  onValueChange={(val) => val && updateMetricCollector(i, val)}
                >
                  <SelectTrigger className="w-full sm:w-48">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {COLLECTOR_OPTIONS.map((c) => (
                      <SelectItem key={c.value} value={c.value}>
                        {c.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>

                <span className="shrink-0 text-xs text-muted-foreground">
                  Unit: {m.unit || "none"}
                </span>
              </div>

              <Button
                variant="ghost"
                size="icon-sm"
                onClick={() => removeMetric(i)}
                aria-label="Remove metric"
              >
                <Trash2 className="size-3.5 text-muted-foreground" />
              </Button>
            </div>
          ))}
        </div>
      </section>

      {/* Scorecard Dimensions */}
      <section className="rounded-lg border border-border p-5 space-y-4">
        <div>
          <h3 className="text-sm font-semibold">Scorecard Dimensions</h3>
          <p className="text-xs text-muted-foreground">
            Enable dimensions to include in the final scorecard.
          </p>
        </div>

        <div className="grid gap-3 sm:grid-cols-2">
          {ALL_DIMENSIONS.map((dim) => {
            const checked = form.enabledDimensions.has(dim);
            return (
              <div key={dim} className="space-y-2">
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={checked}
                    onChange={() => toggleDimension(dim)}
                    className="size-4 rounded border-border accent-primary"
                  />
                  <span className="text-sm font-medium capitalize">{dim}</span>
                </label>

                {/* Latency normalization inputs */}
                {dim === "latency" && checked && (
                  <div className="ml-6 flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-3">
                    <div className="space-y-1">
                      <label className="text-xs text-muted-foreground">Target (ms)</label>
                      <Input
                        type="number"
                        min={0}
                        value={form.latencyTargetMs}
                        onChange={(e) => setNormField("latencyTargetMs", e.target.value)}
                        placeholder="e.g. 5000"
                        className="w-32 text-xs"
                      />
                    </div>
                    <div className="space-y-1">
                      <label className="text-xs text-muted-foreground">Max (ms)</label>
                      <Input
                        type="number"
                        min={0}
                        value={form.latencyMaxMs}
                        onChange={(e) => setNormField("latencyMaxMs", e.target.value)}
                        placeholder="e.g. 30000"
                        className="w-32 text-xs"
                      />
                    </div>
                  </div>
                )}

                {/* Cost normalization inputs */}
                {dim === "cost" && checked && (
                  <div className="ml-6 flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-3">
                    <div className="space-y-1">
                      <label className="text-xs text-muted-foreground">Target (USD)</label>
                      <Input
                        type="number"
                        min={0}
                        step="0.01"
                        value={form.costTargetUsd}
                        onChange={(e) => setNormField("costTargetUsd", e.target.value)}
                        placeholder="e.g. 0.05"
                        className="w-32 text-xs"
                      />
                    </div>
                    <div className="space-y-1">
                      <label className="text-xs text-muted-foreground">Max (USD)</label>
                      <Input
                        type="number"
                        min={0}
                        step="0.01"
                        value={form.costMaxUsd}
                        onChange={(e) => setNormField("costMaxUsd", e.target.value)}
                        placeholder="e.g. 1.00"
                        className="w-32 text-xs"
                      />
                    </div>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </section>

      {/* View JSON toggle */}
      <div className="space-y-3">
        <Button
          variant="outline"
          size="sm"
          onClick={() => setShowJson((prev) => !prev)}
        >
          <Code2 data-icon="inline-start" className="size-3.5" />
          {showJson ? "Hide JSON" : "View JSON"}
        </Button>

        {showJson && (
          <pre className="overflow-x-auto rounded-lg border border-border bg-muted/50 p-4 font-[family-name:var(--font-mono)] text-xs leading-relaxed">
            {specJson}
          </pre>
        )}
      </div>
    </div>
  );
}
