"use client";

import { Button } from "@/components/ui/button";
import { ChevronDown, ChevronUp, GripVertical, Plus, Trash2 } from "lucide-react";
import { controlClass } from "./field";
import { ValueField } from "./value-field";
import { operationLabel, stepReferences } from "./lib/friendly";
import type { ComposedStep, StepRefType, ToolPrimitive } from "./lib/types";

export function StepCard({
  step,
  index,
  total,
  earlierStepIds,
  stepNumberById,
  paramNames,
  primitives,
  toolOptions,
  onChange,
  onRemove,
  onMoveUp,
  onMoveDown,
  onDragStart,
  onDragOver,
  onDrop,
}: {
  step: ComposedStep;
  index: number;
  total: number;
  earlierStepIds: string[];
  stepNumberById: Map<string, number>;
  paramNames: string[];
  primitives: ToolPrimitive[];
  toolOptions: { slug: string; name: string }[];
  onChange: (step: ComposedStep) => void;
  onRemove: () => void;
  onMoveUp: () => void;
  onMoveDown: () => void;
  onDragStart: () => void;
  onDragOver: (e: React.DragEvent) => void;
  onDrop: () => void;
}) {
  const delegatable = primitives.filter((p) => p.delegatable);
  const isHTTP = step.ref.type === "primitive" && step.ref.name === "http_request";

  function setRef(patch: Partial<ComposedStep["ref"]>) {
    onChange({ ...step, ref: { ...step.ref, ...patch } });
  }

  return (
    <div
      draggable
      onDragStart={onDragStart}
      onDragOver={onDragOver}
      onDrop={onDrop}
      className="rounded-lg border border-border bg-card"
    >
      <div className="flex items-center gap-2 border-b border-border px-2 py-1.5">
        <GripVertical className="size-4 cursor-grab text-muted-foreground" aria-hidden />
        <span className="flex size-5 items-center justify-center rounded-full bg-muted text-xs font-medium tabular-nums">
          {index + 1}
        </span>
        <span className="text-xs font-medium text-muted-foreground">Step {index + 1}</span>
        <div className="ml-auto flex items-center">
          <Button type="button" variant="ghost" size="icon-sm" onClick={onMoveUp} disabled={index === 0} aria-label="Move up">
            <ChevronUp className="size-3.5" />
          </Button>
          <Button type="button" variant="ghost" size="icon-sm" onClick={onMoveDown} disabled={index === total - 1} aria-label="Move down">
            <ChevronDown className="size-3.5" />
          </Button>
          <Button type="button" variant="ghost" size="icon-sm" onClick={onRemove} aria-label="Remove step">
            <Trash2 className="size-3.5 text-muted-foreground" />
          </Button>
        </div>
      </div>

      <div className="space-y-3 p-3">
        <div className="grid grid-cols-[9rem_1fr] gap-2">
          <select
            value={step.ref.type}
            onChange={(e) => setRef({ type: e.target.value as StepRefType, name: "" })}
            aria-label="What this step runs"
            className={controlClass}
          >
            <option value="primitive">Built-in operation</option>
            <option value="tool">Another saved tool</option>
          </select>
          {step.ref.type === "primitive" ? (
            <select
              value={step.ref.name}
              onChange={(e) => setRef({ name: e.target.value })}
              aria-label="Operation"
              className={controlClass}
            >
              <option value="">Choose an operation…</option>
              {delegatable.map((p) => (
                <option key={p.name} value={p.name}>
                  {operationLabel(p.name)}
                </option>
              ))}
            </select>
          ) : (
            <select
              value={step.ref.name}
              onChange={(e) => setRef({ name: e.target.value })}
              aria-label="Tool"
              className={controlClass}
            >
              <option value="">Choose a tool…</option>
              {toolOptions.map((t) => (
                <option key={t.slug} value={t.slug}>
                  {t.name}
                </option>
              ))}
            </select>
          )}
        </div>

        <StepInputsEditor
          inputs={step.inputs}
          onChange={(inputs) => onChange({ ...step, inputs })}
          paramNames={paramNames}
          earlierStepIds={earlierStepIds}
          stepNumberById={stepNumberById}
          allowSecrets={isHTTP}
        />
      </div>
    </div>
  );
}

function StepInputsEditor({
  inputs,
  onChange,
  paramNames,
  earlierStepIds,
  stepNumberById,
  allowSecrets,
}: {
  inputs: Record<string, unknown>;
  onChange: (inputs: Record<string, unknown>) => void;
  paramNames: string[];
  earlierStepIds: string[];
  stepNumberById: Map<string, number>;
  allowSecrets: boolean;
}) {
  const rows = Object.entries(inputs);
  const references = stepReferences(paramNames, earlierStepIds, stepNumberById);

  function setKey(oldKey: string, newKey: string) {
    // Don't merge into an existing input — that would silently drop a value.
    if (newKey !== oldKey && newKey in inputs) return;
    const next: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(inputs)) next[k === oldKey ? newKey : k] = v;
    onChange(next);
  }
  function setValue(key: string, value: string) {
    onChange({ ...inputs, [key]: value });
  }
  function remove(key: string) {
    const next = { ...inputs };
    delete next[key];
    onChange(next);
  }
  function add() {
    let name = "input";
    let n = 1;
    while (name in inputs) name = `input${++n}`;
    onChange({ ...inputs, [name]: "" });
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <span className="text-xs font-medium text-muted-foreground">Inputs for this step</span>
        <Button type="button" variant="ghost" size="xs" onClick={add}>
          <Plus data-icon="inline-start" className="size-3" />
          Add input
        </Button>
      </div>
      {rows.length === 0 ? (
        <p className="text-xs text-muted-foreground">No inputs set for this step yet.</p>
      ) : (
        rows.map(([key, value]) => (
          <div key={key} className="grid grid-cols-[9rem_1fr_auto] items-start gap-2">
            <input
              value={key}
              onChange={(e) => setKey(key, e.target.value)}
              aria-label="Input name"
              className={`${controlClass} font-[family-name:var(--font-mono)] text-xs`}
            />
            <ValueField
              value={typeof value === "string" ? value : JSON.stringify(value)}
              onChange={(v) => setValue(key, v)}
              placeholder="Type a value or insert one"
              references={references}
              allowSecret={allowSecrets}
              ariaLabel="Input value"
            />
            <Button type="button" variant="ghost" size="icon-sm" onClick={() => remove(key)} aria-label="Remove input">
              <Trash2 className="size-3.5 text-muted-foreground" />
            </Button>
          </div>
        ))
      )}
    </div>
  );
}
