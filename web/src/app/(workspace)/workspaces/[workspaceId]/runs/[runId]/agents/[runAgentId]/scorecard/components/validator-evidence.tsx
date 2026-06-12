"use client";

import type { ReactNode } from "react";
import type {
  ValidatorCustomEvidence,
  ValidatorEvidence,
  ValidatorJSONPathEvidence,
  ValidatorJSONSchemaEvidence,
  ValidatorRegexEvidence,
  ValidatorTextCompareEvidence,
  ValidatorToolCallAssertionEvidence,
} from "@/lib/api/types";
import { cn } from "@/lib/utils";
import {
  buildRegexHighlightSegments,
  prettyEvidenceValue,
} from "./validator-evidence-utils";

export function ValidatorEvidenceView({
  evidence,
}: {
  evidence: ValidatorEvidence;
}) {
  switch (evidence.kind) {
    case "text_compare":
      return <TextCompareEvidence evidence={evidence} />;
    case "regex_match":
      return <RegexEvidence evidence={evidence} />;
    case "json_schema":
      return <JSONSchemaEvidence evidence={evidence} />;
    case "json_path_match":
      return <JSONPathEvidence evidence={evidence} />;
    case "tool_call_assertion":
      return <ToolCallAssertionEvidence evidence={evidence} />;
    case "custom":
      return <CustomEvidence evidence={evidence} />;
    default:
      return null;
  }
}

function TextCompareEvidence({
  evidence,
}: {
  evidence: ValidatorTextCompareEvidence;
}) {
  return (
    <EvidenceSection title="Evidence" sourceField={evidence.source_field}>
      <EvidenceGrid
        leftLabel="Expected"
        leftValue={evidence.expected}
        rightLabel="Actual"
        rightValue={evidence.actual}
      />
    </EvidenceSection>
  );
}

function RegexEvidence({ evidence }: { evidence: ValidatorRegexEvidence }) {
  const segments = buildRegexHighlightSegments(evidence.pattern, evidence.actual);

  return (
    <EvidenceSection title="Regex Evidence" sourceField={evidence.source_field}>
      {evidence.pattern && <EvidenceMeta label="Pattern" value={evidence.pattern} />}
      {evidence.matched != null && (
        <EvidenceMeta
          label="Matched"
          value={evidence.matched ? "yes" : "no"}
        />
      )}
      <div className="space-y-2">
        <div className="text-2xs uppercase tracking-[0.18em] text-white/35">
          Actual
        </div>
        <pre className="max-h-[60vh] overflow-auto rounded-2xl border border-white/[0.08] bg-white/[0.03] p-4 text-xs text-white/82 whitespace-pre-wrap font-[family-name:var(--font-mono)]">
          {segments?.map((segment, index) =>
            segment.matched ? (
              <mark
                key={index}
                className="bg-amber-400/25 text-amber-100 rounded px-0.5"
              >
                {segment.text}
              </mark>
            ) : (
              <span key={index}>{segment.text}</span>
            ),
          ) ?? "—"}
        </pre>
      </div>
    </EvidenceSection>
  );
}

function JSONSchemaEvidence({
  evidence,
}: {
  evidence: ValidatorJSONSchemaEvidence;
}) {
  return (
    <EvidenceSection title="Schema Evidence" sourceField={evidence.source_field}>
      {evidence.schema_ref && (
        <EvidenceMeta label="Schema" value={evidence.schema_ref} mono />
      )}
      {evidence.actual != null && (
        <EvidenceBlock label="Actual">{evidence.actual}</EvidenceBlock>
      )}
      {evidence.validation_errors && evidence.validation_errors.length > 0 && (
        <div className="space-y-2">
          <div className="text-2xs uppercase tracking-[0.18em] text-white/35">
            Validation Errors
          </div>
          <ul className="space-y-2">
            {evidence.validation_errors.map((item, index) => (
              <li
                key={`${item}-${index}`}
                className="rounded-2xl border border-red-500/15 bg-red-500/[0.06] px-4 py-3 text-sm text-red-100/85"
              >
                {item}
              </li>
            ))}
          </ul>
        </div>
      )}
    </EvidenceSection>
  );
}

function JSONPathEvidence({
  evidence,
}: {
  evidence: ValidatorJSONPathEvidence;
}) {
  return (
    <EvidenceSection title="Path Evidence" sourceField={evidence.source_field}>
      {evidence.path && <EvidenceMeta label="Path" value={evidence.path} mono />}
      {evidence.comparator && (
        <EvidenceMeta label="Comparator" value={evidence.comparator} mono />
      )}
      {evidence.exists != null && (
        <EvidenceMeta
          label="Exists"
          value={evidence.exists ? "true" : "false"}
          mono
        />
      )}
      <EvidenceGrid
        leftLabel="Expected"
        leftValue={prettyEvidenceValue(evidence.expected)}
        rightLabel="Actual"
        rightValue={prettyEvidenceValue(evidence.actual)}
      />
    </EvidenceSection>
  );
}

