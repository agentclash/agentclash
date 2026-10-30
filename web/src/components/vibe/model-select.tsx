"use client";
import type { Model } from "@/lib/vibe";

import { friendlyModelName } from "./model-name";

export function ModelSelect({
  label,
  value,
  models,
  onChange,
  disabled,
}: {
  label: string;
  value: string;
  models: Model[];
  onChange: (value: string) => void;
  disabled?: boolean;
}) {
  if (models.length === 1 && models[0].id === value) {
    return (
      <p className="text-sm">
        <span className="mr-2 text-builder-fg-muted">{label}</span>
        <span className="vibe-num" title={value}>{friendlyModelName(models[0], true)}</span>
      </p>
    );
  }
  return (
    <label className="inline-flex items-center gap-2 text-builder-fg-muted">
      <span>{label}</span>
      <select
        aria-label={`${label} model`}
        value={value}
        title={value}
        disabled={disabled || !models.length}
        onChange={(e) => onChange(e.target.value)}
        className="max-w-48 rounded-[var(--vibe-radius-sm)] border border-builder-border bg-builder-panel px-2 py-2 text-builder-fg outline-none focus-visible:ring-2 focus-visible:ring-builder-border-strong"
      >
        {!models.some((m) => m.id === value) && (
          <option value={value} disabled>{models.length ? `${friendlyModelName({ id: value }, true)} (unavailable)` : "No approved models available"}</option>
        )}
        {models.map((m) => (
          <option key={m.id} value={m.id} title={m.id}>
            {friendlyModelName(m, true)}
          </option>
        ))}
      </select>
    </label>
  );
}
