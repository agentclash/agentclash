"use client";
import { useEffect, useRef, useState } from "react";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { CanonicalResult } from "@/lib/vibe-canonical";
import type { ProjectResultBinding } from "@/lib/vibe-result-evidence";
import { useVibeEvidenceOwner } from "@/lib/use-vibe-evidence-owner";
import { VibeButton } from "./vibe-button";
type Preview = {
    deployment_snapshot_ids: Record<string, string>;
    controlled_settings: Record<string, string>;
    participants: {
        snapshot_id: string;
        hash: string;
        settings: unknown;
    }[];
    case_ids: string[];
    child_runs: number;
    participant_samples: number;
    case_samples: number;
    estimated_usd: number | null;
    price_assumptions: string;
    limits: string[];
    changed_variables: string[];
};
type Admission = {
    mode?: "session" | "fleet";
    request: Record<string, unknown>;
    preview: Preview;
};
export function RepeatedExperiment(p: {
    source: CanonicalResult;
    workspaceId: string;
    actorScope: string;
    canWrite: boolean;
    token: () => Promise<string | null | undefined>;
    onCreated: (b: ProjectResultBinding) => void;
}) {
    const b = p.source.binding;
    const life = useVibeEvidenceOwner(p.actorScope, p.workspaceId, b.session_id, b.artifact_id, b.result_ref.kind, b.result_ref.id, p.canWrite ? "write" : "read");
    const [mode, setMode] = useState<"session" | "fleet">("session");
    const [count, setCount] = useState(3), [preview, setPreview] = useState<{
        owner: object;
        value: Preview;
        request: Record<string, unknown>;
    }>(), [pending, setPending] = useState<{
        owner: object;
        value: Admission;
    }>(), [busy, setBusy] = useState(false), [error, setError] = useState("");
    const token = useRef(p.token);
    useEffect(() => { token.current = p.token; }, [p.token]);
    const key = `vibe-experiment:${p.actorScope}:${b.session_id}:${b.artifact_id}:${b.result_ref.kind}:${b.result_ref.id}`;
    useEffect(() => { setPreview(undefined); setPending(undefined); setBusy(false); setError(""); try {
        const saved = localStorage.getItem(key);
        if (saved) {
            const value = JSON.parse(saved) as Admission;
            if (typeof value.request?.admission_request_id === "string" && value.request.workspace_id === p.workspaceId) {
                setPending({ owner: life.owner, value });
                if (value.mode === "fleet" || value.mode === "session") setMode(value.mode);
                const repetitions = (value.request.eval_session as { repetitions?: unknown } | undefined)?.repetitions;
                if (typeof repetitions === "number" && Number.isInteger(repetitions) && repetitions >= 2 && repetitions <= 100) setCount(repetitions);
            }
        }
    }
    catch {
        setError("The saved admission attempt could not be recovered.");
    } }, [life, key, p.workspaceId]);
    const active = preview?.owner === life.owner ? preview : undefined, uncertain = pending?.owner === life.owner ? pending.value : undefined;
    const participants = Array.from(new Map(p.source.participants.map(v => [v.deployment_id, v])).values());
    const first = participants[0], scope = first ? JSON.stringify([first.challenge_input_set_id, first.cases.map(c => c.source_case_id)]) : "";
    const supported = !!b.challenge_pack_version_id && !!first && participants.every(v => JSON.stringify([v.challenge_input_set_id, v.cases.map(c => c.source_case_id)]) === scope) && participants.every(v => v.cases.every(c => !!c.challenge_identity_id));
    async function prepare() {
        if (!supported || busy || uncertain || !p.canWrite || !first)
            return;
        const current = life.capture();
        setBusy(true);
        setError("");
        setPreview(undefined);
        const request = { workspace_id: p.workspaceId, challenge_pack_version_id: b.challenge_pack_version_id, challenge_input_set_id: first.challenge_input_set_id, challenge_identity_ids: Array.from(new Set(first.cases.map(c => c.challenge_identity_id))), deployment_snapshot_ids: Object.fromEntries(participants.map(v => [v.deployment_id, v.snapshot_id])), participants: participants.map(v => ({ agent_deployment_id: v.deployment_id, label: v.label })), execution_mode: participants.length === 1 ? "single_agent" : "comparison", name: `${p.source.name} · repeated experiment`, eval_session: { repetitions: count, schema_version: 1, routing_task_snapshot: { routing: { mode: participants.length === 1 ? "single_agent" : "comparison" }, task: { challenge_pack_version_id: b.challenge_pack_version_id, challenge_input_set_id: first.challenge_input_set_id } }, aggregation: { method: "mean", report_variance: true, confidence_interval: 0.95 } } };
        try {
            const auth = await token.current();
            if (!current())
                return;
            if (!auth)
                throw Error("Sign in to prepare a batch.");
            const value = await createApiClient(auth).post<Preview>(mode === "fleet" ? "/v1/eval-sets/controlled/preview" : "/v1/eval-sessions/preview", request);
            if (current())
                setPreview({ owner: life.owner, value, request });
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
    async function admit(saved?: Admission) {
        if (busy || !p.canWrite || (!saved && !active))
            return;
        const current = life.capture();
        setError("");
        let admission = saved;
        if (!admission && active)
            admission = { mode, preview: active.value, request: { ...active.request, admission_request_id: crypto.randomUUID(), deployment_snapshot_ids: active.value.deployment_snapshot_ids, controlled_settings: active.value.controlled_settings } };
        if (!admission)
            return;
        // Save before sending. If acknowledgement is lost, an explicit retry uses the
        // same canonical request identity, including after a browser reload.
        try {
            localStorage.setItem(key, JSON.stringify(admission));
        }
        catch {
            setError("The recovery receipt could not be saved. No batch was sent.");
            return;
        }
        setPending({ owner: life.owner, value: admission });
        setBusy(true);
        try {
            const auth = await token.current();
            if (!current())
                return;
            if (!auth)
                throw Error("Sign in to recover this batch.");
            const response = await createApiClient(auth).post<{
                eval_session?: {
                    id: string;
                };
                eval_set?: {
                    id: string;
                    status: string;
                };
                eval_session_ids?: string[];
                run_ids: string[];
            }>(admission.mode === "fleet" ? "/v1/eval-sets/controlled" : "/v1/eval-sessions", admission.request);
            if (!current())
                return;
            const childID = admission.mode === "fleet" ? response.eval_session_ids?.[0] : response.eval_session?.id;
            if (!childID) {
                if (admission.mode === "fleet" && response.eval_set && ["failed", "cancelled", "budget_exceeded"].includes(response.eval_set.status)) {
                    localStorage.removeItem(key);
                    setPending(undefined);
                    setPreview(undefined);
                    setError(`Fleet ${response.eval_set.status} before a child was admitted. You may prepare a new batch.`);
                    return;
                }
                throw Error("Admission acknowledgment is missing; retry this same batch.");
            }
            p.onCreated({ ...b, result_ref: { kind: "eval_session", id: childID } });
            localStorage.removeItem(key);
            setPending(undefined);
            setPreview(undefined);
        }
        catch (e) {
            if (!current())
                return;
            setError((e as Error).message);
            if (!saved && e instanceof ApiError && e.status >= 400 && e.status < 500 && ![401, 403, 404, 408, 429].includes(e.status)) {
                localStorage.removeItem(key);
                setPending(undefined);
                setPreview(undefined);
            }
        }
        finally {
            if (current())
                setBusy(false);
        }
    }
    if (!p.canWrite)
        return null;
    const quote = uncertain?.preview || active?.value;
    return <details className="space-y-3"><summary>Repeat these cases</summary><p>Use the same published cases, expectations and evaluator with the selected deployment snapshots. Each sample remains a separate canonical run.</p>{!supported ? <p>This result combines different case scopes. Select one run with a single fixed scope first.</p> : <><label className="block text-sm">Batch execution<select aria-label="Batch execution" value={mode} disabled={busy || !!uncertain} onChange={e => { setMode(e.target.value as "session" | "fleet"); setPreview(undefined); }} className="ml-2 rounded border bg-background p-2"><option value="session">Repeated evaluation</option><option value="fleet">Fleet sweep</option></select></label><label className="block text-sm">Repetitions<input aria-label="Repetitions" type="number" min={2} max={100} value={count} disabled={busy || !!uncertain} onChange={e => { setCount(Math.max(2, Math.min(100, Number(e.target.value) || 2))); setPreview(undefined); }} className="ml-2 w-20 rounded border bg-background p-2"/></label><p>{participants.length} deployment{participants.length === 1 ? "" : "s"} · {first.cases.length} fixed cases per sample. {participants.length > 1 ? "Deployment model/build/settings vary between participants; repetitions keep each participant fixed." : "Only repetition varies."}</p><VibeButton disabled={busy || !!uncertain} onClick={() => void prepare()}>Preview repeated batch</VibeButton>{quote && <section aria-label="Experiment preview" className="space-y-2"><p>{quote.child_runs} runs · {quote.participant_samples} participant samples · {quote.case_samples} case samples</p><p>{quote.estimated_usd === null ? "Total price unavailable" : `Estimated target cost: $${quote.estimated_usd.toFixed(4)}`}</p><p className="text-sm">{quote.price_assumptions}</p><p>Changed variables: {quote.changed_variables.join(", ")}. Seeds are not acknowledged.</p><details><summary>Fixed settings and limits</summary>{quote.limits.map(v => <p key={v}>{v}</p>)}{quote.participants.map(v => <pre key={v.snapshot_id} className="whitespace-pre-wrap break-words text-xs [overflow-wrap:anywhere]">{JSON.stringify(v.settings, null, 2)}</pre>)}</details>{uncertain ? <><p role="status">Admission acknowledgment may be missing. Recover this same batch before preparing another.</p><VibeButton disabled={busy} onClick={() => void admit(uncertain)}>Retry batch admission</VibeButton></> : <VibeButton disabled={busy} onClick={() => void admit()}>Run repeated batch</VibeButton>}</section>}{error && <p role="alert">{error}</p>}{busy && <p role="status">Checking canonical admission…</p>}</>}</details>;
}
