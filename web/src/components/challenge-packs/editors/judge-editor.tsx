"use client";

import { Field, controlClass } from "@/components/tools/field";
import { updatePieceRef } from "../lib/draft";
import type { JudgeMethodMode, LLMJudgeDeclaration } from "../lib/types";
import { usePackDraft } from "../use-pack-draft";

const MODES: { value: JudgeMethodMode; label: string }[] = [
  { value: "rubric", label: "Rubric — score against instructions" },
  { value: "assertion", label: "Assertion — yes/no claim" },
  { value: "reference", label: "Reference — score vs a gold answer" },
  { value: "n_wise", label: "N-wise — rank all agents together" },
];

export function JudgeEditor({ index }: { index: number }) {
  const { state, update } = usePackDraft();
  const def = (state.composition.judges?.[index]?.inline ?? {}) as LLMJudgeDeclaration;

  const set = (patch: Partial<LLMJudgeDeclaration>) =>
    update((c) => updatePieceRef(c, "judge", index, { inline: { ...def, ...patch } }));

  const mode = def.mode ?? "rubric";
  const scale = def.score_scale ?? { min: 1, max: 5 };
  const setScale = (patch: Partial<{ min: number; max: number }>) =>
    set({ score_scale: { ...scale, ...patch } });

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
      <div className="grid grid-cols-3 gap-4">
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
        <Field label="Samples" hint="median of N">
          <input
            className={controlClass}
            type="number"
            min="1"
            max="10"
            value={def.samples ?? ""}
            onChange={(e) => set({ samples: e.target.value ? Number(e.target.value) : undefined })}
            placeholder="3"
          />
        </Field>
      </div>

      {(mode === "rubric" || mode === "reference") && (
        <>
          <Field label="Rubric" hint="how to score">
            <textarea
              className={controlClass}
              rows={6}
              value={def.rubric ?? ""}
              onChange={(e) => set({ rubric: e.target.value })}
              placeholder="Score 5 if the response is empathetic and gives a concrete next step..."
            />
          </Field>
          <div className="grid grid-cols-2 gap-4">
            <Field label="Scale min">
              <input
                className={controlClass}
                type="number"
                value={scale.min}
                onChange={(e) => setScale({ min: Number(e.target.value) })}
              />
            </Field>
            <Field label="Scale max">
              <input
                className={controlClass}
                type="number"
                value={scale.max}
                onChange={(e) => setScale({ max: Number(e.target.value) })}
              />
            </Field>
          </div>
        </>
      )}

      {mode === "reference" && (
        <Field
          label="Reference from"
          hint="evidence reference to the gold answer, e.g. case.expectations.reference_response"
        >
          <input
            className={controlClass}
            value={def.reference_from ?? ""}
            onChange={(e) => set({ reference_from: e.target.value })}
          />
        </Field>
      )}

      {mode === "assertion" && (
        <>
          <Field label="Assertion" hint="a yes/no claim about the output">
            <textarea
              className={controlClass}
              rows={4}
              value={def.assertion ?? ""}
              onChange={(e) => set({ assertion: e.target.value })}
              placeholder="The assistant gives a concrete refund timeline instead of deflecting."
            />
          </Field>
          <Field label="Expectation">
            <label className="flex h-9 items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={def.expect ?? true}
                onChange={(e) => set({ expect: e.target.checked })}
              />
              Assertion should be true
            </label>
          </Field>
        </>
      )}

      {mode === "n_wise" && (
        <>
          <Field label="Ranking prompt">
            <textarea
              className={controlClass}
              rows={5}
              value={def.prompt ?? ""}
              onChange={(e) => set({ prompt: e.target.value })}
              placeholder="Rank the responses from best to worst by how well they resolve the issue..."
            />
          </Field>
          <Field label="Position debiasing">
            <label className="flex h-9 items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={def.position_debiasing ?? false}
                onChange={(e) => set({ position_debiasing: e.target.checked })}
              />
              Randomize agent order across samples
            </label>
          </Field>
        </>
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
