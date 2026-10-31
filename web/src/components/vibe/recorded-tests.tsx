"use client";
import { useEffect, useState } from "react";
import type { Artifact, EvidenceSet } from "@/lib/vibe";
import { useVibeEditorDraft } from "@/lib/use-vibe-editor-draft";
import { useImprovePrice, priceLabel, ImproveRunNotice } from "./improve-run-quote";
import "./improve.css";
import { VibeButton } from "./vibe-button";

export function RecordedTests({project,artifact,evidence,busy,blocked,inferenceUnavailable,onDirty,onEdit,onRun,editError,runNotice}:{editError?:string;runNotice?:import("react").ReactNode;project:string;artifact:Artifact;evidence?:EvidenceSet;busy:boolean;blocked?:boolean;inferenceUnavailable?:boolean;onDirty:(dirty:boolean)=>void;onEdit:(fields:Record<string,unknown>)=>Promise<boolean>;onRun:()=>void}){
 const price=useImprovePrice();
 const rules=artifact.conversation_evaluation!.expectations;
 const initial=rules.map(r=>r.statement).join("\n");
 const draft=useVibeEditorDraft(project,artifact.id,"recorded-expectations",JSON.stringify(rules),initial);
 const [error,setError]=useState("");
 useEffect(()=>{onDirty(draft.dirty)},[draft.dirty]);
 return <section aria-label="Recorded reply tests" className="space-y-4"><h2>Recorded replies and expectations</h2><p>The Grading model checks these exact saved replies. Your live app is not called.</p>
  {evidence?.conversations.map(chat=><article className="vibe-panel vibe-recorded-chat p-4" key={chat.key}><h3>{chat.title||chat.key}</h3>{chat.messages.map(message=><div className="vibe-recorded-bubble" data-role={message.role} key={message.id}><strong>{message.role==="user"?"Customer":message.role==="assistant"?"Agent":message.role==="system"?"System":message.role==="tool"?"Tool":"Unknown speaker"}</strong><p>{message.content}</p></div>)}</article>)}
  <label>Expected behavior<textarea aria-label="Recorded expectations" aria-describedby={editError||error ? "vibe-recorded-edit-error" : undefined} className="vibe-textarea" value={draft.value} onChange={e=>draft.setValue(e.target.value)} disabled={busy}/></label>
  {draft.conflict&&<p role="alert">The saved expectations changed. Keep this draft for its source or discard it.</p>}
  {draft.dirty&&<><VibeButton disabled={busy||draft.conflict||!draft.value.trim()} onClick={async()=>{const submitted=draft.value;const expectations=submitted.split("\n").map(s=>s.trim()).filter(Boolean).map((statement,i)=>({id:rules[i]?.id||`expectation-${i+1}`,statement}));if(await onEdit({artifact_id:artifact.id,expectations})){draft.discard(submitted);onDirty(false)}else setError("Expectations were not saved. Your previous version remains available.");}}>Save expectations</VibeButton><VibeButton onClick={()=>{draft.discard();onDirty(false)}}>Discard expectation changes</VibeButton></>}
  {(editError||error)&&<p id="vibe-recorded-edit-error" role="alert">{editError||error}{price?.credits && <VibeButton variant="quiet" onClick={price.onCredits}>Credits</VibeButton>}</p>}
  <VibeButton variant="primary" disabled={busy||blocked||inferenceUnavailable||draft.dirty||!evidence||price?.credits} onClick={onRun}>{priceLabel("Run recorded tests",price)}</VibeButton>
  {runNotice}{!editError && <ImproveRunNotice reason={draft.dirty||blocked?"Save or discard your draft before running.":inferenceUnavailable?"Model inference is unavailable.":!evidence?"The saved conversations are unavailable.":undefined} />}
 </section>;
}
