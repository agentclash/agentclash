"use client";

import { useId, useState } from "react";
import { cn } from "@/lib/utils";
import { monoControlClass } from "./field";
import { useReportJsonValidity } from "./json-validity";

/**
 * Edits an arbitrary JSON value with live parse + error. Reports `undefined`
 * when emptied. Initial value is captured on mount, so render it only once the
 * parent's value is ready (e.g. after an edit fetch resolves).
 */
export function JsonValueField({
  label,
  value,
  onChange,
  rows = 4,
  hint,
  placeholder = "{ }",
}: {
  label?: string;
  value: unknown;
  onChange: (value: unknown) => void;
  rows?: number;
  hint?: string;
  placeholder?: string;
}) {
  const [raw, setRaw] = useState<string>(() =>
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
      {label && <label className="mb-1.5 block text-sm font-medium">{label}</label>}
      {hint && <p className="mb-1.5 text-xs text-muted-foreground">{hint}</p>}
      <textarea
        value={raw}
        onChange={(e) => handle(e.target.value)}
        rows={rows}
        spellCheck={false}
        placeholder={placeholder}
        className={cn(monoControlClass, error && "border-destructive")}
      />
      {error && <p className="mt-1 text-xs text-destructive">{error}</p>}
    </div>
  );
}
