"use client";

import { useId, useState } from "react";
import { cn } from "@/lib/utils";
import { monoControlClass } from "./field";
import { useReportJsonValidity } from "./json-validity";

const serialize = (value: unknown) => (value === undefined ? "" : JSON.stringify(value));

/**
 * Edits an arbitrary JSON value with live parse + error. Reports `undefined`
 * when emptied. Resyncs the textarea if the parent replaces `value` out from
 * under it (e.g. switching operations), but leaves in-progress invalid text
 * alone so the user never loses what they're typing.
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
  // Serialized form of the value this textarea is in sync with. Updated by our
  // own edits below; if it diverges from the incoming prop, the parent replaced
  // the value out from under us and we resync (the React-endorsed pattern of
  // adjusting state during render rather than in an effect).
  const [synced, setSynced] = useState(() => serialize(value));
  useReportJsonValidity(useId(), error !== "");

  const incoming = serialize(value);
  if (incoming !== synced) {
    setSynced(incoming);
    setRaw(value === undefined ? "" : JSON.stringify(value, null, 2));
    setError("");
  }

  function handle(next: string) {
    setRaw(next);
    if (next.trim() === "") {
      setError("");
      setSynced("");
      onChange(undefined);
      return;
    }
    try {
      const parsed = JSON.parse(next);
      setError("");
      setSynced(serialize(parsed));
      onChange(parsed);
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
