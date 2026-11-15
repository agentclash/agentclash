import { vibeFetch, type CaseResult } from "./vibe";
import type { CaptureScope, EvidenceMetadata, Fingerprints, ProjectResultBinding } from "./vibe-result-evidence";
import { bytesBase64, type ExportItem } from "./vibe-export";
export type SDKCaseBinding = { source_case_id: string; case_key: string; input_set_key?: string; challenge_key?: string; published_case_id?: string; capture_scope?: CaptureScope };
export type SDKReportImport = {
  envelope_version: 1; source_json: string; external_report_id?: string;
  metadata: { artifact_id: string; challenge_pack_version_id?: string; application_revision: string | null; fingerprints: Fingerprints; capture_scope: CaptureScope; cases: SDKCaseBinding[] };
} & ({ source_kind: "eval_report"; source_schema_version: 1 | 2 } | { source_kind: "agent_result"; source_schema_version: 1 });
export type SDKReport = {
  id: string; binding: ProjectResultBinding; evidence_metadata: EvidenceMetadata;
  source_kind: SDKReportImport["source_kind"]; source_schema_version: 1 | 2; external_report_id: string;
  content_sha256: string; size_bytes: number; created_at: string; cases: SDKCaseBinding[];
  summary?: unknown; diagnostics?: string[]; source_provenance?: unknown;
};
export type PublishedCaseMapping = { artifact_id: string; draft_id: string; challenge_pack_version_id: string; published_case_id: string; input_set_key: string; challenge_key: string; case_key: string };
export type ArtifactCaseMapping = { artifact_id: string; draft_id?: string; composition_hash?: string; cases: { case_key: string; input_set_key: string; challenge_key: string }[]; published: PublishedCaseMapping[]; status?: string; reason?: string };
// Retained publication receipts outlive their editable advanced draft.
export async function loadArtifactCaseMapping(sessionId: string, artifactId: string, token?: string | null): Promise<ArtifactCaseMapping[]> {
  const rows = await vibeFetch<ArtifactCaseMapping[]>(`/sessions/${encodeURIComponent(sessionId)}/artifacts/${encodeURIComponent(artifactId)}/case-mapping`, token);
  if (!Array.isArray(rows) || rows.some(row => !row || row.artifact_id !== artifactId || !Array.isArray(row.cases) || !Array.isArray(row.published))) throw Error("Test mapping could not be read.");
  return rows;
}
export const reportBase = (sessionId: string) => `/sessions/${encodeURIComponent(sessionId)}/reports`;
export function unknownCapture(): CaptureScope { return { messages: { scope: "unknown", instrumentation: "unknown" }, tools: { scope: "unknown", instrumentation: "unknown" }, retrieval: { scope: "unknown", instrumentation: "unknown" } }; }
export const unknownFingerprints = (): Fingerprints => ({ model: null, build: null, test: null, judge: null, config: null });
// Preview only; the server applies the versioned canonical schema. Do not infer
// a binding from a title, array position, matching input text or a similar ID.
// An identical ID is the one permitted suggestion, and the user can change it.
export function exactCaseTarget(sourceCaseID: string, tuples: [string, { case_key: string }][]): string {
  const hits = sourceCaseID ? tuples.filter(([, tuple]) => tuple.case_key === sourceCaseID) : [];
  return hits.length === 1 ? hits[0][0] : "";
}
export function reportSourceCases(source: string, kind: SDKReportImport["source_kind"], version: 1 | 2 = 1): { id: string; label: string }[] {
  const value = JSON.parse(source);
  if (kind === "agent_result" && (version !== 1 || value.schema_version !== undefined)) throw Error("Choose a standalone agent result v1.");
  if (kind === "agent_result") return [{ id: "", label: "Agent result" }];
  if (value.schema_version !== version && (version === 2 || value.schema_version !== undefined)) throw Error(`This file is not an eval report v${version}. Select its explicit source format.`);
  if (!Array.isArray(value.cases) || value.cases.length > 200) throw Error("Choose a supported eval report with at most 200 tests.");
  return value.cases.map((row: { case?: { case_id?: unknown; name?: unknown } }) => {
    const c = row.case || {};
    if (typeof c.case_id !== "string") throw Error("Each report test needs its original case_id.");
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

export type SDKImportFormat = { envelope_version: number; source_kind: string; source_schema_version: number };
export function supportsRefundV2(config: { sdk_report_import_formats?: SDKImportFormat[] } | null | undefined): boolean {
  return Array.isArray(config?.sdk_report_import_formats) && config.sdk_report_import_formats.some(f => f?.envelope_version === 1 && f.source_kind === "eval_report" && f.source_schema_version === 2);
}
// A preview is not validation or execution. Never recursively render arbitrary
// raw JSON; the backend applies the canonical schema and semantic checks on save.
export function refundReportPreview(source: string) {
  const d = JSON.parse(source);
  if (d?.schema_version !== 2 || !Array.isArray(d.cases) || d.cases.length > 200) throw Error("Choose a refund session report v2 with at most 200 tests.");
  const text = (v: unknown, max = 300) => typeof v === "string" ? Array.from(v).slice(0, max).join("") : null;
  const flag = (v: unknown) => typeof v === "boolean" ? v : null;
  return {
    report_id: text(d.report_id, 200), suite_id: text(d.suite_id), suite_digest: text(d.suite_digest),
    target_identity: text(d.target_identity), target_revision: text(d.target_revision),
    execution_status: text(d.execution?.status, 40), required_cases_complete: flag(d.execution?.required_cases_complete),
    missing_required_case_ids: Array.isArray(d.execution?.missing_required_case_ids) ? d.execution.missing_required_case_ids.slice(0,20).map((v: unknown) => text(v,80)) : null,
    cost_usd: null,
    cases: d.cases.map((c: {case?: {case_id?:unknown;name?:unknown};status?:unknown;target_invoked?:unknown;observations?:{tool_calls?:unknown;fixture_state?:unknown};checks?:unknown}) => ({
      case_id: text(c?.case?.case_id,80), name: text(c?.case?.name,200), status: text(c?.status,40),
      target_invoked: flag(c?.target_invoked), tool_calls_observed: flag(c?.observations?.tool_calls), fixture_state_observed: flag(c?.observations?.fixture_state),
      checks: Array.isArray(c?.checks) ? c.checks.slice(0,100).map(check => ({check_id:text(check?.check_id,80),passed:flag(check?.passed)})) : [],
    })),
  };
}

// A local report names its tests but not their inputs. These placeholder tests keep
// the original case IDs so the report can attach to a test version. The required
// capability is deliberately one that hosted execution lacks, so they never run here.
export const localRunnerCapability = "local_runner";
export function reportToTestPack(source: string) {
  const d = JSON.parse(source);
  const cases = reportSourceCases(source, "eval_report", 2);
  if (!cases.length) throw Error("This report has no tests, so there is nothing to keep it with.");
  if (new Set(cases.map(c => c.id)).size !== cases.length || cases.some(c => !c.id.trim() || c.id !== c.id.trim() || new TextEncoder().encode(c.id).length > 128)) throw Error("Each report test needs a unique case_id of at most 128 bytes.");
  const text = (v: unknown, max: number) => typeof v === "string" ? Array.from(v.replace(/[\u0000-\u001f\u007f-\u009f]/g, " ").trim()).slice(0, max).join("") : "";
  const title = text(d.suite_id, 60) || "Local report";
  const note = "Placeholder tests derived from an uploaded local report. The local runner decides their results; they cannot run here.";
  return {
    title, caseIDs: cases.map(c => c.id), suiteDigest: text(d.suite_digest, 200) || null, targetRevision: text(d.target_revision, 200) || null,
    pack: {
      format: "agentclash-vibe-v2", title, agent_prompt: "", scope_note: note, required_capabilities: [localRunnerCapability],
      evaluation: {
        slug: "local-report", name: title, description: note, difficulty: "easy", instructions: "{{question}}",
        cases: cases.map(c => ({ key: c.id, payload: { question: `Local test ${c.label}` } })),
        validators: [{ key: "has_answer", type: "regex_match", target: "final_output", expected_from: "literal:.+" }],
        judges: [{ key: "behavior", mode: "assertion", assertion: "The local runner decided this test. This placeholder cannot grade a reply." }],
        dimensions: [{ key: "output_present", source: "validators", validators: ["has_answer"] }, { key: "behavior_correctness", source: "llm_judge", judge_key: "behavior" }],
      },
    },
  };
}
