"use client";

import { useMemo, useState } from "react";
import { ChevronDown, ChevronRight } from "lucide-react";

import type {
  DatasetGateResult,
  DatasetImportPreviewExample,
  DatasetImportRowError,
} from "@/lib/api/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { JsonField } from "@/components/ui/json-field";

const inputClass =
  "block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50";

function humanizeKey(key: string): string {
  return key.replace(/_/g, " ");
}

function isNestedValue(value: unknown): boolean {
  return (
    value != null &&
    typeof value === "object" &&
    !Array.isArray(value) &&
    Object.keys(value as object).length > 0
  );
}

export function TagBadges({ tags }: { tags: string[] }) {
  if (tags.length === 0) {
    return <span className="text-xs text-muted-foreground">None</span>;
  }
  return (
    <div className="flex flex-wrap gap-1.5">
      {tags.map((tag) => (
        <Badge key={tag} variant="secondary" className="font-normal">
          {tag}
        </Badge>
      ))}
    </div>
  );
}

export function StructuredValue({ value }: { value: unknown }) {
  if (value == null || value === "") {
    return <span className="text-xs text-muted-foreground">—</span>;
  }

  if (typeof value === "string") {
    return (
      <p className="whitespace-pre-wrap break-words text-sm leading-relaxed text-foreground">
        {value}
      </p>
    );
  }

  if (typeof value === "number" || typeof value === "boolean") {
    return (
      <span className="font-[family-name:var(--font-mono)] text-sm">
        {String(value)}
      </span>
    );
  }

  if (Array.isArray(value)) {
    if (
      value.length > 0 &&
      value.every((item) => typeof item === "string" || typeof item === "number")
    ) {
      return <TagBadges tags={value.map(String)} />;
    }
    return (
      <ul className="space-y-2">
        {value.map((item, index) => (
          <li
            key={index}
            className="rounded-md border border-border/70 bg-muted/20 px-3 py-2"
          >
            <StructuredValue value={item} />
          </li>
        ))}
      </ul>
    );
  }

  if (typeof value === "object") {
    const entries = Object.entries(value as Record<string, unknown>).filter(
      ([, v]) => v != null && v !== "",
    );
    if (entries.length === 0) {
      return <span className="text-xs text-muted-foreground">Empty</span>;
    }
    return (
      <dl className="grid gap-x-4 gap-y-3 text-sm sm:grid-cols-2">
        {entries.map(([key, nested]) => (
          <div
            key={key}
            className={isNestedValue(nested) ? "sm:col-span-2" : undefined}
          >
            <dt className="text-2xs font-medium uppercase tracking-wide text-muted-foreground">
              {humanizeKey(key)}
            </dt>
            <dd className="mt-1">
              <StructuredValue value={nested} />
            </dd>
          </div>
        ))}
      </dl>
    );
  }

  return <span className="text-sm">{String(value)}</span>;
}

export function CollapsibleSection({
  title,
  defaultOpen = false,
  children,
}: {
  title: string;
  defaultOpen?: boolean;
  children: React.ReactNode;
}) {
  const [open, setOpen] = useState(defaultOpen);
  return (
    <div className="rounded-lg border border-border/80 bg-card/20">
      <button
        type="button"
        onClick={() => setOpen((prev) => !prev)}
        className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm font-medium hover:bg-muted/30"
      >
        {open ? (
          <ChevronDown className="size-4 shrink-0 text-muted-foreground" />
        ) : (
          <ChevronRight className="size-4 shrink-0 text-muted-foreground" />
        )}
        {title}
      </button>
      {open ? <div className="border-t border-border/70 px-3 py-3">{children}</div> : null}
    </div>
  );
}

export function ExamplePayloadPreview({
  input,
  expected,
  metadata,
}: {
  input: unknown;
  expected?: unknown;
  metadata?: Record<string, unknown>;
}) {
  const metaEntries = Object.entries(metadata ?? {}).filter(
    ([, v]) => v != null && v !== "",
  );

  return (
    <div className="space-y-3">
      <CollapsibleSection title="Input" defaultOpen>
        <StructuredValue value={input} />
      </CollapsibleSection>
      {expected != null ? (
        <CollapsibleSection title="Expected output">
          <StructuredValue value={expected} />
        </CollapsibleSection>
      ) : null}
      {metaEntries.length > 0 ? (
        <CollapsibleSection title="Metadata">
          <StructuredValue value={metadata} />
        </CollapsibleSection>
      ) : null}
    </div>
  );
}

