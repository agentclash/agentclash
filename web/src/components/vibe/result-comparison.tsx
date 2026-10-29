"use client";
import { useEffect, useState } from "react";
import type { CaseResult, Operation } from "@/lib/vibe";
import { compareResultEvidence } from "@/lib/vibe-result-evidence";
import { VibeButton } from "./vibe-button";

function SavedReply({label,operation,caseKey,version,load}:{label:string;operation:string;caseKey:string;version?:string;load:(operation:string,key:string)=>Promise<CaseResult>}){
 const [attempt,setAttempt]=useState(0);
 const identity=JSON.stringify([operation,caseKey,version,attempt]);
 const [state,setState]=useState<{identity:string;result?:CaseResult;error?:string}>();
 const result=state?.identity===identity?state.result:undefined,error=state?.identity===identity?state.error:undefined;
 useEffect(()=>{let live=true;void load(operation,caseKey).then(c=>{if(live){if(c.case_key!==caseKey||(version&&c.version!==version))throw Error("The saved test identity changed.");setState({identity,result:c})}}).catch(e=>{if(live)setState({identity,error:e.message})});return()=>{live=false}},[operation,caseKey,version,attempt,identity]);
 return <section aria-label={label}><h4>{label}</h4>{result?<><p>{result.verdict}</p><pre className="whitespace-pre-wrap break-words">{result.output||result.messages?.filter(m=>m.role==="assistant").map(m=>m.content).join("\n\n")||"No completed reply is saved."}</pre>{result.checks.map(c=><p key={c.key}>{c.key}: {c.evidence||c.error?.message||"Unassessed"}</p>)}</>:error?<><p role="alert">{error}</p><VibeButton onClick={()=>setAttempt(a=>a+1)}>Reload {label.toLowerCase()}</VibeButton></>:<p>Loading saved reply…</p>}</section>;
}
export function ResultComparison({current,baseline,load}:{current:Operation;baseline:Operation;load:(operation:string,key:string)=>Promise<CaseResult>}){
 const comparison=compareResultEvidence(current,baseline);
 const keys=current.results.filter(c=>baseline.results.some(b=>b.case_key===c.case_key)).map(c=>c.case_key);
 const [open,setOpen]=useState(false);const [key,setKey]=useState(keys[0]||"");
 return <section aria-label="Run comparison" className="space-y-3"><h3>{comparison.controlled?"Controlled comparison":"Comparison with changed or unverified settings"}</h3>
  <p>Changed: {comparison.changed.join(", ")||"no known changes"}. Unverified: {comparison.unknown.join(", ")||"none"}.</p>
  {(current.evidence_metadata?.origin==="regraded"||current.source?.comparison==="regraded")&&<p>These are grades on saved outputs, not evidence of improved agent behavior.</p>}
  {!!keys.length&&<VibeButton onClick={()=>setOpen(!open)}>{open?"Hide before and after":"Compare saved replies"}</VibeButton>}
  {open&&<><label>Test<select aria-label="Comparison test" value={key} onChange={e=>setKey(e.target.value)}>{keys.map(k=><option key={k}>{k}</option>)}</select></label><SavedReply label="Before" operation={baseline.id} caseKey={key} version={baseline.result_scope?.artifact_id||baseline.source?.artifact_id||baseline.results.find(c=>c.case_key===key)?.version} load={load}/><SavedReply label="After" operation={current.id} caseKey={key} version={current.result_scope?.artifact_id||current.source?.artifact_id||current.results.find(c=>c.case_key===key)?.version} load={load}/></>}
 </section>;
}
