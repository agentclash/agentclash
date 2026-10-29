"use client";
import Link from "next/link";
import { useState } from "react";
import { vibeFetch } from "@/lib/vibe";
import type { ObservedEvidence } from "@/lib/vibe-observed";
import type { ReportSelection, ReportSnapshot } from "@/lib/report-snapshots";
import type { WorkspaceEvidenceAccess } from "@/lib/use-vibe-workspace-access";
import { useVibeEvidenceOwner } from "@/lib/use-vibe-evidence-owner";
import { ReportSnapshotView } from "@/components/share/report-snapshot-view";
import { VibeButton } from "./vibe-button";
export function ReportSnapshotComposer(p: {
    source: ObservedEvidence;
    actorScope: string;
    workspaceId?: string;
    token: () => Promise<string | null | undefined>;
    access: WorkspaceEvidenceAccess;
}) {
    const { source } = p;
    const life = useVibeEvidenceOwner(p.actorScope, p.workspaceId, source.binding.session_id, source.binding.artifact_id, source.evidence_hash, p.access.canWrite ? "write" : "denied");
    // The keyed inner form also drops every draft/preview when evidence or authority changes.
    if (!p.workspaceId || !p.access.canWrite)
        return null;
    return <Composer key={`${p.actorScope}:${p.workspaceId}:${source.binding.session_id}:${source.binding.artifact_id}:${source.evidence_hash}`} {...p} workspaceId={p.workspaceId} current={life.capture}/>;
}
function Composer(p: {
    source: ObservedEvidence;
    actorScope: string;
    workspaceId: string;
    token: () => Promise<string | null | undefined>;
    access: WorkspaceEvidenceAccess;
    current: () => () => boolean;
}) {
    const { source } = p;
    const [selection, setSelection] = useState<ReportSelection>(() => ({ client_id: crypto.randomUUID(), artifact_id: source.binding.artifact_id, result_ref: source.binding.result_ref, evidence_hash: source.evidence_hash, cases: source.cases.slice(0, 200).map(c => ({ entry_id: c.entry_id, entries: c.entries.filter(e => e.kind === "grade").map(e => ({ entry_id: e.entry_id, include_text: false })) })) }));
    const [preview, setPreview] = useState<ReportSnapshot>(), [created, setCreated] = useState<ReportSnapshot>(), [busy, setBusy] = useState(false), [error, setError] = useState("");
    function change(next: ReportSelection) { setSelection({ ...next, client_id: crypto.randomUUID() }); setPreview(undefined); setCreated(undefined); setError(""); }
    async function submit(create: boolean) {
        const current = p.current();
        setBusy(true);
        setError("");
        try {
            const auth = await p.token();
            if (!current())
                return;
            const body = { ...selection, ...(create ? { preview_hash: preview?.content_hash } : {}) };
            const value = await vibeFetch<ReportSnapshot>(`/sessions/${source.binding.session_id}/report-snapshots${create ? "" : "/preview"}`, auth, { method: "POST", body: JSON.stringify(body) });
            if (!current())
                return;
            if (create)
                setCreated(value);
            else
                setPreview(value);
        }
        catch (e) {
            if (current())
                setError((e as Error).message);
        }
        finally {
            if (current())
                setBusy(false);
        }
    }
    return <details className="rounded-[var(--vibe-radius-md)] border p-4"><summary className="cursor-pointer font-medium">Create a selected workspace report</summary><div className="mt-3 space-y-4">
    <p className="text-sm">Choose cases and entries, then preview the exact copy. Defaults include scalar grades with neutral labels. Source names, messages, explanations and quotes stay private unless you include text.</p>
    <p className="text-sm">The created report is readable by workspace viewers. It persists independently after private source deletion. Public access requires a separate expiring link. Notes never change original grades.</p>
    <fieldset disabled={busy || !!created} className="space-y-3"><legend className="sr-only">Select report contents</legend>
      {source.cases.map((c, i) => {
            const selected = selection.cases.find(s => s.entry_id === c.entry_id);
            return <div key={c.entry_id} className="space-y-2 border-t pt-2"><label className="flex gap-2"><input type="checkbox" checked={!!selected} onChange={e => change({ ...selection, cases: e.target.checked ? [...selection.cases, { entry_id: c.entry_id, entries: [] }] : selection.cases.filter(s => s.entry_id !== c.entry_id) })}/>{c.source_index < 0 ? "Unbound participant evidence" : `Case ${i + 1}`} · {c.status}</label>
        {selected && c.entries.map((entry, j) => { const pick = selected.entries.find(e => e.entry_id === entry.entry_id); return <div key={entry.entry_id} className="ml-6 flex flex-wrap gap-3 text-sm"><label className="flex gap-2"><input type="checkbox" checked={!!pick} onChange={e => change({ ...selection, cases: selection.cases.map(s => s.entry_id !== c.entry_id ? s : { ...s, entries: e.target.checked ? [...s.entries, { entry_id: entry.entry_id, include_text: false }] : s.entries.filter(v => v.entry_id !== entry.entry_id) }) })}/>{entry.kind} · Entry {j + 1}</label>{pick && <label className="flex gap-2"><input type="checkbox" checked={pick.include_text} onChange={e => change({ ...selection, cases: selection.cases.map(s => s.entry_id !== c.entry_id ? s : { ...s, entries: s.entries.map(v => v.entry_id === entry.entry_id ? { ...v, include_text: e.target.checked } : v) }) })}/>Include source text (may contain private details)</label>}</div>; })}
      </div>;
        })}
    </fieldset><p className="text-sm text-muted-foreground">Text includes supported message, output, tool and grade explanation fields, including embedded quotes. Structured objects and configuration are excluded. Review the exact preview before creating.</p>
    {error && <p role="alert">{error}</p>}{!created && <VibeButton disabled={busy || !selection.cases.length} onClick={() => void submit(false)}>{busy ? "Preparing…" : "Preview selected copy"}</VibeButton>}
    {preview && <section aria-label="Exact report preview" className="space-y-4 border-t pt-4"><h3>Exact report preview</h3><ReportSnapshotView content={preview.content}/>{!created && <VibeButton disabled={busy} onClick={() => void submit(true)}>Create workspace report</VibeButton>}</section>}
    {created && <p role="status">Workspace report created. <Link className="underline" href={`/workspaces/${p.workspaceId}/reports/${created.id}`}>Open workspace report</Link></p>}
  </div></details>;
}