export type MappingFieldState = {
  idKey: string;
  tagsKey: string;
  inputKeys: string;
  outputKeys: string;
  metadataKeys: string;
};

export const EMPTY_MAPPING_FIELDS: MappingFieldState = {
  idKey: "",
  tagsKey: "",
  inputKeys: "",
  outputKeys: "",
  metadataKeys: "",
};

function splitCsv(value: string): string[] {
  return value
    .split(",")
    .map((part) => part.trim())
    .filter(Boolean);
}

export function buildMappingFromFields(
  fields: MappingFieldState,
): Record<string, unknown> | undefined {
  const mapping: Record<string, unknown> = {};
  if (fields.idKey.trim()) mapping.id_key = fields.idKey.trim();
  if (fields.tagsKey.trim()) mapping.tags_key = fields.tagsKey.trim();
  const inputKeys = splitCsv(fields.inputKeys);
  if (inputKeys.length > 0) mapping.input_keys = inputKeys;
  const outputKeys = splitCsv(fields.outputKeys);
  if (outputKeys.length > 0) mapping.output_keys = outputKeys;
  const metadataKeys = splitCsv(fields.metadataKeys);
  if (metadataKeys.length > 0) mapping.metadata_keys = metadataKeys;
  return Object.keys(mapping).length > 0 ? mapping : undefined;
}

export function MappingEditor({
  mode,
  onModeChange,
  fields,
  onFieldsChange,
  json,
  onJsonChange,
  jsonError,
}: {
  mode: "simple" | "advanced";
  onModeChange: (mode: "simple" | "advanced") => void;
  fields: MappingFieldState;
  onFieldsChange: (fields: MappingFieldState) => void;
  json: string;
  onJsonChange: (value: string) => void;
  jsonError?: string;
}) {
  return (
    <div className="space-y-3 rounded-lg border border-border/80 bg-muted/10 p-3">
      <div className="flex items-center justify-between gap-2">
        <p className="text-sm font-medium">Column mapping</p>
        <div className="flex rounded-md border border-border p-0.5 text-xs">
          <button
            type="button"
            className={`rounded px-2 py-1 ${mode === "simple" ? "bg-muted font-medium" : "text-muted-foreground"}`}
            onClick={() => onModeChange("simple")}
          >
            Simple
          </button>
          <button
            type="button"
            className={`rounded px-2 py-1 ${mode === "advanced" ? "bg-muted font-medium" : "text-muted-foreground"}`}
            onClick={() => onModeChange("advanced")}
          >
            JSON
          </button>
        </div>
      </div>
      {mode === "simple" ? (
        <div className="grid gap-3 sm:grid-cols-2">
          <MappingTextField
            label="ID column"
            value={fields.idKey}
            onChange={(idKey) => onFieldsChange({ ...fields, idKey })}
            placeholder="example_id"
          />
          <MappingTextField
            label="Tags column"
            value={fields.tagsKey}
            onChange={(tagsKey) => onFieldsChange({ ...fields, tagsKey })}
            placeholder="labels"
          />
          <MappingTextField
            label="Input columns"
            value={fields.inputKeys}
            onChange={(inputKeys) => onFieldsChange({ ...fields, inputKeys })}
            placeholder="prompt, locale"
          />
          <MappingTextField
            label="Output columns"
            value={fields.outputKeys}
            onChange={(outputKeys) => onFieldsChange({ ...fields, outputKeys })}
            placeholder="answer"
          />
          <MappingTextField
            label="Metadata columns"
            value={fields.metadataKeys}
            onChange={(metadataKeys) =>
              onFieldsChange({ ...fields, metadataKeys })
            }
            placeholder="source, locale"
            className="sm:col-span-2"
          />
        </div>
      ) : (
        <JsonField
          label="Mapping JSON"
          description='Example: {"input_keys":["prompt"],"id_key":"id"}'
          value={json}
          onChange={onJsonChange}
          error={jsonError}
          rows={5}
        />
      )}
    </div>
  );
}

function MappingTextField({
  label,
  value,
  onChange,
  placeholder,
  className,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  className?: string;
}) {
  return (
    <div className={className}>
      <label className="mb-1 block text-xs font-medium text-muted-foreground">
        {label}
      </label>
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className={inputClass}
      />
    </div>
  );
}

