"use client";
import { useState } from "react";
import { terminal, type Operation } from "@/lib/vibe";
import { unresolvedCase } from "@/lib/vibe-result-evidence";
import { VibeButton } from "./vibe-button";

const stoppedStates = ["CANCELLED", "EXPIRED", "FAILED"];
// A complete run needs no recovery; its rerun actions are in the scorecard.
export function offersFullRun(operation: Operation) {
 return terminal(operation.state) && (stoppedStates.includes(operation.state) || operation.results.some(unresolvedCase));
}

export function ResultRecoveryControls({operation,busy,onReload,onRetry,onFullRun}:{operation:Operation;busy:boolean;onReload:()=>void;onRetry?:(keys:string[])=>void;onFullRun:()=>void}){
 const unresolved=operation.results.filter(unresolvedCase);
 const [excluded,setExcluded]=useState<string[]>([]);
 const selected=unresolved.map(c=>c.case_key).filter(key=>!excluded.includes(key));
 const reconciled=terminal(operation.state)&&["SETTLED","RELEASED"].includes(operation.billing)&&!operation.diagnostics?.unresolved_billing_since;
 const recovery=offersFullRun(operation);
 if(!recovery&&terminal(operation.state)&&!operation.result_scope?.retry_source_result_ref&&operation.actual_cost_nano_usd!==null&&!operation.progress?.last_confirmed_at)return null;
 return <section aria-label="Result recovery" className="space-y-3">
  {operation.result_scope?.retry_source_result_ref&&<p>This is a separate run of {operation.result_scope.case_keys.length} selected cases. Earlier evidence remains separate; this does not establish that the whole suite passes.</p>}
  {operation.actual_cost_nano_usd===null&&<p>Actual provider cost is unknown while accounting reconciles. Completed evidence stays available.</p>}
  {operation.progress?.last_confirmed_at&&<p>Last confirmed progress: {new Date(operation.progress.last_confirmed_at).toLocaleString()} · phase {operation.progress.phase.replaceAll("_"," ")} · {Math.floor((operation.progress.phase_elapsed_ms||0)/1000)} seconds confirmed phase time.</p>}
  {terminal(operation.state)&&!!unresolved.length&&<fieldset><legend>Unresolved cases to retry</legend>{unresolved.map(c=><label className="block" key={c.case_key}><input type="checkbox" checked={!excluded.includes(c.case_key)} onChange={e=>setExcluded(old=>e.target.checked?old.filter(key=>key!==c.case_key):[...old,c.case_key])}/>{c.title||c.case_key}</label>)}</fieldset>}
  {terminal(operation.state)&&!!unresolved.length&&onRetry&&!reconciled&&<p>Retry becomes available after the original execution and billing finish reconciliation.</p>}
  {(recovery||!terminal(operation.state)||operation.actual_cost_nano_usd===null)&&<div className="flex flex-wrap gap-2">
   <VibeButton onClick={onReload}>Reload saved evidence</VibeButton>
   {recovery&&<>
    {!!unresolved.length&&onRetry&&<VibeButton disabled={busy||!reconciled||!selected.length} onClick={()=>onRetry(selected)}>Retry unresolved</VibeButton>}
    <VibeButton disabled={busy} onClick={onFullRun}>Run all again</VibeButton>
   </>}
  </div>}
 </section>;
}
