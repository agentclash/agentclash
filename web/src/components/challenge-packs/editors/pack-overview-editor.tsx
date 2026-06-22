"use client";

import { Field, controlClass } from "@/components/tools/field";
import { usePackDraft } from "../use-pack-draft";
import type { ExecutionMode } from "../lib/types";

const EXECUTION_MODES: { value: ExecutionMode; label: string }[] = [
  { value: "native", label: "Native — single-turn tool loop" },
  { value: "multi_turn", label: "Multi-turn — scripted/LLM/human conversation" },
  { value: "prompt_eval", label: "Prompt eval — no tools, instruction following" },
  { value: "responses", label: "Responses — structured output" },
];

export function PackOverviewEditor() {
  const { state, update } = usePackDraft();
  const { pack, version } = state.composition;

  return (
    <div className="max-w-2xl space-y-5">
      <div>
        <h2 className="text-base font-semibold">Pack overview</h2>
        <p className="text-sm text-muted-foreground">Name, identity, and how the pack runs.</p>
      </div>
      <Field label="Name">
        <input
          className={controlClass}
          value={pack.name}
          onChange={(e) => update((c) => ({ ...c, pack: { ...c.pack, name: e.target.value } }))}
          placeholder="Refund recovery"
        />
      </Field>
      <Field label="Slug" hint="kebab-case identifier, unique in your workspace">
        <input
          className={controlClass}
          value={pack.slug}
          onChange={(e) => update((c) => ({ ...c, pack: { ...c.pack, slug: e.target.value } }))}
          placeholder="refund-recovery"
        />
      </Field>
      <Field label="Family" hint="Category, e.g. support, security, sre">
        <input
          className={controlClass}
          value={pack.family}
          onChange={(e) => update((c) => ({ ...c, pack: { ...c.pack, family: e.target.value } }))}
          placeholder="support"
        />
      </Field>
      <Field label="Description">
        <textarea
          className={controlClass}
          rows={3}
          value={pack.description ?? ""}
          onChange={(e) => update((c) => ({ ...c, pack: { ...c.pack, description: e.target.value } }))}
          placeholder="What this pack evaluates."
        />
      </Field>
      <Field label="Execution mode">
        <select
          className={controlClass}
          value={version.execution_mode ?? "native"}
          onChange={(e) =>
            update((c) => ({
              ...c,
              version: { ...c.version, execution_mode: e.target.value as ExecutionMode },
            }))
          }
        >
          {EXECUTION_MODES.map((m) => (
            <option key={m.value} value={m.value}>
              {m.label}
            </option>
          ))}
        </select>
      </Field>
    </div>
  );
}
