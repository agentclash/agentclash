"use client";

import { updatePieceRef } from "../lib/draft";
import type { JudgeMethodMode, LLMJudgeDeclaration } from "../lib/types";
import { BuilderSelect } from "../ui/builder-select";
import { controlClass, EditorHeader, Field, FieldRow, monoControlClass } from "../ui/form";
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
    <div className="space-y-6">
      <EditorHeader
        title="LLM judge"
        description="An LLM-as-judge grader. Wire it into a scorecard dimension to make it count."
      />

      <FieldRow label="Key" hint="unique; referenced by scorecard dimensions">
        <input
          className={monoControlClass}
          value={def.key ?? ""}
          onChange={(e) => set({ key: e.target.value })}
          placeholder="empathy"
        />
      </FieldRow>
      <FieldRow label="Mode">
        <BuilderSelect
          ariaLabel="Mode"
          value={mode}
          onChange={(v) => set({ mode: v as JudgeMethodMode })}
          options={MODES}
        />
      </FieldRow>
      <FieldRow label="Model">
        <input
          className={monoControlClass}
          value={def.model ?? ""}
          onChange={(e) => set({ model: e.target.value })}
          placeholder="claude-haiku-4-5-20251001"
        />
      </FieldRow>
      <FieldRow label="Samples" hint="median of N">
        <input
          className={controlClass}
          type="number"
          min="1"
          max="10"
          value={def.samples ?? ""}
          onChange={(e) => set({ samples: e.target.value ? Number(e.target.value) : undefined })}
          placeholder="3"
        />
      </FieldRow>

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
          <FieldRow label="Scale">
            <div className="flex items-center gap-2">
              <input
                className={controlClass}
                type="number"
                aria-label="Scale min"
                value={scale.min}
                onChange={(e) => setScale({ min: Number(e.target.value) })}
              />
              <span className="text-builder-fg-faint">to</span>
              <input
                className={controlClass}
                type="number"
                aria-label="Scale max"
                value={scale.max}
                onChange={(e) => setScale({ max: Number(e.target.value) })}
              />
            </div>
          </FieldRow>
        </>
      )}

      {mode === "reference" && (
        <FieldRow
          label="Reference from"
          hint="evidence reference to the gold answer, e.g. case.expectations.reference_response"
        >
          <input
            className={monoControlClass}
            value={def.reference_from ?? ""}
            onChange={(e) => set({ reference_from: e.target.value })}
          />
        </FieldRow>
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
          <FieldRow label="Expectation">
            <label className="flex items-center gap-2 py-2 text-sm text-builder-fg-muted">
              <input
                type="checkbox"
                className="size-3.5 accent-builder-fg"
                checked={def.expect ?? true}
                onChange={(e) => set({ expect: e.target.checked })}
              />
              Assertion should be true
            </label>
          </FieldRow>
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
          <FieldRow label="Position debiasing">
            <label className="flex items-center gap-2 py-2 text-sm text-builder-fg-muted">
              <input
                type="checkbox"
                className="size-3.5 accent-builder-fg"
                checked={def.position_debiasing ?? false}
                onChange={(e) => set({ position_debiasing: e.target.checked })}
              />
              Randomize agent order across samples
            </label>
          </FieldRow>
        </>
      )}

      <FieldRow
        label="Evidence"
        hint="comma-separated references the judge sees, e.g. final_output, case.payload.order_id"
      >
        <input
          className={monoControlClass}
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
      </FieldRow>
    </div>
  );
}
