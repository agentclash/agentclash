"use client";
import { useEffect, useRef, useState } from "react";
import { VibeError, vibeFetch, type Artifact, type Models, type Operation } from "@/lib/vibe";
import type { ProjectResultBinding } from "@/lib/vibe-result-evidence";
import { reportBase, type SDKReport } from "@/lib/vibe-reports";
import { useVibeEvidenceOwner } from "@/lib/use-vibe-evidence-owner";
import { SDKReportDetail } from "./sdk-report-detail";
import type { SDKEvidenceContext } from "./sdk-report-import";
import { VibeButton } from "./vibe-button";
export function SDKReports(props: SDKEvidenceContext & {
  selectedReport?: SDKReport; onSelectResult?: (binding: ProjectResultBinding, caseKey?: string) => void;
  revision?: number; models?: Models; inferenceAvailable?: boolean; refresh?: number;
  onPromoted?: (artifact: Artifact) => void; onRegraded?: (operation: Operation) => void;
  onExport?: (report: SDKReport) => void;
}) {
  const {actorScope,sessionId,workspaceId,token,refresh,access:{canRead,status}} = props;
  const [attempt, setAttempt] = useState(0);
  const accessRevoked = status === "signed_out" || status === "ready" && !canRead;
  const life = useVibeEvidenceOwner(actorScope, sessionId, workspaceId, accessRevoked ? "revoked" : "active");
  const tokenRef = useRef(token);
  useEffect(() => { tokenRef.current = token; }, [token]);
  const [state, setState] = useState<{ owner: object; reports?: SDKReport[]; denied?: boolean; error?: string }>();
  if (state && state.owner !== life.owner) setState(undefined);
  useEffect(() => {
    let active = true; const current = life.capture();
    if (!canRead || !workspaceId) return;
    void tokenRef.current().then(auth => vibeFetch<SDKReport[]>(reportBase(sessionId), auth)).then(reports => {
      if (!Array.isArray(reports) || reports.some(r => r.binding.session_id !== sessionId || r.binding.result_ref.kind !== "sdk_report" || r.binding.result_ref.id !== r.id)) throw Error("Report identities could not be read.");
      if (active && current()) setState({ owner: life.owner, reports });
    }).catch(e => {
      if (!active || !current()) return;
      // A failed history refresh must not unmount the current detail or lose an
      // in-flight receipt. Authoritative access/project loss invalidates cache.
      const revoked = e instanceof VibeError && ([401, 403, 404].includes(e.status || 0) || ["forbidden", "not_found", "unauthorized"].includes(e.code));
      setState(previous => ({ owner: life.owner, reports: !revoked && previous?.owner === life.owner ? previous.reports : undefined, denied: revoked || previous?.owner === life.owner && previous.denied, error: e.message }));
    });
    return () => { active = false; };
  }, [sessionId, workspaceId, canRead, status, life, attempt, refresh]);
  const unavailable = status === "signed_out" ? "Sign in to read your private SDK reports."
    : !workspaceId ? "Save to a workspace to keep imported reports here."
    : !canRead ? "SDK reports are unavailable until access to this workspace is confirmed." : undefined;
  const active = !unavailable && state?.owner === life.owner ? state : undefined;
  const selected = !unavailable && !active?.denied && props.selectedReport?.binding.session_id === sessionId && props.selectedReport.binding.result_ref.kind === "sdk_report" && props.selectedReport.id === props.selectedReport.binding.result_ref.id && props.selectedReport.binding.artifact_id === props.artifactId ? props.selectedReport : undefined;
  return <section aria-label="SDK reports" className="space-y-4">
    {selected && <SDKReportDetail key={`${props.actorScope}:${props.sessionId}:${selected.id}`} {...props} report={selected} />}
    {active?.error && <p role="alert">{active.error}</p>}
    <details aria-label="SDK report history" className="space-y-3">
      <summary>SDK report history</summary>
      {unavailable ? <p className="text-sm">{unavailable}</p> : <>
        <p>Imported, unverified application execution. Source grades remain separate from hosted checks and reviews.</p>
        {!active?.reports && !active?.error && <p role="status">Loading SDK reports…</p>}
        <VibeButton onClick={() => setAttempt(n => n + 1)}>Reload SDK reports</VibeButton>
        {active?.reports?.length === 0 && <p>No imported reports yet. Import historical evidence without running a check.</p>}
        <ul>{active?.reports?.map(report => <li key={report.id}>
          <VibeButton onClick={() => { if (life.current()) props.onSelectResult?.(report.binding); }}>Open report {report.external_report_id}</VibeButton>
          <span> · {report.cases.length} cases · {report.source_kind === "agent_result" ? "No imported grade" : "Imported grades"}</span>
        </li>)}</ul>
      </>}
    </details>
  </section>;
}
