"use client";

import type { ExecutionMode } from "../lib/types";
import { BuilderSelect } from "../ui/builder-select";
import { controlClass, EditorHeader, Field, FieldRow, monoControlClass } from "../ui/form";
import { usePackDraft } from "../use-pack-draft";

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
    <div className="space-y-6">
      <EditorHeader title="Pack overview" description="Name, identity, and how the pack runs." />

      <FieldRow label="Name">
        <input
          className={controlClass}
          value={pack.name}
          onChange={(e) => update((c) => ({ ...c, pack: { ...c.pack, name: e.target.value } }))}
          placeholder="Refund recovery"
        />
      </FieldRow>
      <FieldRow label="Slug" hint="kebab-case identifier, unique in your workspace">
        <input
          className={monoControlClass}
          value={pack.slug}
          onChange={(e) => update((c) => ({ ...c, pack: { ...c.pack, slug: e.target.value } }))}
          placeholder="refund-recovery"
        />
      </FieldRow>
      <FieldRow label="Family" hint="Category, e.g. support, security, sre">
        <input
          className={controlClass}
          value={pack.family}
          onChange={(e) => update((c) => ({ ...c, pack: { ...c.pack, family: e.target.value } }))}
          placeholder="support"
        />
      </FieldRow>
      <Field label="Description">
        <textarea
          className={controlClass}
          rows={3}
          value={pack.description ?? ""}
          onChange={(e) =>
            update((c) => ({ ...c, pack: { ...c.pack, description: e.target.value } }))
          }
          placeholder="What this pack evaluates."
        />
      </Field>
      <FieldRow label="Execution mode">
        <BuilderSelect
          ariaLabel="Execution mode"
          value={version.execution_mode ?? "native"}
          onChange={(v) =>
            update((c) => ({ ...c, version: { ...c.version, execution_mode: v as ExecutionMode } }))
          }
          options={EXECUTION_MODES}
        />
      </FieldRow>
    </div>
  );
}
