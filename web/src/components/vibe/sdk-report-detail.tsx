"use client";
import { useEffect, useRef, useState } from "react";
import { VibeError, vibeFetch, vibeBytes, type Artifact, type CaseResult, type Models, type Operation, type RunQuote } from "@/lib/vibe";
import { reportBase, loadSDKCase, sdkExportValue, type SDKReport } from "@/lib/vibe-reports";
import { downloadBytes } from "@/lib/vibe-export";
import type { GradeReview, GradeReviewInput } from "@/lib/vibe-result-evidence";
import { useVibeEvidenceOwner } from "@/lib/use-vibe-evidence-owner";
import type { SDKEvidenceContext } from "./sdk-report-import";
import { VibeButton } from "./vibe-button";

type RegradeRequest = { client_id: string; revision: number; kind: "retest"; purpose: "regrade"; artifact_id: string; models: Models; regrade_source_result_ref: SDKReport["binding"]["result_ref"]; source_evidence_hash: string; run_quote_id?: string };
// These endpoint faults occur before inserting a receipt (or roll back its
// transaction). Existing exact receipts return before revision/evidence checks.
// An auth/transport failure or idempotency conflict does not establish that the
// original write was rejected, so it must retain its captured retry identity.
function rejectedSDKWrite(error: unknown, kind: "review" | "promotion") {
  if (!(error instanceof VibeError)) return false;
  const common = ["invalid_request", "evidence_changed", "attachment_limit", "document_limit"];
  return common.includes(error.code) || kind === "promotion" && ["revision_conflict", "unsupported_capability", "operation_running"].includes(error.code);
}
export function SDKReportDetail({ report, sessionId, actorScope, workspaceId, access, token, revision, models, inferenceAvailable, onPromoted, onRegraded, onExport }: SDKEvidenceContext & {
  report: SDKReport; revision?: number; models?: Models; inferenceAvailable?: boolean;
  onPromoted?: (artifact: Artifact) => void; onRegraded?: (operation: Operation) => void; onExport?: (report: SDKReport) => void;
}) {
  const life = useVibeEvidenceOwner(actorScope, sessionId, workspaceId, report.id);
  const tokenRef = useRef(token);
  useEffect(() => { tokenRef.current = token; }, [token]);
  const [sourceID, setSourceID] = useState("");
  const [evidence, setEvidence] = useState<CaseResult>();
  const [reviews, setReviews] = useState<GradeReview[]>([]);
  const [rawSource, setRawSource] = useState<unknown>();
  const [reason, setReason] = useState("");
  const [reviewRequest, setReviewRequest] = useState<GradeReviewInput>();
  const [promotion, setPromotion] = useState<{client_id:string;revision:number;source_case_id:string;source_evidence_hash:string;expected:string;reviewed:true}>();
  const [reviewed, setReviewed] = useState(false);
  const [reviewRejected, setReviewRejected] = useState(false), [promotionRejected, setPromotionRejected] = useState(false);
  const [regrade, setRegrade] = useState<{ request: RegradeRequest; quote: RunQuote }>();
  const [busy, setBusy] = useState(false), [error, setError] = useState(""), [notice, setNotice] = useState("");
  const [attempt, setAttempt] = useState(0);
  const base = `${reportBase(sessionId)}/${report.id}`;
  useEffect(() => {
    let active = true; const current = life.capture();
    void tokenRef.current().then(auth => vibeFetch<GradeReview[]>(`/sessions/${sessionId}/grade-reviews?result_id=${report.id}`, auth)).then(rows => { if (active && current()) setReviews(rows.filter(r => r.result_ref.kind === "sdk_report")); }).catch(e => { if (active && current()) setError(e.message); });
    return () => { active = false; };
  }, [report.id, sessionId, life, attempt]);
  useEffect(() => {
    let active = true; const current = life.capture();
    const binding = report.cases.find(c => c.source_case_id === sourceID);
    if (binding) void tokenRef.current().then(auth => loadSDKCase(sessionId, report, binding, auth)).then(c => { if (active && current()) setEvidence(c); }).catch(e => { if (active && current()) setError(e.message); });
    return () => { active = false; };
  }, [sourceID, report, sessionId, life, attempt]);
  async function action(work: (auth: string | null | undefined, current: () => boolean) => Promise<void>, onError?: (error: unknown) => void) {
    const current = life.capture();
    if (busy || !current()) return; setBusy(true); setError(""); setNotice("");
    try { const auth = await token(); if (current()) await work(auth, current); }
    catch (e) { if (current()) { onError?.(e); setError((e as Error).message); } }
    finally { if (current()) setBusy(false); }
  }
  async function original(auth: string | null | undefined) {
    const bytes = await vibeBytes(`${base}/original?hash=${report.content_sha256}`, auth);
    await sdkExportValue({ kind: "sdk_report", id: report.id, result_ref: report.binding.result_ref, artifact_id: report.binding.artifact_id, hash: report.content_sha256 }, [report], bytes, sessionId);
    return bytes;
  }
  return <article aria-label="Imported report detail" className="space-y-4">
    <h3>{report.external_report_id}</h3>
    <p>Imported source · {report.source_kind === "agent_result" ? "No grade supplied" : "Original SDK judgments"} · {report.size_bytes} bytes</p>
    <p>Importing does not verify execution, change expectations, or replace a hosted result. Capture and application settings are uploader claims.</p>
    {report.diagnostics?.map(text => <p key={text}>{text}</p>)}
    {report.summary !== undefined && <details open><summary>Original summary</summary><pre className="whitespace-pre-wrap break-words">{JSON.stringify(report.summary, null, 2)}</pre></details>}
    <details><summary>Identity, fingerprints and capture claims</summary><pre className="whitespace-pre-wrap break-words">{JSON.stringify({ binding: report.binding, content_sha256: report.content_sha256, evidence: report.evidence_metadata, cases: report.cases }, null, 2)}</pre></details>
    <div className="flex flex-wrap gap-2">
      <VibeButton disabled={busy} onClick={() => action(async (auth, current) => { const bytes = await original(auth); if (current()) downloadBytes(`sdk-report-${report.id}.json`, bytes, "application/json"); })}>Download exact original</VibeButton>
      <VibeButton disabled={busy} onClick={() => action(async (auth, current) => { const bytes = await original(auth); if (current()) setRawSource(JSON.parse(new TextDecoder("utf-8", { fatal: true, ignoreBOM: true }).decode(bytes))); })}>Read original metrics and evidence</VibeButton>
      {onExport && <VibeButton onClick={() => onExport(report)}>Export report with tests</VibeButton>}
    </div>
    {rawSource !== undefined && <details open><summary>Original report contents</summary><pre className="whitespace-pre-wrap break-words">{JSON.stringify(rawSource, null, 2)}</pre></details>}
    <label>Source case<select aria-label="SDK detail source case" disabled={busy || !!reviewRequest || !!promotion} value={sourceID} onChange={e => { setSourceID(e.target.value); setReason(""); setEvidence(undefined); setReviewed(false); }}><option value="">Choose a source case</option>{report.cases.map(c => <option key={c.source_case_id} value={c.source_case_id}>{c.source_case_id} → {[c.input_set_key, c.challenge_key, c.case_key].filter(Boolean).join(" / ")}</option>)}</select></label>
    {evidence && <section className="space-y-3" aria-label="Imported case evidence">
      <p>Original verdict: {report.source_kind === "agent_result" ? "Not supplied" : evidence.verdict}</p>
      <h4>Retained final reply</h4><pre className="whitespace-pre-wrap break-words">{evidence.output || "No final reply recorded."}</pre>
      {evidence.messages?.map(message => <p key={message.id}><strong>{message.role}</strong>: {message.content}</p>)}
      <details><summary>Original checks</summary><pre className="whitespace-pre-wrap break-words">{JSON.stringify(evidence.checks, null, 2)}</pre></details>
      {access.canWrite && <form className="space-y-2" onSubmit={e => { e.preventDefault(); void action(async (auth, current) => {
        const request = reviewRequest || { client_id: crypto.randomUUID(), result_ref: report.binding.result_ref, artifact_id: report.binding.artifact_id, source_case_id: sourceID, case_key: evidence.case_key, source_evidence_hash: evidence.evidence_hash || "", reason };
        setReviewRequest(request); setReviewRejected(false);
        await vibeFetch(`/sessions/${sessionId}/grade-reviews`, auth, { method: "POST", body: JSON.stringify(request) });
        if (current()) { setReviewRequest(undefined); setReason(""); setNotice("Review recorded. The imported grade is unchanged."); setAttempt(n => n + 1); }
      }, e => setReviewRejected(rejectedSDKWrite(e, "review"))); }}><label>Why do you disagree?<textarea className="vibe-textarea" aria-label="SDK grade review reason" disabled={busy || !!reviewRequest} value={reason} onChange={e => setReason(e.target.value)} /></label>
        <VibeButton type="submit" disabled={busy || reviewRejected || !reason.trim() || new TextEncoder().encode(reason).length > 4096 || !evidence.evidence_hash}>{reviewRequest ? "Retry this grade review" : "Record grade review"}</VibeButton>
        {reviewRequest && !reviewRejected && <p>Retry recovers this same review receipt.</p>}
        {reviewRequest && reviewRejected && <div><p>The review was rejected. Your text is retained; reload the evidence and edit before submitting again.</p><VibeButton disabled={busy} onClick={() => { setReviewRequest(undefined); setReviewRejected(false); setEvidence(undefined); setError(""); setAttempt(n => n + 1); }}>Edit rejected grade review</VibeButton></div>}
      </form>}
      {access.canWrite && evidence.expected && <section aria-label="Review imported expectation" className="space-y-2">
        <h4>Review imported expectation</h4><pre className="whitespace-pre-wrap break-words">{evidence.expected}</pre>
        <p>Adopt this exact expectation into a new test version. The existing version, source report and grades remain unchanged.</p>
        <label><input type="checkbox" disabled={busy || !!promotion} checked={reviewed} onChange={e => setReviewed(e.target.checked)} />I reviewed this expectation for the selected test case.</label>
        <VibeButton disabled={busy || promotionRejected || !reviewed || revision === undefined} onClick={() => action(async (auth, current) => {
          const request = promotion || { client_id: crypto.randomUUID(), revision: revision!, source_case_id: sourceID, source_evidence_hash: report.content_sha256, expected: evidence.expected!, reviewed: true as const }; setPromotion(request); setPromotionRejected(false);
          const artifact = await vibeFetch<Artifact>(`${base}/promote`, auth, { method: "POST", body: JSON.stringify(request) });
          if (current()) { setPromotion(undefined); setReviewed(false); setNotice("Created a separate test version with the reviewed expectation."); onPromoted?.(artifact); }
        }, e => setPromotionRejected(rejectedSDKWrite(e, "promotion")))}>{promotion ? "Retry expectation promotion" : "Create version with this expectation"}</VibeButton>
        {promotion && promotionRejected && <div><p>The promotion was rejected. Review the source again before confirming a new request with the latest project revision.</p><VibeButton disabled={busy} onClick={() => { setPromotion(undefined); setPromotionRejected(false); setReviewed(false); setEvidence(undefined); setError(""); setAttempt(n => n + 1); }}>Review expectation again</VibeButton></div>}
      </section>}
    </section>}
    <section aria-label="Hosted SDK regrade" className="space-y-2"><h4>Regrade retained replies</h4>
      <p>Run the pinned tests against retained final replies using hosted grading. This creates a separate result and uses inference credits. It does not rerun the application or verify its execution. Message, tool and retrieval traces are not replayed.</p>
      {!inferenceAvailable && <p>Hosted grading is currently unavailable. Importing and reading reports remain available.</p>}
      <VibeButton disabled={busy || !access.canWrite || !inferenceAvailable || !models || revision === undefined || report.cases.length === 0 || !!regrade} onClick={() => action(async (auth, current) => {
        const request: RegradeRequest = { client_id: crypto.randomUUID(), revision: revision!, kind: "retest", purpose: "regrade", artifact_id: report.binding.artifact_id, models: models!, regrade_source_result_ref: report.binding.result_ref, source_evidence_hash: report.content_sha256 };
        const quote = await vibeFetch<RunQuote>(`/sessions/${sessionId}/run-quote`, auth, { method: "POST", body: JSON.stringify(request) });
        if (current()) setRegrade({ request: { ...request, run_quote_id: quote.id }, quote });
      })}>Estimate hosted regrade</VibeButton>
      {regrade && <div><p>{regrade.quote.cases} cases · up to {regrade.quote.calls} grading calls · maximum ${(regrade.quote.max_cost_nano_usd / 1e9).toFixed(4)}. Quote expires {new Date(regrade.quote.expires_at).toLocaleTimeString()}.</p>
        <VibeButton disabled={busy || !access.canWrite || !inferenceAvailable} onClick={() => action(async (auth, current) => {
          const operation = await vibeFetch<Operation>(`/sessions/${sessionId}/messages`, auth, { method: "POST", body: JSON.stringify(regrade.request) });
          if (current()) { setRegrade(undefined); setNotice("Separate hosted regrade submitted. Original grades are unchanged."); onRegraded?.(operation); }
        })}>Confirm separate hosted regrade</VibeButton>
        <VibeButton disabled={busy} onClick={() => setRegrade(undefined)}>Dismiss estimate</VibeButton>
      </div>}
    </section>
    <section aria-label="SDK grade reviews"><h4>Grade reviews</h4><p>Latest 50 reviews for this report.</p>{reviews.map(r => <article key={r.id}><p>{r.source_case_id || r.case_key}: {r.reason}</p><p>Original reviewed verdict: {r.evidence.verdict}</p></article>)}</section>
    {error && <div role="alert"><p>{error}</p><VibeButton disabled={busy} onClick={() => setAttempt(n => n + 1)}>Retry reading report evidence</VibeButton></div>}
    {notice && <p role="status">{notice}</p>}
  </article>;
}
