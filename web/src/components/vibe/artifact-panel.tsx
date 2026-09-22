"use client";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import type { Artifact, Capability, Model, Requirement } from "@/lib/vibe";
import { Requirements } from "./requirements";

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
export function ArtifactPanel({
  artifact,
  capabilities = [],
  requirements,
  busy,
  onEdit,
  onDirtyChange,
  onRequirement,
}: {
  artifact: Artifact;
  capabilities?: Capability[];
  requirements: Requirement[];
  busy: boolean;
  onEdit: (prompt: string) => Promise<boolean>;
  onDirtyChange: (dirty: boolean) => void;
  onRequirement: (
    id: string,
    status: "accepted" | "rejected" | "superseded",
    statement?: string,
  ) => void;
}) {
  const [prompt, setPrompt] = useState(artifact.agent_prompt);
  const rules = capabilities.find((c) => c.id === "text_preview")?.instructions;
  const prefix = rules && prompt.startsWith(rules) ? rules : "";
  const dirty = prompt !== artifact.agent_prompt;
  return (
    <section
      aria-label="Agent instructions"
      className="space-y-4 rounded-xl border border-builder-border p-5"
    >
      <h2 className="font-medium">Edit instructions</h2>
      <p className="text-sm text-builder-fg-muted">
        Describe how your agent should behave. Applying changes creates a new
        version you can try.
      </p>
      <label className="block text-sm">
        Agent instructions
        <textarea
          aria-label="Agent instructions"
          value={prompt.slice(prefix.length)}
          disabled={busy}
          onChange={(e) => {
            const next = prefix + e.target.value;
            setPrompt(next);
            onDirtyChange(next !== artifact.agent_prompt);
          }}
          className="mt-2 min-h-56 w-full rounded-xl border border-builder-border bg-builder-surface p-3 text-sm leading-6"
        />
      </label>
      {dirty && (
        <div className="flex items-center gap-3">
          <Button
            size="sm"
            disabled={busy || !prompt.slice(prefix.length).trim()}
            onClick={async () => {
              if (await onEdit(prompt)) onDirtyChange(false);
            }}
          >
            Apply changes
          </Button>
          <Button
            size="sm"
            variant="ghost"
            disabled={busy}
            onClick={() => {
              setPrompt(artifact.agent_prompt);
              onDirtyChange(false);
            }}
          >
            Discard edits
          </Button>
        </div>
      )}
      <details className="text-sm text-builder-fg-muted">
        <summary className="cursor-pointer">
          Assumptions and confirmed requirements
        </summary>
        <p className="my-3">
          Trying or checking an agent does not confirm business facts. Confirm
          or correct proposed requirements here.
        </p>
        <Requirements
          requirements={requirements}
          busy={busy || dirty}
          onRequirement={onRequirement}
        />
      </details>
      {rules && (
        <details className="text-sm text-builder-fg-muted">
          <summary className="cursor-pointer">How this preview works</summary>
          <p className="mt-3 whitespace-pre-wrap">{rules.trim()}</p>
        </details>
      )}
    </section>
  );
}
