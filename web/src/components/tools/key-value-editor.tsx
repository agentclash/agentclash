"use client";

import { useId, useState } from "react";
import { Code, Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { controlClass, monoControlClass } from "./field";
import { useReportJsonValidity } from "./json-validity";
import { ValueField } from "./value-field";
import type { ValueReference } from "./lib/friendly";

/**
 * Edits an object-typed argument (e.g. HTTP headers) as friendly key/value rows
 * instead of raw JSON. Anything that isn't a flat map of scalars (nested objects,
 * arrays) falls back to a JSON editor, which the user can also switch to manually.
 */
export function KeyValueEditor({
  value,
  onChange,
  references,
  allowSecret,
  keyPlaceholder = "name",
  valuePlaceholder = "value",
}: {
  value: unknown;
  onChange: (value: unknown) => void;
  references: ValueReference[];
  allowSecret: boolean;
  keyPlaceholder?: string;
  valuePlaceholder?: string;
}) {
  const flat = asFlatMap(value);
  const [jsonMode, setJsonMode] = useState(value !== undefined && flat === null);

  if (jsonMode) {
    return (
      <div className="space-y-1.5">
        <JsonMode value={value} onChange={onChange} />
        <ModeToggle
          icon={<Plus className="size-3" />}
          label="Switch to key/value rows"
          onClick={() => {
            // Only safe to switch back when the current value is a flat map.
            if (asFlatMap(value) !== null || value === undefined) setJsonMode(false);
          }}
          disabled={asFlatMap(value) === null && value !== undefined}
          disabledHint="Simplify the JSON to a flat list of values to switch back."
        />
      </div>
    );
  }

  const rows = Object.entries(flat ?? {});

  function commit(next: Record<string, string>) {
    onChange(Object.keys(next).length === 0 ? undefined : next);
  }
  function setKey(oldKey: string, newKey: string) {
    const next: Record<string, string> = {};
    for (const [k, v] of rows) next[k === oldKey ? newKey : k] = v;
    commit(next);
  }
  function setVal(key: string, val: string) {
    commit({ ...(flat ?? {}), [key]: val });
  }
  function remove(key: string) {
    const next = { ...(flat ?? {}) };
    delete next[key];
    commit(next);
  }
  function add() {
    let name = "";
    let n = 0;
    const current = flat ?? {};
    do {
      name = n === 0 ? "name" : `name${n}`;
      n += 1;
    } while (name in current);
    commit({ ...current, [name]: "" });
  }

  return (
    <div className="space-y-1.5">
      {rows.length === 0 ? (
        <p className="rounded-lg border border-dashed border-border px-3 py-2 text-xs text-muted-foreground">
          No values yet.
        </p>
      ) : (
        <div className="space-y-1.5">
          {rows.map(([key, val]) => (
            <div key={key} className="grid grid-cols-[9rem_1fr_auto] items-start gap-1.5">
              <input
                value={key}
                onChange={(e) => setKey(key, e.target.value)}
                placeholder={keyPlaceholder}
                aria-label="Name"
                className={`${controlClass} font-[family-name:var(--font-mono)] text-xs`}
              />
              <ValueField
                value={val}
                onChange={(v) => setVal(key, v)}
                placeholder={valuePlaceholder}
                references={references}
                allowSecret={allowSecret}
                ariaLabel="Value"
              />
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                onClick={() => remove(key)}
                aria-label="Remove"
              >
                <Trash2 className="size-3.5 text-muted-foreground" />
              </Button>
            </div>
          ))}
        </div>
      )}
      <div className="flex items-center gap-2">
        <Button type="button" variant="outline" size="sm" onClick={add}>
          <Plus data-icon="inline-start" className="size-3.5" />
          Add value
        </Button>
        <ModeToggle
          icon={<Code className="size-3" />}
          label="Edit as JSON"
          onClick={() => setJsonMode(true)}
        />
      </div>
    </div>
  );
}

function ModeToggle({
  icon,
  label,
  onClick,
  disabled,
  disabledHint,
}: {
  icon: React.ReactNode;
  label: string;
  onClick: () => void;
  disabled?: boolean;
  disabledHint?: string;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      title={disabled ? disabledHint : undefined}
      className="inline-flex items-center gap-1 text-xs text-muted-foreground transition-colors hover:text-foreground disabled:opacity-50"
    >
      {icon}
      {label}
    </button>
  );
}

function JsonMode({
  value,
  onChange,
}: {
  value: unknown;
  onChange: (value: unknown) => void;
}) {
  const [raw, setRaw] = useState(() =>
    value === undefined ? "" : JSON.stringify(value, null, 2),
  );
  const [error, setError] = useState("");
  useReportJsonValidity(useId(), error !== "");

  function handle(next: string) {
    setRaw(next);
    if (next.trim() === "") {
      setError("");
      onChange(undefined);
      return;
    }
    try {
      onChange(JSON.parse(next));
      setError("");
    } catch {
      setError("Invalid JSON");
    }
  }

  return (
    <div>
      <textarea
        value={raw}
        onChange={(e) => handle(e.target.value)}
        rows={3}
        spellCheck={false}
        placeholder="{ }"
        className={cn(monoControlClass, error && "border-destructive")}
      />
      {error && <p className="mt-1 text-xs text-destructive">{error}</p>}
    </div>
  );
}

/** Returns the value as a flat string map, or null if it isn't one. */
function asFlatMap(value: unknown): Record<string, string> | null {
  if (value === undefined || value === null) return {};
  if (typeof value !== "object" || Array.isArray(value)) return null;
  const out: Record<string, string> = {};
  for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
    if (typeof v === "string") out[k] = v;
    else if (typeof v === "number" || typeof v === "boolean") out[k] = String(v);
    else return null;
  }
  return out;
}
