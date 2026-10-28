"use client";
import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { VibeError, vibeFetch } from "@/lib/vibe";
import { canonicalBase, type CanonicalResult } from "@/lib/vibe-canonical";
import type { ProjectResultBinding } from "@/lib/vibe-result-evidence";
import type { WorkspaceEvidenceAccess } from "@/lib/use-vibe-workspace-access";
import { useVibeEvidenceOwner } from "@/lib/use-vibe-evidence-owner";
import { VibeButton } from "./vibe-button";
export function CanonicalResults(p: {
    sessionId: string;
    workspaceId?: string;
    actorScope: string;
    token: () => Promise<string | null | undefined>;
    access: WorkspaceEvidenceAccess;
    selected?: CanonicalResult;
    onRefreshSelected?: () => void;
    onSelectResult: (b: ProjectResultBinding) => void;
    onExport: (r: CanonicalResult) => void;
}) {
    const revoked = p.access.status === "signed_out" || p.access.status === "ready" && !p.access.canRead;
    const life = useVibeEvidenceOwner(p.actorScope, p.sessionId, p.workspaceId, revoked ? "revoked" : "active");
    const [state, setState] = useState<{
        owner: object;
        rows?: CanonicalResult[];
        denied?: boolean;
        error?: string;
    }>();
    const [attempt, retry] = useState(0);
    const token = useRef(p.token);
    useEffect(() => { token.current = p.token; }, [p.token]);
    if (state && state.owner !== life.owner)
        setState(undefined);
    useEffect(() => {
        if (!p.access.canRead || !p.workspaceId)
            return;
        let active = true;
        const current = life.capture();
        let timer: ReturnType<typeof setTimeout>;
        async function load() {
            try {
                const auth = await token.current();
                if (!active || !current())
                    return;
                const rows = await vibeFetch<CanonicalResult[]>(canonicalBase(p.sessionId), auth);
                if (!active || !current()) return;
                if (!Array.isArray(rows) || rows.some(row => row.binding.session_id !== p.sessionId || !["run", "eval_session"].includes(row.binding.result_ref.kind)))
                    throw Error("Canonical results could not be read.");
                setState({ owner: life.owner, rows });
                if (rows.some(row => ["queued", "running", "provisioning", "scoring", "aggregating", "pending"].includes(row.status)))
                    timer = setTimeout(load, 3000);
            }
            catch (e) {
                if (!active || !current()) return;
                const denied = e instanceof VibeError && ([401, 403, 404].includes(e.status || 0) || ["forbidden", "not_found", "unauthorized"].includes(e.code));
                setState(old => ({ owner: life.owner, rows: !denied && old?.owner === life.owner ? old.rows : undefined,
                    denied: denied || old?.owner === life.owner && old.denied, error: (e as Error).message }));
            }
        }
        void load();
        return () => { active = false; clearTimeout(timer); };
    }, [life, p.sessionId, p.workspaceId, p.access.canRead, attempt]);
    if (!p.workspaceId || !p.access.canRead)
        return null;
    const visible = state?.owner === life.owner ? state : undefined;
    const selected = !visible?.denied && p.selected?.binding.session_id === p.sessionId ? p.selected : undefined;
    return <section aria-label="Canonical results" className="min-w-0 space-y-3">
      {selected && <article aria-label="Canonical result detail" className="space-y-3"><h4>{selected.name}</h4><p role="status">Canonical status: {selected.status}</p>{selected.experiment && <section aria-label="Repeated sample results"><p>{selected.experiment.completed} completed / {selected.experiment.requested} requested · {selected.experiment.failed} failed or cancelled · {selected.experiment.pending} pending · {selected.experiment.scored} scored</p>{selected.experiment.aggregate?.overall && <p>Overall score: mean {selected.experiment.aggregate.overall.mean.toFixed(3)} · standard deviation {selected.experiment.aggregate.overall.std_dev.toFixed(3)} · n={selected.experiment.aggregate.overall.n}</p>}{selected.experiment.aggregate?.participants?.map((row, i) => row.overall && <p key={i}>{row.label}: mean {row.overall.mean.toFixed(3)} · standard deviation {row.overall.std_dev.toFixed(3)} · n={row.overall.n}</p>)}{!selected.experiment.aggregate && <p>Variance is unavailable until canonical aggregation completes.</p>}{selected.experiment.warnings.map((warning, i) => <p key={i} className="text-sm">{warning}</p>)}<details><summary>Full canonical statistics</summary><pre className="whitespace-pre-wrap break-words">{JSON.stringify(selected.experiment.aggregate, null, 2)}</pre></details></section>}<details><summary>Result identity</summary><p className="break-all">{selected.binding.result_ref.id}</p></details>{selected.participants.map(participant => <div key={participant.run_agent_id} className="space-y-2"><p>{participant.label} · {participant.adapter} · {participant.status}</p><p>{participant.cases.length} selected cases · {participant.events.length} persisted events</p><details><summary>Admitted case and snapshot identities</summary><p className="break-all">Snapshot {participant.snapshot_id}</p><ul>{participant.cases.map(c => <li key={c.source_case_id} className="break-all">{c.input_set_key} / {c.challenge_key} / {c.case_key} · {c.source_case_id}</li>)}</ul></details></div>)}<details><summary>Capture and export limits</summary><p>Historical execution settings, external application internals, model revisions and seeds may be unknown.</p>{selected.omissions.map(note => <p className="text-sm" key={note}>{note}</p>)}</details><VibeButton onClick={() => p.onExport(selected)}>Export canonical result</VibeButton>{selected.binding.result_ref.kind === "run" && selected.status === "completed" && <p><Link className="underline" href={`/workspaces/${p.workspaceId}/ci-setup`}>Review CI baseline setup</Link>. Explicitly choose this completed run and save the profile; publishing tests never promotes a baseline.</p>}</article>}
      {visible?.error && <p role="alert">{visible.error}</p>}
      <details aria-label="Application run history" className="space-y-3">
        <summary>Application run history</summary>
        <p>Live canonical execution, with workspace billing and separate external application fees.</p>
        {!visible?.rows && !visible?.error && <p role="status">Loading application runs…</p>}
        <VibeButton onClick={() => { retry(n => n + 1); p.onRefreshSelected?.(); }}>Refresh application runs</VibeButton>
        {visible?.rows?.length === 0 && <p>No application runs are bound to these published tests yet.</p>}
        <ul>{visible?.rows?.map(r => <li key={`${r.binding.result_ref.kind}:${r.binding.result_ref.id}:${r.binding.artifact_id}`}>
          <VibeButton onClick={() => { if (life.current()) p.onSelectResult(r.binding); }}>{r.binding.result_ref.kind === "eval_session" ? "Evaluation" : "Run"} {r.name} · {r.status}</VibeButton>
        </li>)}</ul>
      </details>
    </section>;
}
