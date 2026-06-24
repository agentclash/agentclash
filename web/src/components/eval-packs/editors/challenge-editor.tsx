"use client";

import { updatePieceRef } from "../lib/draft";
import type { ChallengeDefinition } from "../lib/types";
import { BuilderSelect } from "../ui/builder-select";
import { controlClass, EditorHeader, Field, FieldRow, monoControlClass } from "../ui/form";
import { usePackDraft } from "../use-pack-draft";

const DIFFICULTIES = ["easy", "medium", "hard", "expert"];

export function ChallengeEditor({ index }: { index: number }) {
  const { state, update } = usePackDraft();
  const def = (state.composition.challenges?.[index]?.inline ?? {}) as ChallengeDefinition;

  const set = (patch: Partial<ChallengeDefinition>) =>
    update((c) => updatePieceRef(c, "challenge", index, { inline: { ...def, ...patch } }));

  return (
    <div className="space-y-6">
      <EditorHeader title="Challenge" description="The task an agent is asked to do." />

      <FieldRow label="Key" hint="unique within the pack; cases target it by key">
        <input
          className={monoControlClass}
          value={def.key ?? ""}
          onChange={(e) => set({ key: e.target.value })}
          placeholder="refund-recovery"
        />
      </FieldRow>
      <FieldRow label="Title">
        <input
          className={controlClass}
          value={def.title ?? ""}
          onChange={(e) => set({ title: e.target.value })}
          placeholder="Recover a frustrated refund request"
        />
      </FieldRow>
      <FieldRow label="Category">
        <input
          className={controlClass}
          value={def.category ?? ""}
          onChange={(e) => set({ category: e.target.value })}
          placeholder="support"
        />
      </FieldRow>
      <FieldRow label="Difficulty">
        <BuilderSelect
          ariaLabel="Difficulty"
          value={def.difficulty ?? "medium"}
          onChange={(v) => set({ difficulty: v })}
          options={DIFFICULTIES.map((d) => ({ value: d, label: d }))}
        />
      </FieldRow>
      <Field
        label="Instructions"
        hint="The prompt the agent sees. Supports {{placeholder}} from case payloads."
      >
        <textarea
          className={controlClass}
          rows={8}
          value={def.instructions ?? ""}
          onChange={(e) => set({ instructions: e.target.value })}
          placeholder="You are a support agent helping a customer refund order {{order_id}}..."
        />
      </Field>
    </div>
  );
}
