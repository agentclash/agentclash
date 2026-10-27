"use client";
import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { useAccessToken, useAuth } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import { vibeFetch } from "@/lib/vibe";
import { useVibeEvidenceOwner } from "@/lib/use-vibe-evidence-owner";
import { useVibeWorkspaceAccess } from "@/lib/use-vibe-workspace-access";
import type { ReportSnapshot, ReportNote, ReportLink } from "@/lib/report-snapshots";
import { Button } from "@/components/ui/button";
import { ReportSnapshotView } from "./report-snapshot-view";
export function WorkspaceReports({ workspaceId, reportId }: {
    workspaceId: string;
    reportId?: string;
}) {
    const { user, loading } = useAuth(), { getAccessToken } = useAccessToken();
    const { access, retry } = useVibeWorkspaceAccess(user?.id || "guest", workspaceId, getAccessToken, loading);
    const life = useVibeEvidenceOwner(user?.id, workspaceId, reportId, access.canRead ? "read" : "denied", access.canWrite ? "write" : "denied");
    return <WorkspaceReportBody key={`${user?.id}:${workspaceId}:${reportId}:${access.canRead}:${access.canWrite}`} workspaceId={workspaceId} reportId={reportId} access={access} token={getAccessToken} capture={life.capture} retryAccess={retry}/>;
}
function WorkspaceReportBody(p: {
    workspaceId: string;
    reportId?: string;
    access: ReturnType<typeof useVibeWorkspaceAccess>["access"];
    token: () => Promise<string | null | undefined>;
    capture: () => () => boolean;
    retryAccess: () => void;
}) {
    const base = `/workspaces/${p.workspaceId}/report-snapshots`, detail = `${base}/${p.reportId}`;
    const [rows, setRows] = useState<ReportSnapshot[]>(), [report, setReport] = useState<ReportSnapshot>(), [notes, setNotes] = useState<ReportNote[]>([]), [links, setLinks] = useState<ReportLink[]>([]), [error, setError] = useState(""), [attempt, setAttempt] = useState(0), [busy, setBusy] = useState(false);
    const [note, setNote] = useState(""), [target, setTarget] = useState("report"), [days, setDays] = useState("7");
    const noteID = useRef(crypto.randomUUID()), token = useRef(p.token);
    token.current = p.token;
    useEffect(() => {
        if (!p.access.canRead)
            return;
        const current = p.capture();
        let live = true;
        void (async () => {
            try {
                const auth = await token.current();
                if (!live || !current())
                    return;
                if (!p.reportId) {
                    const rows = await vibeFetch<ReportSnapshot[]>(base, auth);
                    if (live && current())
                        setRows(rows);
                    return;
                }
                const [report, notes, links] = await Promise.all([vibeFetch<ReportSnapshot>(detail, auth), vibeFetch<ReportNote[]>(`${detail}/notes`, auth), p.access.canWrite ? vibeFetch<ReportLink[]>(`${detail}/links`, auth) : Promise.resolve([])]);
                if (live && current()) {
                    setReport(report);
                    setNotes(notes);
                    setLinks(links);
                    setError("");
                }
            }
            catch (e) {
                if (live && current()) {
                    setReport(undefined);
                    setRows(undefined);
                    setNotes([]);
                    setLinks([]);
                    setError((e as Error).message);
                }
            }
        })();
        return () => { live = false; };
    }, [base, detail, p.reportId, p.access.canRead, p.access.canWrite, p.capture, attempt]);
    async function mutate(action: (auth: string) => Promise<unknown>, success?: () => void) { const current = p.capture(); setBusy(true); setError(""); try {
        const auth = await token.current();
        if (!current())
            return;
        if (!auth)
            throw Error("Sign in to continue.");
        await action(auth);
        if (current()) {
            success?.();
            setAttempt(n => n + 1);
        }
    }
    catch (e) {
        if (current())
            setError((e as Error).message);
    }
    finally {
        if (current())
            setBusy(false);
    } }
    if (!p.access.canRead)
        return <section className="space-y-3"><h1>Workspace reports</h1><p role="status">{p.access.status === "loading" ? "Checking workspace access…" : "Workspace access is unavailable."}</p>{p.access.status !== "loading" && <Button onClick={p.retryAccess}>Retry access</Button>}</section>;
    return <section className="mx-auto max-w-4xl space-y-6"><header><h1 className="text-xl font-semibold">Workspace reports</h1><p className="text-sm text-muted-foreground">Selected independent copies, readable by workspace viewers. Private source projects remain private.</p></header>
 {error && <p role="alert">{error} <Button variant="outline" onClick={() => setAttempt(n => n + 1)}>Retry</Button></p>}
 {!p.reportId ? <>{!rows && !error && <p role="status">Loading reports…</p>}{rows?.length === 0 && <p>No reports yet. Open a result in Vibe Evals and choose “Create a selected workspace report”.</p>}<ul className="space-y-2">{rows?.map(r => <li key={r.id}><Link className="block rounded-lg border p-4 hover:underline" href={`/workspaces/${p.workspaceId}/reports/${r.id}`}>Selected report · {new Date(r.created_at).toLocaleString()}<span className="block break-all text-xs text-muted-foreground">{r.id}</span></Link></li>)}</ul></> : <>
 <Link className="underline" href={`/workspaces/${p.workspaceId}/reports`}>All workspace reports</Link>{!report && !error && <p role="status">Loading report…</p>}
 {report && <><ReportSnapshotView content={report.content}/><section aria-label="Workspace annotations" className="space-y-3 border-t pt-4"><h2 className="font-semibold">Workspace annotations</h2><p className="text-sm text-muted-foreground">Append-only notes by members and admins. Original grades remain unchanged. Notes are not included in public links.</p>
 {notes.length === 0 && <p>No annotations yet.</p>}{notes.map(n => <article key={n.id} className="rounded border p-3"><p className="text-xs break-all">{n.author_id} · {new Date(n.created_at).toLocaleString()} · {n.target_id === "report" ? "Report" : report.content.cases.flatMap(c => [{ id: c.entry_id, label: c.label }, ...c.entries.map(e => ({ id: e.entry_id, label: `${c.label} · ${e.label}` }))]).find(t => t.id === n.target_id)?.label || "Selected evidence"}</p><p className="whitespace-pre-wrap break-words [overflow-wrap:anywhere]">{n.text}</p></article>)}
 {p.access.canWrite && <form className="space-y-3" onSubmit={e => { e.preventDefault(); void mutate(auth => vibeFetch(`${detail}/notes`, auth, { method: "POST", body: JSON.stringify({ client_id: noteID.current, target_id: target, text: note }) }), () => { setNote(""); noteID.current = crypto.randomUUID(); }); }}><label className="block">Annotate<select className="block w-full rounded border bg-background p-2" value={target} onChange={e => { setTarget(e.target.value); noteID.current = crypto.randomUUID(); }}><option value="report">Whole report</option>{report.content.cases.flatMap(c => [<option key={c.entry_id} value={c.entry_id}>{c.label}</option>, ...c.entries.map(e => <option key={e.entry_id} value={e.entry_id}>{c.label} · {e.label}</option>)])}</select></label><label className="block">New annotation<textarea required maxLength={4096} disabled={busy} className="block min-h-24 w-full rounded border bg-background p-2" value={note} onChange={e => { setNote(e.target.value); noteID.current = crypto.randomUUID(); }}/></label><Button type="submit" disabled={busy || !note.trim()}>Append annotation</Button></form>}</section>
 {p.access.canWrite && <section aria-label="Public report links" className="space-y-3 border-t pt-4"><h2 className="font-semibold">Public report link</h2><p className="text-sm text-muted-foreground">No public access until you create a link. Anyone with an active link can read this exact selected copy. Links are unindexed, expire and can be revoked. Source deletion does not revoke them.</p><label className="block">New link lifetime<select className="ml-3 rounded border bg-background p-2" value={days} onChange={e => setDays(e.target.value)}><option value="1">1 day</option><option value="7">7 days</option><option value="30">30 days</option></select></label><Button disabled={busy} onClick={() => void mutate(auth => createApiClient(auth).post("/v1/share-links", { resource_type: "report_snapshot", resource_id: report.id, search_indexing: false, expires_at: new Date(Date.now() + Number(days) * 86400000).toISOString() }))}>Create expiring public link</Button><p className="text-xs text-muted-foreground">An existing active link keeps its original expiry. Revoke it before creating a replacement.</p>
 {links.map(link => { const active = link.is_active && !link.revoked_at && new Date(link.expires_at).getTime() > Date.now(); return <div key={link.id} className="space-y-2 rounded border p-3"><p>{link.revoked_at ? "Revoked" : active ? "Active" : "Expired"} · Expires {new Date(link.expires_at).toLocaleString()}</p>{active && <><Link className="block break-all underline" target="_blank" rel="noreferrer" href={`/share/${link.key}`}>Open public report</Link><Button variant="outline" disabled={busy} onClick={() => void navigator.clipboard.writeText(`${window.location.origin}/share/${link.key}`).catch(() => setError("Could not copy the link. Open the public report and copy its address."))}>Copy public link</Button><Button variant="outline" disabled={busy} onClick={() => void mutate(auth => createApiClient(auth).del(`/v1/share-links/${link.id}`))}>Revoke link</Button></>}</div>; })}</section>}</>}
 </>}
 </section>;
}
