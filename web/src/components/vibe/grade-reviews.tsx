"use client";
import { useEffect, useState } from "react";
import { vibeFetch, VibeError, type CaseResult, type Operation } from "@/lib/vibe";
import type { GradeReview, GradeReviewInput } from "@/lib/vibe-result-evidence";
import { useVibeConnection } from "@/lib/vibe-connection";
import { useVibeEditorDraft } from "@/lib/use-vibe-editor-draft";
import { VibeButton } from "./vibe-button";

export function GradeReviews({sessionID,operation,eventCursor,load}:{sessionID:string;operation:Operation;eventCursor?:number;load:(key:string)=>Promise<CaseResult>}){
 const {token}=useVibeConnection();const [open,setOpen]=useState(false);const [pickedKey,setKey]=useState(operation.results[0]?.case_key||"");
 const [evidence,setEvidence]=useState<CaseResult>();const [reviews,setReviews]=useState<GradeReview[]>([]);const [error,setError]=useState("");const [busy,setBusy]=useState(false);const [attempt,setAttempt]=useState(0);
 const pending=useVibeEditorDraft(sessionID,operation.id,"review-request","review-v1","");
 let recovered:GradeReviewInput|undefined;let corrupt=false;
 try {if(pending.value)recovered=JSON.parse(pending.value) as GradeReviewInput;}catch{corrupt=true}
 const key=recovered?.case_key||pickedKey;
 const reason=useVibeEditorDraft(sessionID,operation.id,`review:${key}`,"review-v1","");
 const path=`/sessions/${encodeURIComponent(sessionID)}/grade-reviews`;
 useEffect(()=>{let live=true;setReviews([]);setError("");void token().then(auth=>vibeFetch<GradeReview[]>(`${path}?result_id=${encodeURIComponent(operation.id)}`,auth)).then(items=>{if(!Array.isArray(items))throw new Error("Grade reviews could not be read. Reload to try again.");if(live)setReviews(items)}).catch(e=>{if(live)setError(e.message)});return()=>{live=false}},[sessionID,operation.id,eventCursor,attempt]);
 useEffect(()=>{if(!open||!key)return;let live=true;setEvidence(undefined);void load(key).then(value=>{if(live)setEvidence(value)}).catch(e=>{if(live)setError(e.message)});return()=>{live=false}},[open,key,operation.id,attempt]);
 return <section aria-label="Grade reviews" className="space-y-3"><h3>Grade reviews</h3><p className="text-sm vibe-muted">Latest 50 reviews for this result.</p><p>A review records your disagreement. Changing expectations and regrading create separate versions or results.</p>
  <VibeButton onClick={()=>setOpen(!open)}>{open?"Close grade review":"Review a grade"}</VibeButton>
  {open&&<form className="space-y-3" onSubmit={async event=>{event.preventDefault();if(!evidence||busy||corrupt)return;setBusy(true);setError("");const captured=reason.value;
    const request:GradeReviewInput=recovered||{source_evidence_hash:evidence.evidence_hash||"",client_id:crypto.randomUUID(),result_ref:{kind:"vibe_operation",id:operation.id},artifact_id:evidence.version,case_key:evidence.case_key,reason:captured,quotes:evidence.checks.flatMap(c=>c.finding?.quotes||[]).filter((q,i,all)=>all.findIndex(x=>x.message_id===q.message_id&&x.text===q.text)===i).slice(0,20)};
    pending.setValue(JSON.stringify(request));
    try{await vibeFetch<GradeReview>(path,await token(),{method:"POST",body:JSON.stringify(request)});pending.discard();reason.discard(captured);setOpen(false);setAttempt(a=>a+1)}catch(e){setError((e as Error).message);if(!recovered&&e instanceof VibeError&&e.status&&e.status<500&&e.code!=="idempotency_conflict")pending.discard()}finally{setBusy(false)}
   }}><label>Saved case<select aria-label="Grade review case" value={key} disabled={busy||!!pending.value} onChange={e=>setKey(e.target.value)}>{operation.results.map(c=><option key={c.case_key} value={c.case_key}>{c.title||c.case_key}</option>)}</select></label>
    {evidence&&<><p>Original verdict: {evidence.verdict}</p><pre className="whitespace-pre-wrap break-words">{evidence.output||evidence.messages?.filter(m=>m.role==="assistant").map(m=>m.content).join("\n\n")}</pre></>}
    <label>Why do you disagree?<textarea aria-label="Grade review reason" className="vibe-textarea" disabled={busy||!!pending.value} value={reason.value} onChange={e=>reason.setValue(e.target.value)}/></label>
    <VibeButton type="submit" disabled={busy||corrupt||!evidence?.evidence_hash||!reason.value.trim()}>{pending.value?"Retry saving this review":"Save grade review"}</VibeButton>{pending.value&&<p>Saving is unconfirmed. Retrying recovers the same review receipt.</p>}
   </form>}
  {corrupt&&<p role="alert">The stored review receipt cannot be read. Your draft remains saved.</p>}
  {error&&<div role="alert"><p>{error}</p><VibeButton onClick={()=>setAttempt(a=>a+1)}>Reload grade reviews</VibeButton></div>}
  {reviews.map(review=><article key={review.id}><p>{review.reason}</p><p className="text-sm vibe-muted">Reviewed {review.case_key}: {review.evidence.verdict} · {new Date(review.created_at).toLocaleString()}</p>{review.evidence_changed&&<p>The saved case changed later. This review retains the original evidence.</p>}<details><summary>Evidence reviewed</summary><pre className="whitespace-pre-wrap break-words">{review.evidence.output||review.evidence.messages?.map(m=>`${m.role}: ${m.content}`).join("\n\n")}</pre></details></article>)}
 </section>;
}
