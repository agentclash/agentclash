"use client";

import { Field, controlClass, monoControlClass } from "@/components/tools/field";
import { updatePieceRef } from "../lib/draft";
import type { ValidatorDeclaration, ValidatorType } from "../lib/types";
import { usePackDraft } from "../use-pack-draft";

// v1 surfaces the common, output-targeting validator types. The full 21-type
// registry (file/generation/tool-call/code) lands in a later phase.
const COMMON_TYPES: { value: ValidatorType; label: string }[] = [
  { value: "contains", label: "Output contains text" },
  { value: "exact_match", label: "Output exactly equals" },
  { value: "regex_match", label: "Output matches regex" },
  { value: "fuzzy_match", label: "Fuzzy match (similarity)" },
  { value: "numeric_match", label: "Numeric match" },
  { value: "json_schema", label: "Output matches JSON schema" },
];

const EVIDENCE_TARGETS = ["final_output", "transcript.full", "transcript.from_mismatch"];

export function ValidatorEditor({ index }: { index: number }) {
  const { state, update } = usePackDraft();
  const def = (state.composition.validators?.[index]?.inline ?? {}) as ValidatorDeclaration;

  const set = (patch: Partial<ValidatorDeclaration>) =>
    update((c) => updatePieceRef(c, "validator", index, { inline: { ...def, ...patch } }));

  return (
    <div className="max-w-2xl space-y-5">
      <h2 className="text-base font-semibold">Validator</h2>
      <p className="text-sm text-muted-foreground">
        A deterministic check on the agent&apos;s output. Wire it into a scorecard dimension to make
        it count toward the score.
      </p>
      <Field label="Key" hint="unique; referenced by scorecard dimensions">
        <input
          className={controlClass}
          value={def.key ?? ""}
          onChange={(e) => set({ key: e.target.value })}
          placeholder="mentions_refund"
        />
      </Field>
      <Field label="Check">
        <select
          className={controlClass}
          value={def.type ?? "contains"}
          onChange={(e) => set({ type: e.target.value as ValidatorType })}
        >
          {COMMON_TYPES.map((t) => (
            <option key={t.value} value={t.value}>
              {t.label}
            </option>
          ))}
        </select>
      </Field>
      <Field label="Target" hint="what gets checked">
        <select
          className={controlClass}
          value={def.target ?? "final_output"}
          onChange={(e) => set({ target: e.target.value })}
        >
          {EVIDENCE_TARGETS.map((t) => (
            <option key={t} value={t}>
              {t}
            </option>
          ))}
        </select>
      </Field>
      <Field
        label="Expected"
        hint={'a literal (prefix "literal:") or an evidence reference like case.expectations.answer'}
      >
        <input
          className={monoControlClass}
          value={def.expected_from ?? ""}
          onChange={(e) => set({ expected_from: e.target.value })}
          placeholder="literal:refund"
        />
      </Field>
    </div>
  );
}