function CustomEvidence({ evidence }: { evidence: ValidatorCustomEvidence }) {
  return (
    <EvidenceSection title="Raw Evidence">
      <EvidenceBlock label="Payload">
        {prettyEvidenceValue(evidence.raw)}
      </EvidenceBlock>
    </EvidenceSection>
  );
}

function ToolCallAssertionEvidence({
  evidence,
}: {
  evidence: ValidatorToolCallAssertionEvidence;
}) {
  return (
    <EvidenceSection title="Tool Call Evidence" sourceField={evidence.source_field}>
      <div className="grid gap-3 sm:grid-cols-2">
        {evidence.tool_name && (
          <EvidenceMeta label="Tool" value={evidence.tool_name} mono />
        )}
        {evidence.matched != null && (
          <EvidenceMeta label="Passed" value={evidence.matched ? "yes" : "no"} />
        )}
        {evidence.observed_count != null && (
          <EvidenceMeta label="Observed" value={String(evidence.observed_count)} mono />
        )}
        {evidence.failed_count != null && evidence.failed_count > 0 && (
          <EvidenceMeta label="Failed" value={String(evidence.failed_count)} mono />
        )}
        {evidence.matched_count != null && (
          <EvidenceMeta label="Matches" value={String(evidence.matched_count)} mono />
        )}
        {evidence.expected_count != null && (
          <EvidenceMeta label="Expected" value={String(evidence.expected_count)} mono />
        )}
        {evidence.expected_min_count != null && (
          <EvidenceMeta label="Min" value={String(evidence.expected_min_count)} mono />
        )}
        {evidence.expected_max_count != null && (
          <EvidenceMeta label="Max" value={String(evidence.expected_max_count)} mono />
        )}
        {evidence.expected_order_mode && (
          <EvidenceMeta label="Order" value={evidence.expected_order_mode} mono />
        )}
        {evidence.arguments_contain_set != null && (
          <EvidenceMeta
            label="Args"
            value={evidence.arguments_contain_set ? "fragment" : "not checked"}
          />
        )}
      </div>
      <EvidenceGrid
        leftLabel="Observed Tools"
        leftValue={prettyEvidenceValue(evidence.observed_tool_names)}
        rightLabel="Expected Order"
        rightValue={prettyEvidenceValue(evidence.expected_order)}
      />
      {evidence.matched_indices && evidence.matched_indices.length > 0 && (
        <EvidenceBlock label="Matched Indices">
          {prettyEvidenceValue(evidence.matched_indices)}
        </EvidenceBlock>
      )}
    </EvidenceSection>
  );
}

function EvidenceSection({
  title,
  sourceField,
  children,
}: {
  title: string;
  sourceField?: string;
  children: ReactNode;
}) {
  return (
    <section className="space-y-3">
      <div className="flex items-center justify-between gap-3">
        <h3 className="text-2xs uppercase tracking-[0.22em] text-white/40">
          {title}
        </h3>
        {sourceField && (
          <span className="font-[family-name:var(--font-mono)] text-2xs text-white/35">
            {sourceField}
          </span>
        )}
      </div>
      {children}
    </section>
  );
}

function EvidenceMeta({
  label,
  value,
  mono = false,
}: {
  label: string;
  value: string;
  mono?: boolean;
}) {
  return (
    <div className="grid grid-cols-[92px_minmax(0,1fr)] gap-3 items-start">
      <div className="text-2xs uppercase tracking-[0.18em] text-white/30">
        {label}
      </div>
      <div
        className={cn(
          "text-sm text-white/78 break-words",
          mono && "font-[family-name:var(--font-mono)] text-xs",
        )}
      >
        {value}
      </div>
    </div>
  );
}

function EvidenceGrid({
  leftLabel,
  leftValue,
  rightLabel,
  rightValue,
}: {
  leftLabel: string;
  leftValue?: string;
  rightLabel: string;
  rightValue?: string;
}) {
  return (
    <div className="grid gap-3 sm:grid-cols-2">
      <EvidenceBlock label={leftLabel}>{leftValue ?? "—"}</EvidenceBlock>
      <EvidenceBlock label={rightLabel}>{rightValue ?? "—"}</EvidenceBlock>
    </div>
  );
}

function EvidenceBlock({
  label,
  children,
}: {
  label: string;
  children: string;
}) {
  return (
    <div className="space-y-2">
      <div className="text-2xs uppercase tracking-[0.18em] text-white/35">
        {label}
      </div>
      <pre className="max-h-[60vh] overflow-auto rounded-2xl border border-white/[0.08] bg-white/[0.03] p-4 text-xs text-white/82 whitespace-pre-wrap break-words font-[family-name:var(--font-mono)]">
        {children}
      </pre>
    </div>
  );
}
