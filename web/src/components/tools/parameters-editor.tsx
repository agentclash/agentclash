"use client";

import { Button } from "@/components/ui/button";
import { Plus, Trash2 } from "lucide-react";
import { controlClass } from "./field";
import { TYPE_OPTIONS } from "./lib/friendly";
import type { JsonSchemaType, ParamField } from "./lib/types";

/**
 * Edits a tool's input parameters as friendly rows. The parent converts to/from
 * JSON Schema with schemaToParams / paramsToSchema.
 */
export function ParametersEditor({
  params,
  onChange,
}: {
  params: ParamField[];
  onChange: (params: ParamField[]) => void;
}) {
  function update(index: number, patch: Partial<ParamField>) {
    onChange(params.map((p, i) => (i === index ? { ...p, ...patch } : p)));
  }
  function remove(index: number) {
    onChange(params.filter((_, i) => i !== index));
  }
  function add() {
    onChange([...params, { name: "", type: "string", required: true }]);
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-medium">Inputs the agent provides</h3>
        <Button type="button" variant="outline" size="sm" onClick={add}>
          <Plus data-icon="inline-start" className="size-3.5" />
          Add input
        </Button>
      </div>
      <p className="text-xs text-muted-foreground">
        The values the agent fills in when it calls this tool. You can drop these
        into the fields below using the “Insert” button.
      </p>

      {params.length === 0 ? (
        <div className="rounded-lg border border-dashed border-border p-4 text-center text-xs text-muted-foreground">
          No inputs yet. Add one if the tool needs information from the agent.
        </div>
      ) : (
        <div className="space-y-2">
          {params.map((p, i) => (
            <div
              key={i}
              className="grid grid-cols-[1fr_9.5rem_auto_auto] items-center gap-2 rounded-lg border border-border p-2"
            >
              <input
                value={p.name}
                onChange={(e) => update(i, { name: e.target.value })}
                placeholder="name"
                aria-label="Input name"
                className={`${controlClass} font-[family-name:var(--font-mono)] text-xs`}
              />
              <select
                value={p.type}
                onChange={(e) => update(i, { type: e.target.value as JsonSchemaType })}
                aria-label="Input type"
                className={`${controlClass} py-1.5`}
              >
                {TYPE_OPTIONS.map((t) => (
                  <option key={t.value} value={t.value}>
                    {t.label}
                  </option>
                ))}
              </select>
              <label className="flex items-center gap-1.5 px-1 text-xs text-muted-foreground">
                <input
                  type="checkbox"
                  checked={p.required}
                  onChange={(e) => update(i, { required: e.target.checked })}
                  className="size-3.5 accent-primary"
                />
                required
              </label>
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                onClick={() => remove(i)}
                aria-label="Remove input"
              >
                <Trash2 className="size-3.5 text-muted-foreground" />
              </Button>
              <input
                value={p.description ?? ""}
                onChange={(e) => update(i, { description: e.target.value })}
                placeholder="What is this input? (optional)"
                aria-label="Input description"
                className={`${controlClass} col-span-4 text-xs`}
              />
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
