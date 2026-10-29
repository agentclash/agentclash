"use client";
import { ReportSnapshotComposer } from "./report-snapshot-composer";
import { useEffect, useRef, useState } from "react";
import { terminal, VibeError } from "@/lib/vibe";
import { testCount } from "@/lib/vibe-presentation";
import { loadObservedEvidence, type ObservedEvidence, type ObservedEntry } from "@/lib/vibe-observed";
import type { DimensionCapture, ProjectResultBinding } from "@/lib/vibe-result-evidence";
import type { WorkspaceEvidenceAccess } from "@/lib/use-vibe-workspace-access";
import { useVibeEvidenceOwner } from "@/lib/use-vibe-evidence-owner";
import { VibeButton } from "./vibe-button";
function coverage(d: DimensionCapture, count: number) { return d.instrumentation === "not_instrumented" ? "not instrumented" : d.scope === "complete" && d.instrumentation === "instrumented" ? (count ? `complete · ${count} observed` : "observed none") : d.scope === "partial" ? `partial · ${count} observed` : `unknown${count ? ` · ${count} retained` : ""}`; }
function Entry({ entry }: {
    entry: ObservedEntry;
}) {
    const value = entry.value;
    const fields = value && typeof value === "object" ? value as Record<string, unknown> : {};
    const text = typeof value === "string" ? value : typeof fields.content === "string" ? fields.content : entry.value_json || JSON.stringify(value, null, 2);
    const grade = entry.kind === "grade";
    const label = [fields.name, fields.key, fields.judge_key].find(v => typeof v === "string");
    const verdict = typeof fields.verdict === "string" ? fields.verdict : typeof fields.passed === "boolean" ? fields.passed ? "passed" : "failed" : typeof fields.status === "string" ? fields.status : "Grade retained";
    return <div className="min-w-0 space-y-1">
  {grade ? <><p>{typeof label === "string" ? `${label}: ` : ""}{verdict}{entry.score_text !== undefined ? ` · Score ${entry.score_text}` : ""}</p><details className="text-sm"><summary>Original grade JSON</summary><pre className="whitespace-pre-wrap break-words [overflow-wrap:anywhere]">{entry.value_json || JSON.stringify(value, null, 2)}</pre></details></> : <pre className="whitespace-pre-wrap break-words text-sm [overflow-wrap:anywhere]">{text}</pre>}
  <details className="text-xs vibe-muted"><summary>Source identity</summary><p className="break-all">{entry.entry_id}</p><p className="break-all">{entry.source_path}{entry.event_id ? ` · Event ${entry.event_id}` : ""}{entry.source_event_id ? ` · Source ${entry.source_event_id}` : ""}</p><p>Source index {entry.source_index}{entry.sequence !== undefined ? ` · Sequence ${entry.sequence}` : ""}</p>{entry.payload_hash && <p className="break-all">{entry.payload_hash}</p>}</details>
 </div>;
}
export function ObservedResults(p: {
    binding: ProjectResultBinding;
    actorScope: string;
    workspaceId?: string;
    token: () => Promise<string | null | undefined>;
    access: WorkspaceEvidenceAccess;
    onExport: () => void;
}) {
    const { binding } = p, ref = binding.result_ref;
    const revoked = p.access.status === "signed_out" || p.access.status === "ready" && !p.access.canRead;
    const life = useVibeEvidenceOwner(p.actorScope, binding.session_id, p.workspaceId, binding.artifact_id, ref.kind, ref.id, revoked ? "revoked" : "active");
    const token = useRef(p.token);
    useEffect(() => { token.current = p.token; }, [p.token]);
    const [state, setState] = useState<{
        owner: object;
        value?: ObservedEvidence;
        error?: string;
    }>(), [attempt, retry] = useState(0), [caseID, setCaseID] = useState("");
    useEffect(() => {
        if (!p.access.canRead)
            return;
        const current = life.capture();
        let live = true, timer: ReturnType<typeof setTimeout>;
        async function load() {
            try {
                const auth = await token.current();
                if (!live || !current())
                    return;
                const value = await loadObservedEvidence({ session_id: binding.session_id, artifact_id: binding.artifact_id, result_ref: { kind: ref.kind, id: ref.id } }, auth);
                if (!live || !current())
                    return;
                setState({ owner: life.owner, value });
                if (ref.kind === "vibe_operation" ? !terminal(value.status) : ["queued", "running", "provisioning", "scoring", "aggregating"].includes(value.status))
                    timer = setTimeout(load, 3000);
            }
            catch (e) {
                if (!live || !current())
                    return;
                const denied = e instanceof VibeError && ["not_found", "forbidden"].includes(e.code);
                setState(old => ({ owner: life.owner, value: !denied && old?.owner === life.owner ? old.value : undefined, error: (e as Error).message }));
            }
        }
        void load();
        return () => { live = false; clearTimeout(timer); };
    }, [life, binding.session_id, binding.artifact_id, ref.kind, ref.id, p.access.canRead, attempt]);
    if (!p.access.canRead)
        return null;
    const active = state?.owner === life.owner ? state : undefined, source = active?.value, selected = source?.cases.find(c => c.entry_id === caseID) || source?.cases[0];
    const groups = [{ kind: "grade", title: "Original grades" }, { kind: "diagnostic", title: "Source error" }, { kind: "output", title: "Saved output" }, { kind: "message", title: "Messages" }, { kind: "tool_proposal", title: "Tool proposals" }, { kind: "tool_execution", title: "Observed tool executions" }, { kind: "tool_attempt", title: "Tool attempts" }, { kind: "retrieval", title: "Observed retrieval" }];
    return <section aria-label="Observed result evidence" className="min-w-0 space-y-3"><div className="flex flex-wrap items-center justify-between gap-2"><h3>Test evidence</h3><VibeButton onClick={() => retry(n => n + 1)}>Refresh evidence</VibeButton></div>{active?.error && <p role="alert">{active.error}</p>}{!source && !active?.error && <p role="status">Loading test evidence…</p>}{source && <><p role="status">{source.status} · {testCount(source.cases.filter(c => c.source_index >= 0).length)} · {source.evidence_metadata.origin === "imported" ? "Imported SDK claims" : ref.kind === "vibe_operation" ? "Vibe Evals run" : "Canonical execution"}</p>{selected ? <><label className="block text-sm">Test or participant<select className="mt-1 w-full min-w-0 rounded-[var(--vibe-radius-sm)] border bg-background p-2" value={selected.entry_id} onChange={e => setCaseID(e.target.value)}>{source.cases.map((c, i) => <option key={c.entry_id} value={c.entry_id}>{c.source_index < 0 ? "Unbound participant evidence" : `Test ${i + 1}: ${c.case_key}`} · {c.run_agent_id ? `Participant ${c.run_agent_id.slice(0, 8)} · ` : ""}{c.status}</option>)}</select></label><p>{selected.status} · {selected.capture_assurance}</p>{selected.run_agent_id && <details><summary>Participant identity</summary><p className="break-all">Run agent: {selected.run_agent_id}</p></details>}<p className="text-sm vibe-muted">Messages: {coverage(selected.capture_scope.messages, selected.entries.filter(e => e.kind === "message").length)} · Tools: {coverage(selected.capture_scope.tools, selected.entries.filter(e => e.kind === "tool_execution" || e.kind === "tool_attempt").length)} · Retrieval: {coverage(selected.capture_scope.retrieval, selected.entries.filter(e => e.kind === "retrieval").length)}</p>{groups.map(group => { const entries = selected.entries.filter(e => e.kind === group.kind); return entries.length > 0 && <section key={group.kind} aria-label={group.title} className="space-y-2"><h4>{group.title}</h4>{entries.map(entry => <Entry key={entry.entry_id} entry={entry}/>)}</section>; })}{selected.entries.length === 0 && <p>No retained observations are attributed to this test yet.</p>}</> : <p>No test evidence is present. The original report may contain configuration diagnostics.</p>}<details><summary>Source and capture details</summary><p className="break-all">{source.binding.result_ref.kind}:{source.binding.result_ref.id}</p><p className="break-all">Evidence hash: {source.evidence_hash}</p>{source.omissions.map(note => <p className="text-sm" key={note}>{note}</p>)}<p>Saved reference materials are not observed retrieval. Proposals do not prove a tool executed. Unknown capture does not mean nothing happened.</p></details><VibeButton onClick={() => {
                if (life.current())
                    p.onExport();
            }}>Export with observed evidence</VibeButton><ReportSnapshotComposer source={source} actorScope={p.actorScope} workspaceId={p.workspaceId} token={p.token} access={p.access}/></>}</section>;
}
