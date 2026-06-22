"use client";

import { Field, controlClass } from "@/components/tools/field";
import { updatePieceRef } from "../lib/draft";
import type { JudgeMethodMode, LLMJudgeDeclaration } from "../lib/types";
import { usePackDraft } from "../use-pack-draft";

const MODES: { value: JudgeMethodMode; label: string }[] = [
  { value: "rubric", label: "Rubric — score 1–5 against instructions" },
  { value: "assertion", label: "Assertion — yes/no claim about the output" },
];

export function JudgeEditor({ index }: { index: number }) {
  const { state, update } = usePackDraft();
  const def = (state.composition.judges?.[index]?.inline ?? {}) as LLMJudgeDeclaration;

  const set = (patch: Partial<LLMJudgeDeclaration>) =>
    update((c) => updatePieceRef(c, "judge", index, { inline: { ...def, ...patch } }));

  const mode = def.mode ?? "rubric";

  return (
    <div className="max-w-2xl space-y-5">
      <h2 className="text-base font-semibold">LLM judge</h2>
      <p className="text-sm text-muted-foreground">
        An LLM-as-judge grader. Wire it into a scorecard dimension to make it count.
      </p>
      <Field label="Key" hint="unique; referenced by scorecard dimensions">
        <input
          className={controlClass}
          value={def.key ?? ""}
          onChange={(e) => set({ key: e.target.value })}
          placeholder="empathy"
        />
      </Field>
      <div className="grid grid-cols-2 gap-4">
        <Field label="Mode">
          <select
            className={controlClass}
            value={mode}
            onChange={(e) => set({ mode: e.target.value as JudgeMethodMode })}
          >
            {MODES.map((m) => (
              <option key={m.value} value={m.value}>
                {m.label}
              </option>
            ))}
          </select>
        </Field>
        <Field label="Model">
          <input
            className={controlClass}
            value={def.model ?? ""}
            onChange={(e) => set({ model: e.target.value })}
            placeholder="claude-haiku-4-5-20251001"
          />
        </Field>
      </div>
      {mode === "rubric" ? (
        <Field label="Rubric" hint="how to score 1–5">
          <textarea
            className={controlClass}
            rows={6}
            value={def.rubric ?? ""}
            onChange={(e) => set({ rubric: e.target.value })}
            placeholder="Score 5 if the response is empathetic and gives a concrete next step..."
          />
        </Field>
      ) : (
        <Field label="Assertion" hint="a yes/no claim the output should satisfy">
          <textarea
            className={controlClass}
            rows={4}
            value={def.assertion ?? ""}
            onChange={(e) => set({ assertion: e.target.value })}
            placeholder="The assistant gives a concrete refund timeline instead of deflecting."
          />
        </Field>
      )}
      <Field
        label="Evidence"
        hint="comma-separated references the judge sees, e.g. final_output, case.payload.order_id"
      >
        <input
          className={controlClass}
          value={(def.context_from ?? []).join(", ")}
          onChange={(e) =>
            set({
              context_from: e.target.value
                .split(",")
                .map((s) => s.trim())
                .filter(Boolean),
            })
          }
          placeholder="final_output"
        />
      </Field>
    </div>
  );
}
