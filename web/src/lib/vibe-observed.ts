import { vibeFetch } from "./vibe";
import type { CaptureScope, CaseEvidenceBinding, EvidenceMetadata, ProjectResultBinding } from "./vibe-result-evidence";
import type { ExportItem } from "./vibe-export";
export type ObservedEntry = {
    score_text?: string;
    entry_id: string;
    kind: string;
    source_path: string;
    source_index: number;
    event_id?: string;
    source_event_id?: string;
    sequence?: number;
    payload_hash?: string;
    value: unknown;
    value_json?: string;
};
export type ObservedCase = CaseEvidenceBinding & {
    entry_id: string;
    source_index: number;
    status: string;
    capture_scope: CaptureScope;
    capture_assurance: string;
    entries: ObservedEntry[];
};
export type ObservedEvidence = {
    binding: ProjectResultBinding;
    evidence_metadata: EvidenceMetadata;
    source_hash: string;
    evidence_hash: string;
    status: string;
    cases: ObservedCase[];
    omissions: string[];
};
export async function loadObservedEvidence(binding: ProjectResultBinding, token?: string | null, hash?: string) {
    const ref = binding.result_ref, params = new URLSearchParams({ artifact_id: binding.artifact_id });
    if (hash)
        params.set("hash", hash);
    const out = await vibeFetch<ObservedEvidence>(`/sessions/${encodeURIComponent(binding.session_id)}/results/${ref.kind}/${encodeURIComponent(ref.id)}/observed-evidence?${params}`, token);
    if (out.binding.session_id !== binding.session_id || out.binding.artifact_id !== binding.artifact_id || out.binding.result_ref.kind !== ref.kind || out.binding.result_ref.id !== ref.id || !out.evidence_hash)
        throw Error("Observed source identity changed.");
    return out;
}
export function observedExportValue(item: ExportItem, values: ObservedEvidence[], session: string) {
    const value = values.find(v => v.binding.result_ref.kind === item.result_ref?.kind && v.binding.result_ref.id === item.result_ref.id && v.binding.artifact_id === item.artifact_id);
    if (!value || value.binding.session_id !== session || value.evidence_hash !== item.hash || item.id !== `${value.binding.result_ref.kind}:${value.binding.result_ref.id}:${value.binding.artifact_id}`)
        throw Error("Observed export differs from the frozen preparation.");
    return structuredClone(value);
}