export function parseMappingInput(
  mode: "simple" | "advanced",
  fields: MappingFieldState,
  json: string,
): { mapping?: Record<string, unknown>; error?: string } {
  if (mode === "simple") {
    return { mapping: buildMappingFromFields(fields) };
  }
  if (!json.trim()) {
    return { mapping: undefined };
  }
  try {
    const parsed = JSON.parse(json) as unknown;
    if (parsed == null || typeof parsed !== "object" || Array.isArray(parsed)) {
      return { error: "Mapping must be a JSON object" };
    }
    return { mapping: parsed as Record<string, unknown> };
  } catch {
    return { error: "Invalid mapping JSON" };
  }
}

export type RedactionFieldState = {
  dropKeys: string;
  hashKeys: string;
  dropPaths: string;
};

export const EMPTY_REDACTION_FIELDS: RedactionFieldState = {
  dropKeys: "",
  hashKeys: "",
  dropPaths: "",
};

export function buildRedactionFromFields(
  fields: RedactionFieldState,
): ImportDatasetTracesRedaction | undefined {
  const redaction: ImportDatasetTracesRedaction = {};
  const dropKeys = splitCsv(fields.dropKeys);
  if (dropKeys.length > 0) redaction.drop_metadata_keys = dropKeys;
  const hashKeys = splitCsv(fields.hashKeys);
  if (hashKeys.length > 0) redaction.hash_metadata_keys = hashKeys;
  const dropPaths = splitCsv(fields.dropPaths);
  if (dropPaths.length > 0) redaction.drop_metadata_paths = dropPaths;
  return Object.keys(redaction).length > 0 ? redaction : undefined;
}

type ImportDatasetTracesRedaction = {
  drop_metadata_keys?: string[];
  hash_metadata_keys?: string[];
  drop_metadata_paths?: string[];
};

export function RedactionEditor({
  fields,
  onFieldsChange,
}: {
  fields: RedactionFieldState;
  onFieldsChange: (fields: RedactionFieldState) => void;
}) {
  return (
    <div className="space-y-3 rounded-lg border border-border/80 bg-muted/10 p-3">
      <p className="text-sm font-medium">Redaction</p>
      <MappingTextField
        label="Drop metadata keys"
        value={fields.dropKeys}
        onChange={(dropKeys) => onFieldsChange({ ...fields, dropKeys })}
        placeholder="email, user_id"
      />
      <MappingTextField
        label="Hash metadata keys"
        value={fields.hashKeys}
        onChange={(hashKeys) => onFieldsChange({ ...fields, hashKeys })}
        placeholder="session_id"
      />
      <MappingTextField
        label="Drop metadata paths"
        value={fields.dropPaths}
        onChange={(dropPaths) => onFieldsChange({ ...fields, dropPaths })}
        placeholder="user.email"
      />
    </div>
  );
}

