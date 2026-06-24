"use client";

import { useState } from "react";

import { updatePieceRef } from "../lib/draft";
import type { ValidatorDeclaration, ValidatorType } from "../lib/types";
import { BuilderSelect } from "../ui/builder-select";
import { controlClass, EditorHeader, Field, FieldRow, monoControlClass } from "../ui/form";
import { usePackDraft } from "../use-pack-draft";
import {
  type ConfigField,
  VALIDATOR_GROUPS,
  VALIDATOR_TYPES,
  defaultTargetForType,
  validatorMeta,
} from "./validator-types";

const EVIDENCE_TARGETS = ["final_output", "transcript.full", "transcript.from_mismatch"];

const TYPE_GROUPS = VALIDATOR_GROUPS.map((group) => ({
  label: group,
  options: VALIDATOR_TYPES.filter((t) => t.group === group).map((t) => ({
    value: t.value,
    label: t.label,
  })),
}));

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
    <div className="space-y-6">
      <EditorHeader
        title="Validator"
        description="A deterministic check. Wire it into a scorecard dimension to make it count."
      />

      <FieldRow label="Key" hint="unique; referenced by scorecard dimensions">
        <input
          className={monoControlClass}
          value={def.key ?? ""}
          onChange={(e) => set({ key: e.target.value })}
          placeholder="mentions_refund"
        />
      </FieldRow>

      <FieldRow label="Check">
        <BuilderSelect
          ariaLabel="Check"
          value={def.type ?? "contains"}
          onChange={(v) => changeType(v as ValidatorType)}
          groups={TYPE_GROUPS}
        />
      </FieldRow>

      {meta.forcesToolCalls ? (
        <FieldRow label="Target" hint="tool-call validators inspect the agent's tool calls">
          <input className={monoControlClass} value="tool_calls" disabled />
        </FieldRow>
      ) : meta.isFile ? (
        <FieldRow label="File path" hint="sandbox path to check">
          <input
            className={monoControlClass}
            value={filePath}
            onChange={(e) => set({ target: `file:${e.target.value}` })}
            placeholder="/workspace/out.json"
          />
        </FieldRow>
      ) : (
        <FieldRow label="Target" hint="what gets checked">
          <BuilderSelect
            ariaLabel="Target"
            value={def.target ?? "final_output"}
            onChange={(v) => set({ target: v })}
            options={EVIDENCE_TARGETS.map((t) => ({ value: t, label: t }))}
          />
        </FieldRow>
      )}

      {meta.needsExpected && (
        <FieldRow
          label="Expected"
          hint={'a literal (prefix "literal:") or an evidence reference like case.expectations.answer'}
        >
          <input
            className={monoControlClass}
            value={def.expected_from ?? ""}
            onChange={(e) => set({ expected_from: e.target.value })}
            placeholder="literal:refund"
          />
        </FieldRow>
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
      <FieldRow label={field.label} hint={field.hint}>
        <label className="flex items-center gap-2 py-2 text-sm text-builder-fg-muted">
          <input
            type="checkbox"
            className="size-3.5 accent-builder-fg"
            checked={Boolean(value)}
            onChange={(e) => onChange(e.target.checked)}
          />
          Enabled
        </label>
      </FieldRow>
    );
  }
  if (field.type === "number") {
    return (
      <FieldRow label={field.label} hint={field.hint}>
        <input
          className={controlClass}
          type="number"
          step="any"
          value={typeof value === "number" ? value : ""}
          onChange={(e) => onChange(e.target.value === "" ? undefined : Number(e.target.value))}
          placeholder={field.placeholder}
        />
      </FieldRow>
    );
  }
  if (field.type === "json") {
    return <JsonConfigField field={field} value={value} onChange={onChange} />;
  }
  return (
    <FieldRow label={field.label} hint={field.hint}>
      <input
        className={controlClass}
        value={typeof value === "string" ? value : ""}
        onChange={(e) => onChange(e.target.value)}
        placeholder={field.placeholder}
      />
    </FieldRow>
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
