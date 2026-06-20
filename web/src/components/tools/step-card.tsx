"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { ChevronDown, ChevronUp, GripVertical, Plus, Trash2 } from "lucide-react";
import { controlClass } from "./field";
import type { ComposedStep, StepRefType, ToolPrimitive } from "./lib/types";

export function StepCard({
  step,
  index,
  total,
  earlierStepIds,
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
        <code className="font-[family-name:var(--font-mono)] text-xs text-muted-foreground">
          {step.id}
        </code>
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
        <div className="grid grid-cols-[8rem_1fr] gap-2">
          <select
            value={step.ref.type}
            onChange={(e) => setRef({ type: e.target.value as StepRefType, name: "" })}
            aria-label="Reference type"
            className={controlClass}
          >
            <option value="primitive">Primitive</option>
            <option value="tool">Saved tool</option>
          </select>
          {step.ref.type === "primitive" ? (
            <select
              value={step.ref.name}
              onChange={(e) => setRef({ name: e.target.value })}
              aria-label="Primitive"
              className={controlClass}
            >
              <option value="">Select a primitive…</option>
              {delegatable.map((p) => (
                <option key={p.name} value={p.name}>
                  {p.name}
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
              <option value="">Select a tool…</option>
              {toolOptions.map((t) => (
                <option key={t.slug} value={t.slug}>
                  {t.name} ({t.slug})
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
  allowSecrets,
}: {
  inputs: Record<string, unknown>;
  onChange: (inputs: Record<string, unknown>) => void;
  paramNames: string[];
  earlierStepIds: string[];
  allowSecrets: boolean;
}) {
  const [focusedKey, setFocusedKey] = useState<string | null>(null);
  const rows = Object.entries(inputs);

  function setKey(oldKey: string, newKey: string) {
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
  function insert(token: string) {
    if (!focusedKey) return;
    const cur = typeof inputs[focusedKey] === "string" ? (inputs[focusedKey] as string) : "";
    setValue(focusedKey, cur + token);
  }

  const chips = [
    ...paramNames.map((p) => `\${params.${p}}`),
    ...earlierStepIds.map((s) => `\${steps.${s}.output}`),
    ...(allowSecrets ? ["${secrets.NAME}"] : []),
  ];

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <span className="text-xs font-medium text-muted-foreground">Inputs</span>
        <Button type="button" variant="ghost" size="xs" onClick={add}>
          <Plus data-icon="inline-start" className="size-3" />
          Add input
        </Button>
      </div>
      {rows.length === 0 ? (
        <p className="text-xs text-muted-foreground">No inputs mapped.</p>
      ) : (
        rows.map(([key, value]) => (
          <div key={key} className="grid grid-cols-[10rem_1fr_auto] items-center gap-2">
            <input
              value={key}
              onChange={(e) => setKey(key, e.target.value)}
              aria-label="Input name"
              className={`${controlClass} font-[family-name:var(--font-mono)] text-xs`}
            />
            <input
              value={typeof value === "string" ? value : JSON.stringify(value)}
              onChange={(e) => setValue(key, e.target.value)}
              onFocus={() => setFocusedKey(key)}
              placeholder="value or ${params.x}"
              aria-label="Input value"
              className={`${controlClass} font-[family-name:var(--font-mono)] text-xs`}
            />
            <Button type="button" variant="ghost" size="icon-sm" onClick={() => remove(key)} aria-label="Remove input">
              <Trash2 className="size-3.5 text-muted-foreground" />
            </Button>
          </div>
        ))
      )}
      {chips.length > 0 && (
        <div className="flex flex-wrap items-center gap-1.5">
          {chips.map((c) => (
            <button
              key={c}
              type="button"
              onClick={() => insert(c)}
              disabled={!focusedKey}
              className={cn(
                "rounded-md border border-border bg-background px-1.5 py-0.5 font-[family-name:var(--font-mono)] text-[11px] text-muted-foreground transition-colors hover:text-foreground disabled:opacity-50",
              )}
            >
              {c}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
