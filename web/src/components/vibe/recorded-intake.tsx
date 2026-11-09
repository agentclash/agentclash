"use client";
import { useId, useState } from "react";
import { useVibeEditorDraft } from "@/lib/use-vibe-editor-draft";
import type { RecordedIntakeRequest, RecordedIntakeOutcome } from "@/lib/vibe-result-evidence";
import { VibeDataUseNotice } from "./vibe-data-use";
import { ReplyCount, repliesPlaceholder } from "./improve-inputs";
import "./improve.css";
import { VibeButton } from "./vibe-button";

export function RecordedIntake({project,revision,busy,onSave,onClose,reviewedExpectations,saveError}:{saveError?:string;project:string;revision:number;busy:boolean;reviewedExpectations?:{id:string;statement:string}[];onSave:(request:RecordedIntakeRequest)=>Promise<RecordedIntakeOutcome>;onClose:()=>void}){
 const content=useVibeEditorDraft(project,"intake","recorded-replies","recorded-v1","");
 const expectations=useVibeEditorDraft(project,"intake","recorded-expectations","recorded-v1",reviewedExpectations?.map(rule=>rule.statement).join("\n") || "");
 const pending=useVibeEditorDraft(project,"intake","recorded-request","recorded-v1","");
 const [error,setError]=useState("");
 const id=useId();
 return <form className="space-y-4" aria-label="Recorded reply intake" onSubmit={async event=>{
  event.preventDefault();if(busy)return;
  const wasPending=!!pending.value;
  let request:RecordedIntakeRequest;
  try{request=pending.value?JSON.parse(pending.value):{client_id:crypto.randomUUID(),revision,label:"App replies",content:content.value,expectations:expectations.value.split("\n").map(s=>s.trim()).filter(Boolean).map((statement,i)=>({id:reviewedExpectations?.[i]?.id || `expectation-${i+1}`,statement}))};}catch{setError("The saved request could not be read. Your draft is preserved.");return;}
  if(!request.content.trim()||!request.expectations.length){setError("Add actual replies and at least one expected behavior.");return;}
  pending.setValue(JSON.stringify(request));setError("");
  const outcome=await onSave(request);
  if(outcome==="saved"){content.discard(request.content);expectations.discard();pending.discard();onClose();}
  else if(outcome==="rejected"&&!wasPending){pending.discard();setError("The replies were not saved. Review the reported issue and try again.");}
  else setError("Saving is still unconfirmed. Retry the same saved request to recover its receipt; your source will not be duplicated.");
 }}>
  <h2>Test replies from my app</h2><p>Paste 1–3 complete conversations, including customer messages and actual agent replies. Enter your expectations separately. This grades saved replies; it does not call your app.</p>
  <VibeDataUseNotice/>
  <details><summary>Sample transcript format</summary><pre className="whitespace-pre-wrap">{"Customer: I opened an item bought 10 days ago.\nAgent: I can refund that."}</pre><p>Example expectation: Opened items are not eligible for a refund. Use your own rules.</p></details>
  <label>Recorded replies<textarea aria-describedby={saveError||error ? `${id}-error` : undefined} aria-label="Recorded replies" className="vibe-textarea" placeholder={repliesPlaceholder} disabled={busy||!!pending.value} value={content.value} onChange={e=>content.setValue(e.target.value)}/></label>
  <ReplyCount text={content.value} />
  <label>Expected behavior<textarea placeholder="One rule per line. For example: Opened items are not eligible for a refund." aria-describedby={saveError||error ? `${id}-error` : undefined} aria-label="Expected behavior" className="vibe-textarea" disabled={busy||!!pending.value} value={expectations.value} onChange={e=>expectations.setValue(e.target.value)}/></label><p className="text-sm vibe-muted">One expectation per line. Saving does not run a model. The server checks the upload and project storage limits.</p>
  {(saveError||error)&&<p id={`${id}-error`} role="alert">{saveError||error}</p>}
  <VibeButton variant="primary" type="submit" disabled={busy}>{pending.value?"Retry saving these replies":"Review recorded replies"}</VibeButton><VibeButton variant="quiet" className="underline" disabled={busy} onClick={onClose}>Back</VibeButton>
 </form>;
}

// A committed snapshot can arrive before the POST acknowledgment. Keep recovery
// reachable even after the initial intake has been replaced by the workspace.
export function RecordedIntakeRecovery({project,busy,onSave,saveError}:{saveError?:string;project:string;busy:boolean;onSave:(request:RecordedIntakeRequest)=>Promise<RecordedIntakeOutcome>}) {
 const pending=useVibeEditorDraft(project,"intake","recorded-request","recorded-v1","");
 const content=useVibeEditorDraft(project,"intake","recorded-replies","recorded-v1","");
 const expectations=useVibeEditorDraft(project,"intake","recorded-expectations","recorded-v1","");
 const [error,setError]=useState("");
 if(!pending.value)return null;
 return <section aria-label="Recorded intake recovery" className="vibe-panel p-4 space-y-3">
  <p>Saving these replies is still unconfirmed. A newer project snapshot does not identify this request. Recover its exact receipt before running.</p>
  <VibeButton disabled={busy} onClick={async()=>{
   const submitted=pending.value;const expected=expectations.value;
   let request:RecordedIntakeRequest;try{request=JSON.parse(submitted)}catch{setError("The saved request cannot be read. Your source draft remains saved.");return}
   const outcome=await onSave(request);
   if(outcome==="saved"){content.discard(request.content);expectations.discard(expected);pending.discard(submitted);setError("")}
   else if(outcome==="rejected"){setError("This retry was rejected, but the original save is still unconfirmed. Your exact request and source remain saved.")}
   else setError("The receipt is still unavailable. Retry this same request when the connection returns.");
  }}>Retry saving these replies</VibeButton>{(saveError||error)&&<p role="alert">{saveError||error}</p>}
 </section>;
}
