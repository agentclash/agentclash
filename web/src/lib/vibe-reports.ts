import { vibeFetch, type CaseResult } from "./vibe";
import type { CaptureScope, EvidenceMetadata, Fingerprints, ProjectResultBinding } from "./vibe-result-evidence";
import { bytesBase64, type ExportItem } from "./vibe-export";
export type SDKCaseBinding = { source_case_id: string; case_key: string; input_set_key?: string; challenge_key?: string; published_case_id?: string; capture_scope?: CaptureScope };
export type SDKReportImport = {
  envelope_version: 1; source_kind: "eval_report" | "agent_result"; source_schema_version: 1; source_json: string; external_report_id?: string;
  metadata: { artifact_id: string; challenge_pack_version_id?: string; application_revision: string | null; fingerprints: Fingerprints; capture_scope: CaptureScope; cases: SDKCaseBinding[] };
};
export type SDKReport = {
  id: string; binding: ProjectResultBinding; evidence_metadata: EvidenceMetadata;
  source_kind: SDKReportImport["source_kind"]; source_schema_version: 1; external_report_id: string;
  content_sha256: string; size_bytes: number; created_at: string; cases: SDKCaseBinding[];
  summary?: unknown; diagnostics?: string[];
};
export type PublishedCaseMapping = { artifact_id: string; draft_id: string; challenge_pack_version_id: string; published_case_id: string; input_set_key: string; challenge_key: string; case_key: string };
export type ArtifactCaseMapping = { artifact_id: string; draft_id?: string; composition_hash?: string; cases: { case_key: string; input_set_key: string; challenge_key: string }[]; published: PublishedCaseMapping[]; status?: string; reason?: string };
export const reportBase = (sessionId: string) => `/sessions/${encodeURIComponent(sessionId)}/reports`;
export function unknownCapture(): CaptureScope { return { messages: { scope: "unknown", instrumentation: "unknown" }, tools: { scope: "unknown", instrumentation: "unknown" }, retrieval: { scope: "unknown", instrumentation: "unknown" } }; }
export const unknownFingerprints = (): Fingerprints => ({ model: null, build: null, test: null, judge: null, config: null });
// Preview only; the server applies the unchanged canonical schema. Do not infer
// a binding from a title, array position or matching input text.
export function reportSourceCases(source: string, kind: SDKReportImport["source_kind"]): { id: string; label: string }[] {
  const value = JSON.parse(source);
  if (kind === "agent_result") return [{ id: "", label: "Agent result" }];
  if (!Array.isArray(value.cases) || value.cases.length > 200) throw Error("Choose a v1 eval report with at most 200 cases.");
  return value.cases.map((row: { case?: { case_id?: unknown; name?: unknown } }) => {
    const c = row.case || {};
    if (typeof c.case_id !== "string") throw Error("Each report case needs its original case_id.");
    return { id: c.case_id, label: typeof c.name === "string" ? `${c.case_id} — ${c.name}` : c.case_id };
  });
}
export async function sdkExportValue(item: ExportItem, reports: SDKReport[], bytes: Uint8Array, sessionId: string) {
  const report = reports.find(r => r.id === item.id);
  if (!report || item.result_ref?.kind !== "sdk_report" || item.result_ref.id !== report.id || report.binding.result_ref.kind !== "sdk_report" || report.binding.result_ref.id !== report.id || report.binding.session_id !== sessionId || report.binding.artifact_id !== item.artifact_id || report.content_sha256 !== item.hash) throw Error("SDK export identity differs from the frozen preparation.");
  const hash = Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256", new Uint8Array(bytes).buffer)), b => b.toString(16).padStart(2, "0")).join("");
  if (hash !== item.hash) throw Error("SDK original file hash changed.");
  return { metadata: structuredClone(report), encoding: "base64", content: bytesBase64(bytes), content_sha256: hash };
}
export function loadSDKCase(sessionId: string, report: SDKReport, binding: SDKCaseBinding, token?: string | null) {
  return vibeFetch<CaseResult>(`${reportBase(sessionId)}/${report.id}/case?source_case_id=${encodeURIComponent(binding.source_case_id)}&key=${encodeURIComponent(binding.case_key)}`, token);
}
