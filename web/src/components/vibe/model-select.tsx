"use client";
import type { Model } from "@/lib/vibe";

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
      <p className="text-xs">
        <span className="mr-2 text-builder-fg-muted">{label}</span>
        {models[0].name || value}
      </p>
    );
  }
  return (
    <label className="inline-flex items-center gap-2 text-xs text-builder-fg-muted">
      <span>{label}</span>
      <select
        aria-label={`${label} model`}
        value={value}
        disabled={disabled}
        onChange={(e) => onChange(e.target.value)}
        className="max-w-48 rounded-md border border-builder-border bg-builder-panel px-2 py-1.5 text-builder-fg outline-none focus-visible:ring-2 focus-visible:ring-builder-border-strong"
      >
        {!models.some((m) => m.id === value) && (
          <option value={value}>{value.split("/").pop()}</option>
        )}
        {models.map((m) => (
          <option key={m.id} value={m.id}>
            {m.name || m.id.split("/").pop()}
          </option>
        ))}
      </select>
    </label>
  );
}
