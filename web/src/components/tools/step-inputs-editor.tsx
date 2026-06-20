"use client";

import { Button } from "@/components/ui/button";
import { Plus, Trash2 } from "lucide-react";
import { controlClass } from "./field";
import { ValueField } from "./value-field";
import type { ValueReference } from "./lib/friendly";

/**
 * Edits a free-form map of inputs (name → value) for a step or tool reference,
 * where the value can be literal text or an inserted reference. Used for tool
 * references whose target schema we don't know, so keys are user-defined.
 */
export function StepInputsEditor({
  inputs,
  onChange,
  references,
  allowSecrets,
  label = "Inputs",
  emptyHint = "No inputs set yet.",
}: {
  inputs: Record<string, unknown>;
  onChange: (inputs: Record<string, unknown>) => void;
  references: ValueReference[];
  allowSecrets: boolean;
  label?: string;
  emptyHint?: string;
}) {
  const rows = Object.entries(inputs);

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
        <span className="text-xs font-medium text-muted-foreground">{label}</span>
        <Button type="button" variant="ghost" size="xs" onClick={add}>
          <Plus data-icon="inline-start" className="size-3" />
          Add input
        </Button>
      </div>
      {rows.length === 0 ? (
        <p className="text-xs text-muted-foreground">{emptyHint}</p>
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
