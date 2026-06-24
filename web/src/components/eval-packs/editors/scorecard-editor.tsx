"use client";

import { Plus, Trash2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { pieceKeys, setDimensions } from "../lib/draft";
import type { DimensionDeclaration, DimensionSource, ScoringStrategy } from "../lib/types";
import { BuilderSelect } from "../ui/builder-select";
import { controlClass, EditorHeader, Field, FieldRow, monoControlClass } from "../ui/form";
import { usePackDraft } from "../use-pack-draft";

const STRATEGIES: { value: ScoringStrategy; label: string }[] = [
  { value: "weighted", label: "Weighted — weighted average of dimensions" },
  { value: "binary", label: "Binary — pass only if every dimension passes" },
  { value: "hybrid", label: "Hybrid — gates must pass + weighted score" },
];

const SOURCES: { value: DimensionSource; label: string }[] = [
  { value: "validators", label: "Validators" },
  { value: "llm_judge", label: "LLM judge" },
];

function numberOrUndefined(value: string): number | undefined {
  if (value.trim() === "") return undefined;
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : undefined;
}

export function ScorecardEditor() {
  const { state, update } = usePackDraft();
  const scorecard = state.composition.scorecard;
  const dims = scorecard.dimensions ?? [];
  const validatorKeys = pieceKeys(state.composition, "validator");
  const judgeKeys = pieceKeys(state.composition, "judge");

  const setScorecard = (patch: Partial<typeof scorecard>) =>
    update((c) => ({ ...c, scorecard: { ...c.scorecard, ...patch } }));
  const setDim = (i: number, patch: Partial<DimensionDeclaration>) =>
    update((c) => setDimensions(c, dims.map((d, j) => (j === i ? { ...d, ...patch } : d))));
  const addDim = () =>
    update((c) =>
      setDimensions(c, [
        ...dims,
        { key: `dimension-${dims.length + 1}`, source: "validators", validators: [], weight: 1 },
      ]),
    );
  const removeDim = (i: number) => update((c) => setDimensions(c, dims.filter((_, j) => j !== i)));

  const strategy = scorecard.strategy ?? "weighted";

  return (
    <div className="space-y-6">
      <EditorHeader
        title="Scoring"
        description="Wire your validators and judges into scoring dimensions."
      />

      <FieldRow label="Strategy">
        <BuilderSelect
          ariaLabel="Strategy"
          value={strategy}
          onChange={(v) => setScorecard({ strategy: v as ScoringStrategy })}
          options={STRATEGIES}
        />
      </FieldRow>
      {strategy !== "binary" && (
        <FieldRow label="Pass threshold" hint="overall score 0–1 to pass">
          <input
            className={controlClass}
            type="number"
            step="0.05"
            min="0"
            max="1"
            value={scorecard.pass_threshold ?? ""}
            onChange={(e) => setScorecard({ pass_threshold: numberOrUndefined(e.target.value) })}
            placeholder="0.75"
          />
        </FieldRow>
      )}

      <div>
        <div className="mb-2 flex items-center justify-between">
          <span className="text-xs font-medium uppercase tracking-wide text-builder-fg-subtle">
            Dimensions {dims.length > 0 && <span className="tabular-nums">({dims.length})</span>}
          </span>
          <Button size="xs" variant="outline" onClick={addDim}>
            <Plus className="size-3.5" /> Add dimension
          </Button>
        </div>
        <div className="space-y-3">
          {dims.map((dim, i) => (
            <div key={i} className="space-y-3 rounded-md border border-builder-border bg-builder-surface p-3">
              <div className="flex items-center gap-2">
                <input
                  className={monoControlClass}
                  value={dim.key}
                  onChange={(e) => setDim(i, { key: e.target.value })}
                  placeholder="correctness"
                />
                <BuilderSelect
                  ariaLabel="Source"
                  className="w-44"
                  value={dim.source}
                  onChange={(v) => setDim(i, { source: v as DimensionSource })}
                  options={SOURCES}
                />
                <Button
                  size="icon-sm"
                  variant="ghost"
                  onClick={() => removeDim(i)}
                  aria-label="Remove dimension"
                >
                  <Trash2 className="size-4" />
                </Button>
              </div>

              {dim.source === "validators" ? (
                <Field label="Validators" hint="which validators feed this dimension">
                  {validatorKeys.length === 0 ? (
                    <p className="text-xs text-builder-fg-faint">Add a validator piece first.</p>
                  ) : (
                    <div className="flex flex-wrap gap-2">
                      {validatorKeys.map((key) => {
                        const checked = (dim.validators ?? []).includes(key);
                        return (
                          <label
                            key={key}
                            className="flex cursor-pointer items-center gap-1.5 rounded-md border border-builder-border px-2 py-1 font-[family-name:var(--font-mono)] text-xs text-builder-fg-muted transition-colors hover:border-builder-border-strong hover:text-builder-fg"
                          >
                            <input
                              type="checkbox"
                              className="size-3.5 accent-builder-fg"
                              checked={checked}
                              onChange={(e) =>
                                setDim(i, {
                                  validators: e.target.checked
                                    ? [...(dim.validators ?? []), key]
                                    : (dim.validators ?? []).filter((k) => k !== key),
                                })
                              }
                            />
                            {key}
                          </label>
                        );
                      })}
                    </div>
                  )}
                </Field>
              ) : (
                <Field label="Judge" hint="which judge feeds this dimension">
                  {judgeKeys.length === 0 ? (
                    <p className="text-xs text-builder-fg-faint">Add a judge piece first.</p>
                  ) : (
                    <BuilderSelect
                      ariaLabel="Judge"
                      placeholder="Select a judge…"
                      value={dim.judge_key ?? ""}
                      onChange={(v) => setDim(i, { judge_key: v })}
                      options={judgeKeys.map((key) => ({ value: key, label: key }))}
                    />
                  )}
                </Field>
              )}

              <div className="grid grid-cols-3 gap-3">
                <Field label="Weight">
                  <input
                    className={controlClass}
                    type="number"
                    step="0.1"
                    value={dim.weight ?? ""}
                    onChange={(e) => setDim(i, { weight: numberOrUndefined(e.target.value) })}
                    placeholder="1.0"
                  />
                </Field>
                <Field label="Gate threshold" hint="0–1, optional">
                  <input
                    className={controlClass}
                    type="number"
                    step="0.05"
                    min="0"
                    max="1"
                    value={dim.pass_threshold ?? ""}
                    onChange={(e) => setDim(i, { pass_threshold: numberOrUndefined(e.target.value) })}
                    placeholder="0.5"
                  />
                </Field>
                <Field label="Gate">
                  <label className="flex items-center gap-2 py-2 text-sm text-builder-fg-muted">
                    <input
                      type="checkbox"
                      className="size-3.5 accent-builder-fg"
                      checked={dim.gate ?? false}
                      onChange={(e) => setDim(i, { gate: e.target.checked })}
                    />
                    Must pass
                  </label>
                </Field>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
