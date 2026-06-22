"use client";

import { Plus, Trash2 } from "lucide-react";

import { Field, controlClass } from "@/components/tools/field";
import { Button } from "@/components/ui/button";
import { pieceKeys, setDimensions } from "../lib/draft";
import type { DimensionDeclaration, DimensionSource, ScoringStrategy } from "../lib/types";
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
    <div className="max-w-2xl space-y-5">
      <div>
        <h2 className="text-base font-semibold">Scoring</h2>
        <p className="text-sm text-muted-foreground">
          Wire your validators and judges into scoring dimensions.
        </p>
      </div>
      <div className="grid grid-cols-2 gap-4">
        <Field label="Strategy">
          <select
            className={controlClass}
            value={strategy}
            onChange={(e) => setScorecard({ strategy: e.target.value as ScoringStrategy })}
          >
            {STRATEGIES.map((s) => (
              <option key={s.value} value={s.value}>
                {s.label}
              </option>
            ))}
          </select>
        </Field>
        {strategy !== "binary" && (
          <Field label="Pass threshold" hint="overall score 0–1 to pass">
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
          </Field>
        )}
      </div>

      <div>
        <div className="mb-2 flex items-center justify-between">
          <span className="text-sm font-medium">Dimensions ({dims.length})</span>
          <Button size="xs" variant="outline" onClick={addDim}>
            <Plus className="size-3.5" /> Add dimension
          </Button>
        </div>
        <div className="space-y-3">
          {dims.map((dim, i) => (
            <div key={i} className="space-y-3 rounded-lg border border-border p-3">
              <div className="flex items-center gap-2">
                <input
                  className={controlClass}
                  value={dim.key}
                  onChange={(e) => setDim(i, { key: e.target.value })}
                  placeholder="correctness"
                />
                <select
                  className={controlClass}
                  value={dim.source}
                  onChange={(e) => setDim(i, { source: e.target.value as DimensionSource })}
                >
                  {SOURCES.map((s) => (
                    <option key={s.value} value={s.value}>
                      {s.label}
                    </option>
                  ))}
                </select>
                <Button size="icon-sm" variant="ghost" onClick={() => removeDim(i)} aria-label="Remove dimension">
                  <Trash2 className="size-4" />
                </Button>
              </div>

              {dim.source === "validators" ? (
                <Field label="Validators" hint="which validators feed this dimension">
                  {validatorKeys.length === 0 ? (
                    <p className="text-xs text-muted-foreground">Add a validator piece first.</p>
                  ) : (
                    <div className="flex flex-wrap gap-2">
                      {validatorKeys.map((key) => {
                        const checked = (dim.validators ?? []).includes(key);
                        return (
                          <label
                            key={key}
                            className="flex cursor-pointer items-center gap-1.5 rounded-md border border-border px-2 py-1 text-xs"
                          >
                            <input
                              type="checkbox"
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
                    <p className="text-xs text-muted-foreground">Add a judge piece first.</p>
                  ) : (
                    <select
                      className={controlClass}
                      value={dim.judge_key ?? ""}
                      onChange={(e) => setDim(i, { judge_key: e.target.value })}
                    >
                      <option value="">Select a judge…</option>
                      {judgeKeys.map((key) => (
                        <option key={key} value={key}>
                          {key}
                        </option>
                      ))}
                    </select>
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
                  <label className="flex h-9 items-center gap-2 text-sm">
                    <input
                      type="checkbox"
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
