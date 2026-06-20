"use client";

import { useId, useState } from "react";
import { cn } from "@/lib/utils";
import { monoControlClass } from "./field";
import { useReportJsonValidity } from "./json-validity";
import { KeyValueEditor } from "./key-value-editor";
import { ValueField } from "./value-field";
import { primitiveReferences, typeLabel } from "./lib/friendly";
import type { JsonSchemaType, ToolPrimitive } from "./lib/types";

/**
 * Fills in what a base operation needs. Fields are derived from the operation's
 * own schema: plain values use a ValueField (type literal text or insert an agent
 * input), key/value groups use a KeyValueEditor, lists fall back to JSON. The user
 * never types `${...}` — references are inserted from a menu.
 */
export function ArgsEditor({
  primitive,
  args,
  onChange,
  paramNames,
  allowSecrets,
}: {
  primitive: ToolPrimitive | null;
  args: Record<string, unknown>;
  onChange: (args: Record<string, unknown>) => void;
  paramNames: string[];
  allowSecrets: boolean;
}) {
  if (!primitive) {
    return (
      <p className="rounded-lg border border-dashed border-border p-4 text-center text-xs text-muted-foreground">
        Choose an operation above to set up its details.
      </p>
    );
  }

  const props = primitive.parameters?.properties ?? {};
  const required = new Set(primitive.parameters?.required ?? []);
  const entries = Object.entries(props);
  const references = primitiveReferences(paramNames);

  function setArg(key: string, value: unknown) {
    const next = { ...args };
    if (value === undefined || value === "") delete next[key];
    else next[key] = value;
    onChange(next);
  }

  if (entries.length === 0) {
    return (
      <p className="text-xs text-muted-foreground">
        This operation doesn’t need any details.
      </p>
    );
  }

  return (
    <div className="space-y-4">
      {entries.map(([key, prop]) => (
        <div key={key}>
          <label className="mb-1 flex items-center gap-2 text-sm font-medium">
            {key}
            <span className="text-xs font-normal text-muted-foreground">
              {typeLabel(prop.type as JsonSchemaType)}
            </span>
            {required.has(key) && (
              <span className="text-xs font-normal text-muted-foreground">· required</span>
            )}
          </label>
          {prop.description && (
            <p className="mb-1.5 text-xs text-muted-foreground">{prop.description}</p>
          )}
          {prop.type === "object" ? (
            <KeyValueEditor
              value={args[key]}
              onChange={(v) => setArg(key, v)}
              references={references}
              allowSecret={allowSecrets}
            />
          ) : prop.type === "array" ? (
            <JsonArgField
              value={args[key]}
              onChange={(v) => setArg(key, v)}
              placeholder="[ ]"
            />
          ) : (
            <ValueField
              value={typeof args[key] === "string" ? (args[key] as string) : ""}
              onChange={(v) => setArg(key, v)}
              placeholder={prop.description ?? "Type a value or insert an input"}
              references={references}
              allowSecret={allowSecrets}
              ariaLabel={key}
            />
          )}
        </div>
      ))}
    </div>
  );
}

/** A small JSON editor for list/array arguments, with live parse + validity. */
function JsonArgField({
  value,
  onChange,
  placeholder,
}: {
  value: unknown;
  onChange: (value: unknown) => void;
  placeholder?: string;
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
        placeholder={placeholder}
        className={cn(monoControlClass, error && "border-destructive")}
      />
      {error && <p className="mt-1 text-xs text-destructive">{error}</p>}
    </div>
  );
}
