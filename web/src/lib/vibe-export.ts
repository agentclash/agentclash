import type {ObservedEvidence} from "./vibe-observed";
import type {CanonicalResult} from "./vibe-canonical";
import type { ResultRef } from "./vibe-result-evidence";
import type { SDKReport } from "./vibe-reports";
import type { CaseResult, Session } from "./vibe";

// Session snapshots intentionally omit full case bodies. Exports must retrieve
// the authorized evidence instead of silently exporting only the scorecard.
export async function exportRuns(
  session: Session,
  load: (id: string, key: string) => Promise<CaseResult>,
  selection?: { operationIDs: string[] },
) {
  return Promise.all(
    session.operations
      .filter((operation) => !selection || selection.operationIDs.includes(operation.id))
      .map(async (operation) => ({
        ...operation,
        results: await Promise.all(
          operation.results.map((result) => load(operation.id, result.case_key)),
        ),
      })),
  );
}

export function downloadJSON(name: string, value: unknown) {
  const url = URL.createObjectURL(
    new Blob([JSON.stringify(value, null, 2)], { type: "application/json" }),
  );
  const link = document.createElement("a");
  link.href = url;
  link.download = name;
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

export type ExportItem = {
  kind: "observed_evidence" | "canonical_result" | "sdk_report" | "case" | "original" | "grade_review" | "validation_review" | "scope";
  id: string;
  result_ref?: ResultRef;
  hash?: string;
  operation_id?: string;
  artifact_id?: string;
  case_key?: string;
  missing?: string;
  evidence_changed?: boolean;
};
export type ExportPreparation = {
  format: "agentclash-evaluation-v2";
  session_id: string;
  title: string;
  captured_at: string;
  artifact_id: string;
  selection: { include_observed?:boolean; scope: "selected" | "history"; artifact_id: string; result_id?: string; result_ref?: ResultRef };
  artifacts: Session["document"]["artifacts"];
  rules: unknown[];
  runs: Session["operations"];
  sdk_reports?: SDKReport[];
  canonical_results?: CanonicalResult[];
  observed_evidence?: ObservedEvidence[];
  items: ExportItem[];
  case_mappings: unknown[];
  omissions: string[];
};
export type ExportJob = {
  preparation: ExportPreparation;
  values: Map<string, unknown>;
  missing: Map<string, string>;
};
export const exportItemKey = (item: ExportItem) => `${item.kind}:${item.id}`;
export function newExportJob(preparation: ExportPreparation): ExportJob {
  return {
    preparation: structuredClone(preparation),
    values: new Map(),
    missing: new Map(
      preparation.items.map((item) => [exportItemKey(item), item.missing || "Not loaded yet"]),
    ),
  };
}
export async function collectExport(
  job: ExportJob,
  load: (item: ExportItem) => Promise<unknown>,
  current: () => boolean,
  progress: () => void,
) {
  const pending = job.preparation.items.filter((item) => job.missing.has(exportItemKey(item)));
  let index = 0;
  async function worker() {
    while (current() && index < pending.length) {
      const item = pending[index++],
        key = exportItemKey(item);
      try {
        if (item.missing) throw new Error(item.missing);
        const value = await load(item);
        if (!current()) return;
        if (item.kind === "case") {
          const c = value as CaseResult;
          if (
            c.case_key !== item.case_key ||
            c.version !== item.artifact_id ||
            !item.hash ||
            c.evidence_hash !== item.hash
          )
            throw new Error("Test identity or evidence changed after preparation.");
        }
        job.values.set(key, structuredClone(value));
        job.missing.delete(key);
      } catch (error) {
        if (!current()) return;
        job.missing.set(
          key,
          (error instanceof Error ? error.message : "Evidence unavailable").slice(0, 300),
        );
      }
      progress();
    }
  }
  await Promise.all(Array.from({ length: Math.min(3, pending.length) }, worker));
}
export function exportDocument(job: ExportJob, confirmedPartial = false) {
  if (job.missing.size && !confirmedPartial)
    throw new Error("Confirm the missing evidence before downloading a partial export.");
  const p = job.preparation;
  return {
    ...p,
    items: undefined,
    observed_evidence: undefined,
    runs: p.runs.map((run) => ({
      ...run,
      results: p.items
        .filter(
          (item) =>
            item.kind === "case" &&
            item.operation_id === run.id &&
            job.values.has(exportItemKey(item)),
        )
        .map((item) => job.values.get(exportItemKey(item))),
    })),
    evidence: p.items
      .filter((item) => item.kind !== "case" && job.values.has(exportItemKey(item)))
      .map((item) => ({ reference: item, value: job.values.get(exportItemKey(item)) })),
    manifest: {
      version: 2,
      selection: p.selection,
      completeness: job.missing.size ? "partial" : "exact",
      partial_confirmed: !!job.missing.size && confirmedPartial,
      expected: p.items,
      included: p.items.filter((item) => job.values.has(exportItemKey(item))),
      missing: p.items
        .filter((item) => job.missing.has(exportItemKey(item)))
        .map((item) => ({ ...item, reason: job.missing.get(exportItemKey(item)) })),
      omissions: p.omissions,
    },
  };
}
export function exportFilename(p: ExportPreparation, partial: boolean) {
  const title = p.title.replace(/[^\p{L}\p{N}._-]+/gu, "-").slice(0, 80) || "project";
  return `${title}-${p.artifact_id.slice(0, 8)}-${p.selection.scope === "history" ? "history" : (p.selection.result_ref?.id || p.selection.result_id)?.slice(0, 8) || "definition"}${partial ? "-partial" : ""}.json`;
}
export function downloadBytes(name: string, bytes: Uint8Array, type = "application/octet-stream") {
  const copy = new Uint8Array(bytes);
  const url = URL.createObjectURL(new Blob([copy.buffer], { type }));
  const link = document.createElement("a");
  link.href = url;
  link.download = name;
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
export function bytesBase64(bytes: Uint8Array) {
  let text = "";
  for (let i = 0; i < bytes.length; i += 32768)
    text += String.fromCharCode(...bytes.subarray(i, i + 32768));
  return btoa(text);
}
