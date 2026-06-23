"use client";

import { useState } from "react";

import { Field, controlClass, monoControlClass } from "@/components/tools/field";
import { updatePieceRef } from "../lib/draft";
import type { ValidatorDeclaration, ValidatorType } from "../lib/types";
import { usePackDraft } from "../use-pack-draft";
import {
  type ConfigField,
  VALIDATOR_GROUPS,
  VALIDATOR_TYPES,
  defaultTargetForType,
  validatorMeta,
} from "./validator-types";

const EVIDENCE_TARGETS = ["final_output", "transcript.full", "transcript.from_mismatch"];

export function ValidatorEditor({ index }: { index: number }) {
  const { state, update } = usePackDraft();
  const def = (state.composition.validators?.[index]?.inline ?? {}) as ValidatorDeclaration;
  const meta = validatorMeta(def.type);
  const config = (def.config ?? {}) as Record<string, unknown>;

  const set = (patch: Partial<ValidatorDeclaration>) =>
    update((c) => updatePieceRef(c, "validator", index, { inline: { ...def, ...patch } }));

  const changeType = (type: ValidatorType) => {
    const next = validatorMeta(type);
    set({
      type,
      target: defaultTargetForType(next),
      config: {},
      expected_from: next.needsExpected ? (def.expected_from ?? "") : undefined,
    });
  };

  const setConfig = (key: string, value: unknown) => set({ config: { ...config, [key]: value } });
  const filePath = (def.target ?? "").startsWith("file:") ? (def.target ?? "").slice(5) : "";

  return (
    <div className="max-w-2xl space-y-5">
      <h2 className="text-base font-semibold">Validator</h2>
      <p className="text-sm text-muted-foreground">
        A deterministic check. Wire it into a scorecard dimension to make it count.
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
          onChange={(e) => changeType(e.target.value as ValidatorType)}
        >
          {VALIDATOR_GROUPS.map((group) => (
            <optgroup key={group} label={group}>
              {VALIDATOR_TYPES.filter((t) => t.group === group).map((t) => (
                <option key={t.value} value={t.value}>
                  {t.label}
                </option>
              ))}
            </optgroup>
          ))}
        </select>
      </Field>

      {meta.forcesToolCalls ? (
        <Field label="Target" hint="tool-call validators inspect the agent's tool calls">
          <input className={controlClass} value="tool_calls" disabled />
        </Field>
      ) : meta.isFile ? (
        <Field label="File path" hint="sandbox path to check">
          <input
            className={monoControlClass}
            value={filePath}
            onChange={(e) => set({ target: `file:${e.target.value}` })}
            placeholder="/workspace/out.json"
          />
        </Field>
      ) : (
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
      )}

      {meta.needsExpected && (
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
      )}

      {meta.config?.map((field) => (
        <ConfigFieldInput
          key={field.key}
          field={field}
          value={config[field.key]}
          onChange={(v) => setConfig(field.key, v)}
        />
      ))}
    </div>
  );
}

function ConfigFieldInput({
  field,
  value,
  onChange,
}: {
  field: ConfigField;
  value: unknown;
  onChange: (value: unknown) => void;
}) {
  if (field.type === "boolean") {
    return (
      <Field label={field.label} hint={field.hint}>
        <label className="flex h-9 items-center gap-2 text-sm">
          <input type="checkbox" checked={Boolean(value)} onChange={(e) => onChange(e.target.checked)} /> Enabled
        </label>
      </Field>
    );
  }
  if (field.type === "number") {
    return (
      <Field label={field.label} hint={field.hint}>
        <input
          className={controlClass}
          type="number"
          step="any"
          value={typeof value === "number" ? value : ""}
          onChange={(e) => onChange(e.target.value === "" ? undefined : Number(e.target.value))}
          placeholder={field.placeholder}
        />
      </Field>
    );
  }
  if (field.type === "json") {
    return <JsonConfigField field={field} value={value} onChange={onChange} />;
  }
  return (
    <Field label={field.label} hint={field.hint}>
      <input
        className={controlClass}
        value={typeof value === "string" ? value : ""}
        onChange={(e) => onChange(e.target.value)}
        placeholder={field.placeholder}
      />
    </Field>
  );
}

function JsonConfigField({
  field,
  value,
  onChange,
}: {
  field: ConfigField;
  value: unknown;
  onChange: (value: unknown) => void;
}) {
  const [raw, setRaw] = useState(() => (value === undefined ? "" : JSON.stringify(value, null, 2)));
  const [error, setError] = useState("");
  return (
    <Field label={field.label} hint={field.hint} error={error}>
      <textarea
        className={monoControlClass}
        rows={4}
        value={raw}
        onChange={(e) => {
          const next = e.target.value;
          setRaw(next);
          if (!next.trim()) {
            setError("");
            onChange(undefined);
            return;
          }
          try {
            onChange(JSON.parse(next));
            setError("");
          } catch {
            setError("Invalid JSON");
          }
        }}
      />
    </Field>
  );
}
