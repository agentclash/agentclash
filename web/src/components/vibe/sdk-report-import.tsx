"use client";
import { useEffect, useRef, useState } from "react";
import { vibeFetch } from "@/lib/vibe";
import { loadArtifactCaseMapping, reportBase, reportSourceCases, unknownCapture, unknownFingerprints, type ArtifactCaseMapping, type SDKReport, type SDKReportImport } from "@/lib/vibe-reports";
import type { ProjectResultBinding } from "@/lib/vibe-result-evidence";
import type { WorkspaceEvidenceAccess } from "@/lib/use-vibe-workspace-access";
import { useVibeEvidenceOwner } from "@/lib/use-vibe-evidence-owner";
import { VibeButton } from "./vibe-button";
export type SDKEvidenceContext = {
  sessionId: string; workspaceId?: string; artifactId: string; actorScope: string;
  access: WorkspaceEvidenceAccess; token: () => Promise<string | null | undefined>;
};
export function SDKReportImportForm(props: SDKEvidenceContext & { onImported: (binding: ProjectResultBinding) => void }) {
  const lifetime = useVibeEvidenceOwner(props.actorScope, props.sessionId, props.workspaceId, props.artifactId);
  // A keyed inner form prevents draft metadata crossing projects or identities.
  return <SDKImportFields key={JSON.stringify([props.actorScope, props.sessionId, props.workspaceId, props.artifactId])} {...props} parentCurrent={lifetime.current} />;
}
function SDKImportFields({ sessionId, workspaceId, artifactId, actorScope, access, token, onImported, parentCurrent }: SDKEvidenceContext & { onImported: (binding: ProjectResultBinding) => void; parentCurrent: () => boolean }) {
  const life = useVibeEvidenceOwner(actorScope, sessionId, workspaceId, artifactId);
  const tokenRef = useRef(token);
  useEffect(() => { tokenRef.current = token; }, [token]);
  const [kind, setKind] = useState<SDKReportImport["source_kind"]>("eval_report");
  const [source, setSource] = useState("");
  const [cases, setCases] = useState<{ id: string; label: string; target: string }[]>();
  const [mapping, setMapping] = useState<ArtifactCaseMapping[]>();
  const [mappingAttempt, setMappingAttempt] = useState(0);
  const [version, setVersion] = useState("");
  const [revision, setRevision] = useState("");
  const [fingerprints, setFingerprints] = useState(unknownFingerprints);
  const [capture, setCapture] = useState(unknownCapture);
  const fileRead = useRef(0);
  const [standaloneID, setStandaloneID] = useState(() => crypto.randomUUID());
  const [request, setRequest] = useState<SDKReportImport>();
  const [busy, setBusy] = useState(false), [error, setError] = useState(""), [notice, setNotice] = useState("");
  useEffect(() => {
    let live = true; const current = life.capture();
    if (!access.canRead || !workspaceId) return;
    void tokenRef.current().then(auth => loadArtifactCaseMapping(sessionId, artifactId, auth)).then(rows => {
      if (live && current()) setMapping(rows);
    }).catch(e => { if (live && current()) setError(e.message); });
    return () => { live = false; };
  }, [sessionId, artifactId, workspaceId, access.canRead, mappingAttempt, life]);
  if (access.status === "signed_out") return <p className="text-sm">Sign in and save this project to a workspace to import an SDK report.</p>;
  if (!workspaceId) return <p>Save this project to a workspace before importing historical evidence.</p>;
  if (access.status !== "ready") return <p role="status">{access.status === "loading" ? "Checking workspace access…" : "Workspace access is unavailable. Retry the access check to import."}</p>;
  if (!access.canWrite) return <p>Your workspace role can read evidence. Importing requires write access.</p>;
  const tuples = Array.from(new Map((mapping || []).flatMap(r => r.cases).map(c => [JSON.stringify([c.input_set_key, c.challenge_key, c.case_key]), c])).entries());
  const published = (mapping || []).flatMap(r => r.published);
  const versions = Array.from(new Set(published.map(r => r.challenge_pack_version_id)));
  const preview = (text: string, sourceKind = kind) => {
    fileRead.current++; setSource(text); setCases(undefined); setNotice("");
    try { if (new TextEncoder().encode(text).length > 1024 * 1024) throw Error("Report files must be at most 1 MiB."); setCases(reportSourceCases(text, sourceKind).map(c => ({ ...c, target: "" }))); setError(""); }
    catch (e) { setError((e as Error).message); }
  };
  return <section aria-label="Import SDK report" className="space-y-4">
    <h3>Import an SDK report</h3>
    <p>Keep a v1 eval report or standalone agent result with this test version. Importing starts no runs and uses no inference credits. Original bytes and imported grades stay unchanged.</p>
    <form className="space-y-3" onSubmit={async e => {
      e.preventDefault(); if (busy || !cases || !mapping) return;
      const bound = life.capture(); const current = () => bound() && parentCurrent();
      let body = request;
      if (!body) {
        const bindings = cases.map(c => {
          const tuple = tuples.find(([key]) => key === c.target)?.[1];
          if (!tuple || !c.id.trim()) throw Error("Map every source test explicitly.");
          const ids = Array.from(new Set(published.filter(p => p.challenge_pack_version_id === version && p.case_key === tuple.case_key && p.challenge_key === tuple.challenge_key && p.input_set_key === tuple.input_set_key).map(p => p.published_case_id)));
          return { source_case_id: c.id, ...tuple, ...(version && ids.length === 1 ? { published_case_id: ids[0] } : {}) };
        });
        body = { envelope_version: 1, source_kind: kind, source_schema_version: 1, source_json: source, ...(kind === "agent_result" ? { external_report_id: standaloneID } : {}), metadata: { artifact_id: artifactId, ...(version ? { challenge_pack_version_id: version } : {}), application_revision: revision.trim() || null, fingerprints, capture_scope: capture, cases: bindings } };
        setRequest(body);
      }
      setBusy(true); setError("");
      try {
        const auth = await token(); if (!current()) return;
        const accepted = await vibeFetch<SDKReport>(reportBase(sessionId), auth, { method: "POST", body: JSON.stringify(body) });
        if (!current()) return;
        if (accepted.binding.session_id !== sessionId || accepted.binding.artifact_id !== artifactId || accepted.binding.result_ref.kind !== "sdk_report" || accepted.binding.result_ref.id !== accepted.id) throw Error("The server returned a different report identity.");
        setNotice("Report imported. Its grades and settings are uploader claims, not verified application execution.");
        setRequest(undefined); setStandaloneID(crypto.randomUUID()); onImported(accepted.binding);
      } catch (e) { if (current()) setError((e as Error).message); }
      finally { if (current()) setBusy(false); }
    }}>
      <fieldset disabled={busy || !!request} className="space-y-3">
        <label className="block">Source format<select aria-label="SDK source format" value={kind} onChange={e => { const next = e.target.value as typeof kind; fileRead.current++; setKind(next); if (source) preview(source, next); }}><option value="eval_report">Eval report v1</option><option value="agent_result">Standalone agent result</option></select></label>
        <label className="block">JSON file<input type="file" accept="application/json,.json" onChange={async e => {
          const file = e.target.files?.[0]; if (!file) return; const current = life.capture(), reading = ++fileRead.current;
          try { if (file.size > 1024 * 1024) throw Error("Report files must be at most 1 MiB."); const bytes = await file.arrayBuffer(); if (current() && reading === fileRead.current) preview(new TextDecoder("utf-8", { fatal: true, ignoreBOM: true }).decode(bytes)); }
          catch (e) { if (current() && reading === fileRead.current) setError((e as Error).message); }
        }} /></label>
        <label className="block">Original JSON<textarea aria-label="SDK original JSON" className="vibe-textarea" value={source} onChange={e => { fileRead.current++; setSource(e.target.value); setCases(undefined); }} /></label>
        <VibeButton disabled={!source} onClick={() => preview(source)}>Read source tests</VibeButton>
        {cases && <fieldset><legend>Bind each source test to this test version</legend>
          {cases.length === 0 && <p>This report has no tests. Its diagnostic summary will be retained.</p>}
          {cases.map((c, index) => <div key={index} className="space-y-2">
            {kind === "agent_result" ? <label>Source test ID<input aria-label="Standalone source test ID" value={c.id} onChange={e => setCases(old => old?.map((item, i) => i === index ? { ...item, id: e.target.value } : item))} /></label> : <p>{c.label}</p>}
            <label>Test<select aria-label={`Test for ${c.label}`} value={c.target} onChange={e => setCases(old => old?.map((item, i) => i === index ? { ...item, target: e.target.value } : item))}>
              <option value="">Choose the exact test</option>{tuples.map(([key, tuple]) => <option key={key} value={key}>{[tuple.input_set_key, tuple.challenge_key, tuple.case_key].filter(Boolean).join(" / ")}</option>)}
            </select></label>
          </div>)}
        </fieldset>}
        <label className="block">Published version claim<select aria-label="Published version claim" value={version} onChange={e => setVersion(e.target.value)}><option value="">No published version claim</option>{versions.map(v => <option key={v} value={v}>{v}</option>)}</select></label>
        <details><summary>Declared application settings and capture</summary><p>Unknown stays unknown. These are uploaded claims, even when a fingerprint matches another result.</p>
          <label>Application revision<input value={revision} onChange={e => setRevision(e.target.value)} /></label>
          {(Object.keys(fingerprints) as (keyof typeof fingerprints)[]).map(key => <label className="block" key={key}>{key} fingerprint<input value={fingerprints[key] || ""} onChange={e => setFingerprints(old => ({ ...old, [key]: e.target.value || null }))} /></label>)}
          {(["messages", "tools", "retrieval"] as const).map(key => <div key={key}><label>{key} capture<select value={capture[key].scope} onChange={e => setCapture(old => ({ ...old, [key]: { ...old[key], scope: e.target.value as typeof old[typeof key]["scope"] } }))}>{["unknown", "partial", "complete"].map(v => <option key={v}>{v}</option>)}</select></label><label>{key} instrumentation<select value={capture[key].instrumentation} onChange={e => setCapture(old => ({ ...old, [key]: { ...old[key], instrumentation: e.target.value as typeof old[typeof key]["instrumentation"] } }))}>{["unknown", "not_instrumented", "instrumented"].map(v => <option key={v}>{v}</option>)}</select></label></div>)}
        </details>
      </fieldset>
      {!mapping && <VibeButton disabled={busy} onClick={() => { setError(""); setMapping(undefined); setMappingAttempt(n => n + 1); }}>Retry test mapping</VibeButton>}
      <VibeButton type="submit" variant="primary" loading={busy} disabled={busy || !mapping || !cases || cases.some(c => !c.id.trim() || !c.target)}>{request ? "Retry this report import" : "Import historical report"}</VibeButton>
      {request && <p>The import is unconfirmed. Retry sends the same bytes and metadata. <VibeButton disabled={busy} onClick={() => { setRequest(undefined); setError(""); }}>Edit import metadata</VibeButton></p>}
    </form>
    {error && <p role="alert">{error}</p>}{notice && <p role="status">{notice}</p>}
  </section>;
}
