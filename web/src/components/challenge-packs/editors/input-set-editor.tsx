"use client";

import { Plus, Trash2 } from "lucide-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { pieceKeys, updatePieceRef } from "../lib/draft";
import type { CaseDefinition, InputSetDefinition } from "../lib/types";
import { BuilderSelect } from "../ui/builder-select";
import { controlClass, EditorHeader, Field, FieldRow, monoControlClass } from "../ui/form";
import { usePackDraft } from "../use-pack-draft";
import { CaseFlowEditor } from "./flow-editor";

export function InputSetEditor({ index }: { index: number }) {
  const { state, update } = usePackDraft();
  const def = (state.composition.input_sets?.[index]?.inline ?? {
    key: "",
    name: "",
    cases: [],
  }) as InputSetDefinition;
  const challengeKeys = pieceKeys(state.composition, "challenge");
  const cases = def.cases ?? [];
  const executionMode = state.composition.version.execution_mode ?? "native";

  const set = (patch: Partial<InputSetDefinition>) =>
    update((c) => updatePieceRef(c, "input_set", index, { inline: { ...def, ...patch } }));
  const setCase = (i: number, patch: Partial<CaseDefinition>) =>
    set({ cases: cases.map((cs, j) => (j === i ? { ...cs, ...patch } : cs)) });
  const addCase = () =>
    set({
      cases: [
        ...cases,
        { challenge_key: challengeKeys[0] ?? "", case_key: `case-${cases.length + 1}`, payload: {} },
      ],
    });
  const removeCase = (i: number) => set({ cases: cases.filter((_, j) => j !== i) });

  return (
    <div className="space-y-6">
      <EditorHeader
        title="Input set"
        description="The cases (test data) a challenge runs against."
      />

      <FieldRow label="Key">
        <input
          className={monoControlClass}
          value={def.key ?? ""}
          onChange={(e) => set({ key: e.target.value })}
          placeholder="default"
        />
      </FieldRow>
      <FieldRow label="Name">
        <input
          className={controlClass}
          value={def.name ?? ""}
          onChange={(e) => set({ name: e.target.value })}
          placeholder="Default"
        />
      </FieldRow>

      <div>
        <div className="mb-2 flex items-center justify-between">
          <span className="text-xs font-medium uppercase tracking-wide text-builder-fg-subtle">
            Cases {cases.length > 0 && <span className="tabular-nums">({cases.length})</span>}
          </span>
          <Button size="xs" variant="outline" onClick={addCase} disabled={challengeKeys.length === 0}>
            <Plus className="size-3.5" /> Add case
          </Button>
        </div>
        {challengeKeys.length === 0 && (
          <p className="text-xs text-builder-fg-faint">
            Add a challenge first — each case targets a challenge by key.
          </p>
        )}
        <div className="space-y-3">
          {cases.map((cs, i) => (
            <div key={i} className="space-y-3 rounded-md border border-builder-border bg-builder-surface p-3">
              <div className="flex items-center gap-2">
                <BuilderSelect
                  ariaLabel="Challenge"
                  placeholder="challenge"
                  value={cs.challenge_key ?? ""}
                  onChange={(v) => setCase(i, { challenge_key: v })}
                  options={challengeKeys.map((k) => ({ value: k, label: k }))}
                />
                <input
                  className={monoControlClass}
                  value={cs.case_key ?? ""}
                  onChange={(e) => setCase(i, { case_key: e.target.value })}
                  placeholder="case key"
                />
                <Button
                  size="icon-sm"
                  variant="ghost"
                  onClick={() => removeCase(i)}
                  aria-label="Remove case"
                >
                  <Trash2 className="size-4" />
                </Button>
              </div>
              <PayloadField value={cs.payload} onChange={(payload) => setCase(i, { payload })} />
              {executionMode === "multi_turn" && (
                <CaseFlowEditor
                  value={cs.user_simulator}
                  onChange={(user_simulator) => setCase(i, { user_simulator })}
                />
              )}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

function PayloadField({
  value,
  onChange,
}: {
  value?: Record<string, unknown>;
  onChange: (value: Record<string, unknown>) => void;
}) {
  const [raw, setRaw] = useState(() => JSON.stringify(value ?? {}, null, 2));
  const [error, setError] = useState("");

  return (
    <Field
      label="Payload"
      hint="JSON available to {{placeholders}} in the challenge instructions"
      error={error}
    >
      <textarea
        className={monoControlClass}
        rows={4}
        value={raw}
        onChange={(e) => {
          const next = e.target.value;
          setRaw(next);
          try {
            const parsed = next.trim() ? JSON.parse(next) : {};
            if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
              setError("");
              onChange(parsed as Record<string, unknown>);
            } else {
              setError("Payload must be a JSON object");
            }
          } catch {
            setError("Invalid JSON");
          }
        }}
      />
    </Field>
  );
}
