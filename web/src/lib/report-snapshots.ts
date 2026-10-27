import type { CaptureScope, ResultRef } from "./vibe-result-evidence";
export type ReportFact = {
    label: string;
    value: string;
};
export type ReportEntry = {
    entry_id: string;
    kind: string;
    label: string;
    facts: ReportFact[];
    text: ReportFact[];
    text_selected: boolean;
};
export type ReportContent = {
    result_ref: ResultRef;
    artifact_id: string;
    source_hash: string;
    evidence_hash: string;
    fingerprints?: {
        model: string | null;
        build: string | null;
        test: string | null;
        judge: string | null;
        config: string | null;
    };
    origin: string;
    status: string;
    omitted_cases: number;
    omissions: string[];
    cases: {
        entry_id: string;
        label: string;
        status: string;
        capture_scope: CaptureScope;
        capture_assurance: string;
        entries: ReportEntry[];
        omitted_entries: number;
    }[];
};
export type ReportSnapshot = {
    id: string;
    workspace_id: string;
    created_at: string;
    content_hash: string;
    content: ReportContent;
};
export type ReportSelection = {
    client_id: string;
    artifact_id: string;
    result_ref: ResultRef;
    evidence_hash: string;
    preview_hash?: string;
    cases: {
        entry_id: string;
        entries: {
            entry_id: string;
            include_text: boolean;
        }[];
    }[];
};
export type ReportNote = {
    id: string;
    author_id: string;
    target_id: string;
    text: string;
    created_at: string;
};
export type ReportLink = {
    id: string;
    key: string;
    expires_at: string;
    revoked_at?: string;
    is_active: boolean;
};
