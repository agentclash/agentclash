"use client";
import type { Model } from "@/lib/vibe";

// Prefer the display name. A name that only repeats the ID drops the provider prefix unless that
// would make two models look the same. The full ID stays in the title.
const shortID = (id: string) => id.split("/").pop() || id;
function modelName(model: Model, models: Model[]) {
  if (model.name && model.name !== model.id) return model.name;
  return models.filter(item => shortID(item.id) === shortID(model.id)).length > 1 ? model.id : shortID(model.id);
}

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
        <span className="vibe-num" title={value}>{modelName(models[0], models)}</span>
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
          <option value={value} disabled>{models.length ? `${value.split("/").pop()} (unavailable)` : "No approved models available"}</option>
        )}
        {models.map((m) => (
          <option key={m.id} value={m.id} title={m.id}>
            {modelName(m, models)}
          </option>
        ))}
      </select>
    </label>
  );
}
