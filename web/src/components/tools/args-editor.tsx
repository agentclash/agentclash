"use client";

import { useId, useState } from "react";
import { cn } from "@/lib/utils";
import { controlClass, monoControlClass } from "./field";
import { useReportJsonValidity } from "./json-validity";
import type { ToolPrimitive } from "./lib/types";

/**
 * Edits the arguments passed to a delegated base primitive. Fields are derived
 * from the primitive's own JSON schema; object-typed args (e.g. HTTP headers)
 * get a JSON editor. A placeholder palette inserts ${param} / ${secrets.X}.
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
  const [focusedKey, setFocusedKey] = useState<string | null>(null);
  const [rawByKey, setRawByKey] = useState<Record<string, string>>({});
  const [errByKey, setErrByKey] = useState<Record<string, string>>({});
  useReportJsonValidity(useId(), Object.values(errByKey).some(Boolean));

  if (!primitive) {
    return (
      <p className="rounded-lg border border-dashed border-border p-4 text-center text-xs text-muted-foreground">
        Choose a primitive to configure its arguments.
      </p>
    );
  }

  const props = primitive.parameters?.properties ?? {};
  const required = new Set(primitive.parameters?.required ?? []);
  const entries = Object.entries(props);

  function setScalar(key: string, value: string) {
    const next = { ...args };
    if (value === "") delete next[key];
    else next[key] = value;
    onChange(next);
  }

  function setObject(key: string, raw: string) {
    setRawByKey((p) => ({ ...p, [key]: raw }));
    if (raw.trim() === "") {
      setErrByKey((p) => ({ ...p, [key]: "" }));
      const next = { ...args };
      delete next[key];
      onChange(next);
      return;
    }
    try {
      const parsed = JSON.parse(raw);
      setErrByKey((p) => ({ ...p, [key]: "" }));
      onChange({ ...args, [key]: parsed });
    } catch {
      setErrByKey((p) => ({ ...p, [key]: "Invalid JSON" }));
    }
  }

  function insertPlaceholder(token: string) {
    if (!focusedKey) return;
    const current = typeof args[focusedKey] === "string" ? (args[focusedKey] as string) : "";
    setScalar(focusedKey, current + token);
  }

  const chips = [
    ...paramNames.map((p) => `\${${p}}`),
    "${parameters}",
    ...(allowSecrets ? ["${secrets.NAME}"] : []),
  ];

  return (
    <div className="space-y-3">
      {entries.length === 0 ? (
        <p className="text-xs text-muted-foreground">
          This primitive takes no arguments.
        </p>
      ) : (
        entries.map(([key, prop]) => {
          const isObject = prop.type === "object" || prop.type === "array";
          const raw =
            rawByKey[key] ??
            (args[key] !== undefined ? JSON.stringify(args[key], null, 2) : "");
          return (
            <div key={key}>
              <label className="mb-1 flex items-center gap-2 text-sm font-medium">
                <code className="font-[family-name:var(--font-mono)] text-xs">{key}</code>
                <span className="text-xs font-normal text-muted-foreground">{prop.type}</span>
                {required.has(key) && (
                  <span className="text-xs font-normal text-muted-foreground">· required</span>
                )}
              </label>
              {isObject ? (
                <textarea
                  value={raw}
                  onChange={(e) => setObject(key, e.target.value)}
                  rows={3}
                  spellCheck={false}
                  placeholder="{ }"
                  className={cn(monoControlClass, errByKey[key] && "border-destructive")}
                />
              ) : (
                <input
                  value={typeof args[key] === "string" ? (args[key] as string) : ""}
                  onChange={(e) => setScalar(key, e.target.value)}
                  onFocus={() => setFocusedKey(key)}
                  placeholder={prop.description ?? "value or ${param}"}
                  className={`${controlClass} font-[family-name:var(--font-mono)] text-xs`}
                />
              )}
              {errByKey[key] && <p className="mt-1 text-xs text-destructive">{errByKey[key]}</p>}
            </div>
          );
        })
      )}

      {chips.length > 0 && entries.some(([, p]) => p.type !== "object" && p.type !== "array") && (
        <div className="flex flex-wrap items-center gap-1.5 rounded-lg border border-border bg-muted/30 p-2">
          <span className="text-xs text-muted-foreground">Insert:</span>
          {chips.map((c) => (
            <button
              key={c}
              type="button"
              onClick={() => insertPlaceholder(c)}
              disabled={!focusedKey}
              className="rounded-md border border-border bg-background px-1.5 py-0.5 font-[family-name:var(--font-mono)] text-xs text-muted-foreground transition-colors hover:text-foreground disabled:opacity-50"
            >
              {c}
            </button>
          ))}
          <span className="text-xs text-muted-foreground">
            {focusedKey ? `→ ${focusedKey}` : "focus a field first"}
          </span>
        </div>
      )}
    </div>
  );
}
