"use client";
import { useEffect, useState } from "react";
import type { Artifact, EvidenceSet } from "@/lib/vibe";
import { useVibeEditorDraft } from "@/lib/use-vibe-editor-draft";
import { VibeButton } from "./vibe-button";

export function RecordedTests({project,artifact,evidence,busy,blocked,onDirty,onEdit,onRun}:{project:string;artifact:Artifact;evidence?:EvidenceSet;busy:boolean;blocked?:boolean;onDirty:(dirty:boolean)=>void;onEdit:(fields:Record<string,unknown>)=>Promise<boolean>;onRun:()=>void}){
 const rules=artifact.conversation_evaluation!.expectations;
 const initial=rules.map(r=>r.statement).join("\n");
 const draft=useVibeEditorDraft(project,artifact.id,"recorded-expectations",JSON.stringify(rules),initial);
 const [error,setError]=useState("");
 useEffect(()=>{onDirty(draft.dirty)},[draft.dirty]);
 return <section aria-label="Recorded reply tests" className="space-y-4"><h2>Recorded replies and expectations</h2><p>The evaluator grades these exact saved replies. No target model or live app is called.</p>
  {evidence?.conversations.map(chat=><article className="vibe-panel p-4" key={chat.key}><h3>{chat.title||chat.key}</h3>{chat.messages.map(message=><div key={message.id}><strong>{message.role}</strong><pre className="whitespace-pre-wrap break-words">{message.content}</pre></div>)}</article>)}
  <label>Expected behavior<textarea aria-label="Recorded expectations" className="vibe-textarea" value={draft.value} onChange={e=>draft.setValue(e.target.value)} disabled={busy}/></label>
  {draft.conflict&&<p role="alert">The saved expectations changed. Keep this draft for its source or discard it.</p>}
  {draft.dirty&&<><VibeButton disabled={busy||draft.conflict||!draft.value.trim()} onClick={async()=>{const submitted=draft.value;const expectations=submitted.split("\n").map(s=>s.trim()).filter(Boolean).map((statement,i)=>({id:rules[i]?.id||`expectation-${i+1}`,statement}));if(await onEdit({artifact_id:artifact.id,expectations})){draft.discard(submitted);onDirty(false)}else setError("Expectations were not saved. Your previous version remains available.");}}>Save expectations</VibeButton><VibeButton onClick={()=>{draft.discard();onDirty(false)}}>Discard expectation changes</VibeButton></>}
  {error&&<p role="alert">{error}</p>}
  <VibeButton disabled={busy||blocked||draft.dirty||!evidence} onClick={onRun}>Run recorded tests</VibeButton>
 </section>;
}