export function ImportPreviewPanel({
  importedCount,
  preview,
  errors,
}: {
  importedCount: number;
  preview?: DatasetImportPreviewExample[];
  errors?: DatasetImportRowError[];
}) {
  return (
    <div className="space-y-3 rounded-lg border border-border bg-muted/20 p-3">
      <p className="text-sm font-medium">
        Preview: {importedCount} row{importedCount === 1 ? "" : "s"}
      </p>
      {errors && errors.length > 0 ? (
        <div className="space-y-1">
          <p className="text-xs font-medium text-destructive">
            {errors.length} row error{errors.length === 1 ? "" : "s"}
          </p>
          <ul className="max-h-24 space-y-1 overflow-y-auto text-xs text-destructive/90">
            {errors.slice(0, 8).map((err, index) => (
              <li key={`${err.row}-${index}`}>
                Row {err.row}
                {err.field ? ` · ${err.field}` : ""}: {err.message}
              </li>
            ))}
          </ul>
        </div>
      ) : null}
      {preview && preview.length > 0 ? (
        <div className="space-y-2">
          {preview.slice(0, 3).map((row, index) => (
            <div
              key={index}
              className="rounded-md border border-border/70 bg-background/40 p-3"
            >
              <div className="mb-2 flex flex-wrap items-center gap-2">
                {row.external_id ? (
                  <span className="text-xs font-medium">{row.external_id}</span>
                ) : (
                  <span className="text-xs text-muted-foreground">
                    Row {index + 1}
                  </span>
                )}
                {row.tags && row.tags.length > 0 ? (
                  <TagBadges tags={row.tags} />
                ) : null}
              </div>
              <ExamplePayloadPreview
                input={row.input}
                expected={row.expected}
                metadata={row.metadata}
              />
            </div>
          ))}
          {preview.length > 3 ? (
            <p className="text-xs text-muted-foreground">
              +{preview.length - 3} more rows in preview
            </p>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

export function GateResultPanel({
  gate,
  onDownloadJUnit,
}: {
  gate: DatasetGateResult;
  onDownloadJUnit?: () => void;
}) {
  const passRate = useMemo(
    () => `${Math.round(gate.pass_rate * 1000) / 10}%`,
    [gate.pass_rate],
  );
  const baselinePassRate = useMemo(
    () => `${Math.round(gate.baseline_pass_rate * 1000) / 10}%`,
    [gate.baseline_pass_rate],
  );

  return (
    <div className="space-y-4 rounded-lg border border-border bg-muted/15 p-4">
      <div className="flex flex-wrap items-center gap-2">
        <Badge variant={gate.pass ? "default" : "destructive"}>
          {gate.pass ? "PASS" : "FAIL"}
        </Badge>
        <span className="text-sm text-muted-foreground">
          Pass rate {passRate} · Baseline {baselinePassRate}
        </span>
        <span className="text-sm text-muted-foreground">
          {gate.regression_count} regression
          {gate.regression_count === 1 ? "" : "s"} · {gate.evaluated_examples}{" "}
          examples
        </span>
        {onDownloadJUnit ? (
          <Button type="button" size="sm" variant="outline" onClick={onDownloadJUnit}>
            Download JUnit
          </Button>
        ) : null}
      </div>

      {gate.failed_thresholds && gate.failed_thresholds.length > 0 ? (
        <div>
          <p className="mb-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
            Failed thresholds
          </p>
          <TagBadges tags={gate.failed_thresholds} />
        </div>
      ) : null}

      {gate.regressions.length > 0 ? (
        <div className="overflow-hidden rounded-md border border-border">
          <table className="w-full text-sm">
            <thead className="border-b border-border bg-muted/30 text-left text-xs text-muted-foreground">
              <tr>
                <th className="px-3 py-2 font-medium">Example</th>
                <th className="px-3 py-2 font-medium">Reason</th>
                <th className="px-3 py-2 font-medium">Baseline</th>
                <th className="px-3 py-2 font-medium">Candidate</th>
              </tr>
            </thead>
            <tbody>
              {gate.regressions.map((row) => (
                <tr key={`${row.dataset_example_id}-${row.reason}`} className="border-b border-border/60 last:border-0">
                  <td className="px-3 py-2 font-[family-name:var(--font-mono)] text-xs">
                    {row.dataset_example_id.slice(0, 8)}
                  </td>
                  <td className="px-3 py-2">{row.reason}</td>
                  <td className="px-3 py-2 text-muted-foreground">
                    {row.baseline_verdict ?? "—"}
                    {row.baseline_score != null
                      ? ` (${row.baseline_score.toFixed(2)})`
                      : ""}
                  </td>
                  <td className="px-3 py-2 text-muted-foreground">
                    {row.candidate_verdict ?? "—"}
                    {row.candidate_score != null
                      ? ` (${row.candidate_score.toFixed(2)})`
                      : ""}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : null}
    </div>
  );
}

export function generationJobStorageKey(datasetId: string): string {
  return `agentclash:dataset-generation-jobs:${datasetId}`;
}

export function readStoredGenerationJobIds(datasetId: string): string[] {
  if (typeof window === "undefined") return [];
  try {
    const raw = window.localStorage.getItem(generationJobStorageKey(datasetId));
    if (!raw) return [];
    const parsed = JSON.parse(raw) as unknown;
    return Array.isArray(parsed)
      ? parsed.filter((id): id is string => typeof id === "string")
      : [];
  } catch {
    return [];
  }
}

export function storeGenerationJobId(datasetId: string, jobId: string): void {
  if (typeof window === "undefined") return;
  const existing = readStoredGenerationJobIds(datasetId).filter(
    (id) => id !== jobId,
  );
  const next = [jobId, ...existing].slice(0, 10);
  window.localStorage.setItem(
    generationJobStorageKey(datasetId),
    JSON.stringify(next),
  );
}
