"use client";
import { useEffect, useRef, useState } from "react";
import { connectorClient, type ConnectorBinding, type ConnectorConfig, type ConnectorPublication, type DeploymentAdapter } from "@/lib/connector-client";
import type { ProjectResultBinding } from "@/lib/vibe-result-evidence";
import type { WorkspaceEvidenceAccess } from "@/lib/use-vibe-workspace-access";
import { VibeButton } from "./vibe-button";
export type ConnectorSettingsProps = {
    sessionId: string;
    workspaceId: string;
    artifactId: string;
    actorScope: string;
    access: WorkspaceEvidenceAccess;
    token: () => Promise<string | null | undefined>;
    publication?: ConnectorPublication;
    onRunCreated: (binding: ProjectResultBinding) => void;
};
export function ConnectorSettings(p: ConnectorSettingsProps) {
    const { token, workspaceId, access } = p;
    const [source, setSource] = useState<DeploymentAdapter>("native");
    const [items, setItems] = useState<ConnectorBinding[]>([]), [selected, setSelected] = useState<ConnectorBinding | null>(null);
    const [name, setName] = useState("Application"), [endpoint, setEndpoint] = useState(""), [secret, setSecret] = useState("");
    const [auth, setAuth] = useState<"bearer" | "api_key">("bearer"), [input, setInput] = useState('{"question":"/prompt"}'), [pointer, setPointer] = useState("/answer");
    const [busy, setBusy] = useState(false), [message, setMessage] = useState(""), [reviewed, setReviewed] = useState(false), [uncertain, setUncertain] = useState(false);
    const generation = `${p.actorScope}:${p.workspaceId}:${p.sessionId}:${p.artifactId}:${p.access.status}:${p.access.canRead}:${p.access.canWrite}:${p.access.canAdmin}`;
    const current = useRef({ key: generation, epoch: 0 });
    if (current.current.key !== generation)
        current.current = { key: generation, epoch: current.current.epoch + 1 };
    const epoch = current.current.epoch;
    const mounted = useRef(true), locked = useRef(false);
    useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
    useEffect(() => {
        setItems([]);
        setSelected(null);
        setReviewed(false);
        setMessage("");
        setUncertain(false);
        setBusy(false);
        locked.current = false;
        if (access.status !== "ready" || !access.canRead || !token || !workspaceId)
            return;
        const controller = new AbortController();
        token().then(auth => { if (controller.signal.aborted || current.current.epoch !== epoch)
            return; if (!auth)
            throw Error("Sign in to this workspace."); return connectorClient(auth, workspaceId).list(controller.signal); }).then(result => { if (result && current.current.epoch === epoch && !controller.signal.aborted)
            setItems(result.connectors); }).catch(() => { if (!controller.signal.aborted && current.current.epoch === epoch)
            setMessage("Connectors could not be loaded. Reopen settings to try again."); });
        return () => controller.abort();
    }, [epoch, generation, access.status, access.canRead, token, workspaceId]);
    const active = () => mounted.current && current.current.epoch === epoch;
    async function perform(action: "save" | "approve" | "revoke" | "run") {
        if (locked.current || !p.token || p.access.status !== "ready")
            return;
        if (action === "run" ? !p.access.canWrite : !p.access.canAdmin)
            return;
        locked.current = true;
        setBusy(true);
        setMessage("");
        try {
            const authToken = await p.token();
            if (!active())
                return;
            if (!authToken)
                throw Error("Sign in to this workspace.");
            const client = connectorClient(authToken, p.workspaceId);
            if (action === "save") {
                if (source === "native")
                    return;
                let fields: Record<string, string> | undefined;
                if (source === "http_json") {
                    const parsed: unknown = JSON.parse(input);
                    if (!parsed || Array.isArray(parsed) || typeof parsed !== "object" || Object.values(parsed).some(v => typeof v !== "string"))
                        throw new Error("Map JSON field names to JSON pointer strings.");
                    fields = parsed as Record<string, string>;
                }
                const config: ConnectorConfig = { adapter: source, endpoint_url: endpoint, secret_key: secret, auth, input_fields: fields, response_pointer: source === "http_json" ? pointer : "", timeout_seconds: 60 };
                const binding = await client.save(name, config);
                if (!active())
                    return;
                if (binding.workspace_id !== p.workspaceId)
                    throw Error("Connector workspace identity changed.");
                setSelected(binding);
                setItems(old => [...old, binding]);
                setReviewed(false);
                setMessage("Saved. Review the target and credential binding before approval.");
            }
            else if (selected) {
                if (action === "approve") {
                    if (!reviewed)
                        return;
                    const binding = await client.approve(selected);
                    if (!active())
                        return;
                    if (binding.workspace_id !== p.workspaceId || binding.snapshot_id !== selected.snapshot_id)
                        throw Error("Connector approval identity changed.");
                    setSelected(binding);
                    setItems(old => old.map(item => item.snapshot_id === binding.snapshot_id ? binding : item));
                    setMessage("Connector approved. Running tests is a separate action.");
                }
                if (action === "revoke") {
                    await client.revoke(selected);
                    if (!active())
                        return;
                    setSelected({ ...selected, approved: false });
                    setItems(old => old.map(item => item.snapshot_id === selected.snapshot_id ? { ...item, approved: false } : item));
                    setReviewed(false);
                    setMessage("Approval revoked. Future dispatches are blocked.");
                }
                if (action === "run" && selected.approved && p.publication?.artifact_id === p.artifactId && !uncertain) {
                    const result = await client.run(selected, p.publication);
                    if (!active())
                        return;
                    p.onRunCreated({ session_id: p.sessionId, artifact_id: p.artifactId, result_ref: { kind: "run", id: result.id }, challenge_pack_version_id: p.publication.challenge_pack_version_id });
                    setMessage("Application run created.");
                }
            }
        }
        catch (error) {
            if (active()) {
                setMessage(error instanceof Error ? error.message : "Connector action failed.");
                if (action === "run") {
                    setUncertain(true);
                    setMessage("Run creation was not confirmed. Check run history before starting another run.");
                }
            }
        }
        finally {
            if (active()) {
                locked.current = false;
                setBusy(false);
            }
        }
    }
    if (!p.workspaceId || p.access.status === "signed_out")
        return <p className="text-sm">Sign in to a workspace to connect an application. Guest projects support text only.</p>;
    if (p.access.status !== "ready")
        return <p role="status">{p.access.status === "loading" ? "Checking workspace access…" : "Workspace access is unavailable. Reopen settings to try again."}</p>;
    if (!p.access.canRead)
        return <p>Workspace access is required to view connectors.</p>;
    const inputClass = "w-full min-w-0 rounded-[var(--vibe-radius-sm)] border border-builder-border bg-builder-bg px-3 py-2";
    return <section aria-label="Application source" className="min-w-0 space-y-4">
  <label className="block text-sm">Application source<select className={inputClass} value={source} onChange={e => { setSource(e.target.value as DeploymentAdapter); setSelected(null); setReviewed(false); }} disabled={busy}><option value="native">Native AgentClash agent</option><option value="http_json">HTTP JSON application</option><option value="hosted">Hosted callback application</option></select></label>
  {source === "native" ? <p className="text-sm">Use the native agent and model settings for your AgentClash deployment.</p> : <>
   <label className="block text-sm">Saved connector<select className={inputClass} value={selected?.snapshot_id ?? ""} disabled={busy} onChange={e => { const b = items.find(item => item.snapshot_id === e.target.value) ?? null; setSelected(b); setReviewed(false); }}><option value="">Choose a connector</option>{items.filter(item => item.config.adapter === source).map(item => <option key={item.snapshot_id} value={item.snapshot_id}>{item.snapshot_id} · {item.config.endpoint_url} ({item.approved ? "approved" : "needs approval"})</option>)}</select></label>
   {p.access.canAdmin ? <details><summary className="cursor-pointer text-sm">Configure a new connector</summary><form className="mt-3 space-y-3" onSubmit={e => { e.preventDefault(); void perform("save"); }}>
    <label className="block text-sm">Name<input required className={inputClass} maxLength={200} value={name} onChange={e => setName(e.target.value)} disabled={busy}/></label>
    <label className="block text-sm">HTTPS endpoint<input required type="url" className={inputClass} placeholder="https://app.example.com/invoke" value={endpoint} onChange={e => setEndpoint(e.target.value)} disabled={busy}/></label>
    <label className="block text-sm">Workspace secret key<input required className={inputClass} placeholder="APP_TOKEN" autoComplete="off" value={secret} onChange={e => setSecret(e.target.value)} disabled={busy}/></label><p className="text-sm">Add the credential in workspace Secrets first. Enter its key here, never the secret value.</p>
    <label className="block text-sm">Authentication<select className={inputClass} value={auth} onChange={e => setAuth(e.target.value as typeof auth)} disabled={busy}><option value="bearer">Bearer token</option><option value="api_key">X-API-Key header</option></select></label>
    {source === "http_json" ? <><label className="block text-sm">JSON input field mapping<textarea required className={inputClass} rows={3} value={input} onChange={e => setInput(e.target.value)} disabled={busy}/></label><p className="text-sm">Map each request field to a JSON pointer in the published test payload, such as /prompt.</p><label className="block text-sm">Response JSON pointer<input className={inputClass} value={pointer} onChange={e => setPointer(e.target.value)} disabled={busy}/></label></> : <p className="text-sm">The application receives /agentclash/runs requests and sends signed callbacks. Its original deadline applies to the whole run.</p>}
    <p className="text-sm">Requests and responses are limited to 1 MiB. This connector uses a 60-second deadline and never follows redirects.</p>
    <VibeButton type="submit" disabled={busy}>Save connector</VibeButton>
   </form></details> : <p className="text-sm">A workspace admin must configure and approve connectors.</p>}
   {selected && <div className="min-w-0 space-y-3 text-sm">
    <section aria-label="Selected connector contract" className="min-w-0 space-y-3 rounded-[var(--vibe-radius-md)] border border-builder-border p-3">
     <h3>Selected connector contract</h3>
     <dl className="min-w-0 space-y-2">
      <div><dt className="text-xs">Snapshot</dt><dd className="break-all font-mono">{selected.snapshot_id}</dd></div>
      <div><dt className="text-xs">HTTPS endpoint</dt><dd className="break-all">{selected.config.endpoint_url}</dd></div>
      <div><dt className="text-xs">Target origin</dt><dd className="break-all">{selected.target_origin}</dd></div>
      <div><dt className="text-xs">Authentication</dt><dd>{selected.config.auth === "bearer" ? "Bearer token" : "X-API-Key header"}</dd></div>
      <div><dt className="text-xs">Workspace credential reference</dt><dd className="break-all">{selected.config.secret_key}, revision {selected.secret_revision}</dd></div>
      {selected.config.adapter === "http_json" ? <>
       <div><dt className="text-xs">JSON input field mapping</dt><dd><pre className="whitespace-pre-wrap break-all">{JSON.stringify(selected.config.input_fields ?? {}, null, 2)}</pre></dd></div>
       <div><dt className="text-xs">Literal JSON fields</dt><dd><pre className="whitespace-pre-wrap break-all">{JSON.stringify(selected.config.literal_json ?? {}, null, 2)}</pre></dd></div>
       <div><dt className="text-xs">Response JSON pointer</dt><dd className="break-all font-mono">{selected.config.response_pointer || "(whole response)"}</dd></div>
      </> : <div><dt className="text-xs">Invocation protocol</dt><dd>POST /agentclash/runs with signed callbacks. JSON field mapping, literals and response pointers do not apply.</dd></div>}
      <div><dt className="text-xs">Timeout</dt><dd>{selected.config.timeout_seconds} seconds</dd></div>
     </dl>
     <p>{selected.approved ? "Approved" : "Approval required"}. Secret values are never shown here.</p>
    </section>
    {p.access.canAdmin && !selected.approved && <><label className="flex items-start gap-2"><input type="checkbox" checked={reviewed} onChange={e => setReviewed(e.target.checked)} disabled={busy}/><span>I approve sending this workspace credential and mapped test inputs to this target.</span></label><VibeButton disabled={busy || !reviewed} onClick={() => void perform("approve")}>Approve connector</VibeButton></>}
    {p.access.canAdmin && selected.approved && <VibeButton disabled={busy} onClick={() => void perform("revoke")}>Revoke approval</VibeButton>}
    {selected.approved && p.access.canWrite && <><p>Application run charges and workspace spend limits apply. External application or provider fees are separate and excluded from the platform maximum. Cancellation stops local waiting; remote cancellation is not confirmed.</p>{p.publication?.artifact_id === p.artifactId ? <><p>{p.publication.case_count} published cases selected.</p><VibeButton disabled={busy || uncertain} onClick={() => void perform("run")}>Run published tests</VibeButton></> : <p>Select an exact published test version before running this connector.</p>}</>}
   </div>}
  </>}
  {message && <p role="status" className="break-words text-sm">{message}</p>}
 </section>;
}
