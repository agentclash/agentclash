"use client";

import { Field, controlClass } from "@/components/tools/field";
import { updatePieceRef } from "../lib/draft";
import type { ChallengeDefinition } from "../lib/types";
import { usePackDraft } from "../use-pack-draft";

const DIFFICULTIES = ["easy", "medium", "hard", "expert"];

export function ChallengeEditor({ index }: { index: number }) {
  const { state, update } = usePackDraft();
  const def = (state.composition.challenges?.[index]?.inline ?? {}) as ChallengeDefinition;

  const set = (patch: Partial<ChallengeDefinition>) =>
    update((c) => updatePieceRef(c, "challenge", index, { inline: { ...def, ...patch } }));

  return (
    <div className="max-w-2xl space-y-5">
      <h2 className="text-base font-semibold">Challenge</h2>
      <Field label="Key" hint="unique within the pack; cases target it by key">
        <input
          className={controlClass}
          value={def.key ?? ""}
          onChange={(e) => set({ key: e.target.value })}
          placeholder="refund-recovery"
        />
      </Field>
      <Field label="Title">
        <input
          className={controlClass}
          value={def.title ?? ""}
          onChange={(e) => set({ title: e.target.value })}
          placeholder="Recover a frustrated refund request"
        />
      </Field>
      <div className="grid grid-cols-2 gap-4">
        <Field label="Category">
          <input
            className={controlClass}
            value={def.category ?? ""}
            onChange={(e) => set({ category: e.target.value })}
            placeholder="support"
          />
        </Field>
        <Field label="Difficulty">
          <select
            className={controlClass}
            value={def.difficulty ?? "medium"}
            onChange={(e) => set({ difficulty: e.target.value })}
          >
            {DIFFICULTIES.map((d) => (
              <option key={d} value={d}>
                {d}
              </option>
            ))}
          </select>
        </Field>
      </div>
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
