"use client";
import {useEffect,useRef,useState} from "react";
import {VibeError,vibeFetch} from "@/lib/vibe";
import {canonicalBase,loadCanonicalResult,type CanonicalResult} from "@/lib/vibe-canonical";
import type {ProjectResultBinding} from "@/lib/vibe-result-evidence";
import type {WorkspaceEvidenceAccess} from "@/lib/use-vibe-workspace-access";
import {useVibeEvidenceOwner} from "@/lib/use-vibe-evidence-owner";
import {VibeButton} from "./vibe-button";
export function CanonicalResults(p:{sessionId:string;workspaceId?:string;actorScope:string;token:()=>Promise<string|null|undefined>;access:WorkspaceEvidenceAccess;selectedResult?:ProjectResultBinding;onSelectResult:(b:ProjectResultBinding)=>void;onExport:(r:CanonicalResult)=>void}){
 const selectedKind=p.selectedResult?.result_ref.kind,selectedID=p.selectedResult?.result_ref.id,selectedArtifact=p.selectedResult?.artifact_id;
 const revoked=p.access.status==="signed_out"||p.access.status==="ready"&&!p.access.canRead;
 const life=useVibeEvidenceOwner(p.actorScope,p.sessionId,p.workspaceId,revoked?"revoked":"active");
 const [state,setState]=useState<{owner:object;rows?:CanonicalResult[];detail?:CanonicalResult;error?:string}>();const [attempt,retry]=useState(0);
 const token=useRef(p.token);useEffect(()=>{token.current=p.token},[p.token]);
 if(state&&state.owner!==life.owner)setState(undefined);
 useEffect(()=>{if(!p.access.canRead||!p.workspaceId)return;let active=true;const current=life.capture();let timer:ReturnType<typeof setTimeout>;
 async function load(){
  try{
   const auth=await token.current();if(!active||!current())return;
   const [list,detail]=await Promise.allSettled([
    vibeFetch<CanonicalResult[]>(canonicalBase(p.sessionId),auth).then(rows=>{if(!Array.isArray(rows)||rows.some(r=>r.binding.session_id!==p.sessionId||!["run","eval_session"].includes(r.binding.result_ref.kind)))throw Error("Canonical results could not be read.");return rows}),
    selectedID&&(selectedKind==="run"||selectedKind==="eval_session")?loadCanonicalResult(p.sessionId,{kind:selectedKind,id:selectedID},auth,selectedArtifact):Promise.resolve(undefined),
   ]);
   if(!active||!current())return;
   const errors=[list,detail].flatMap(result=>result.status==="rejected"?[result.reason]:[]);
   const denied=errors.some(e=>e instanceof VibeError&&(e.code==="not_found"||e.code==="forbidden"));
   setState(old=>({owner:life.owner,
    rows:denied?undefined:list.status==="fulfilled"?list.value:old?.owner===life.owner?old.rows:undefined,
    detail:denied?undefined:detail.status==="fulfilled"?detail.value:old?.owner===life.owner?old.detail:undefined,
    error:errors.map(e=>(e as Error).message).join(" ")||undefined}));
  }catch(e){if(active&&current())setState(old=>({owner:life.owner,rows:old?.owner===life.owner?old.rows:undefined,detail:old?.owner===life.owner?old.detail:undefined,error:(e as Error).message}))}
  finally{if(active&&current())timer=setTimeout(load,3000)}
 }
 void load();return()=>{active=false;clearTimeout(timer)}},[life,p.sessionId,p.workspaceId,p.access.canRead,selectedKind,selectedID,selectedArtifact,attempt]);
 if(!p.workspaceId||!p.access.canRead)return null;
 const visible=state?.owner===life.owner?state:undefined;
 const selected=[visible?.detail,...(visible?.rows||[])].filter((r):r is CanonicalResult=>!!r).find(r=>r.binding.result_ref.kind===p.selectedResult?.result_ref.kind&&r.binding.result_ref.id===p.selectedResult?.result_ref.id&&r.binding.artifact_id===p.selectedResult?.artifact_id);
 return <section aria-label="Canonical results" className="min-w-0 space-y-3"><h3>Application runs</h3><p>Live canonical execution, with workspace billing and separate external application fees.</p>{visible?.error&&<p role="alert">{visible.error}</p>}{!visible?.rows&&!visible?.error&&<p role="status">Loading application runs…</p>}<VibeButton onClick={()=>retry(n=>n+1)}>Refresh application runs</VibeButton>{visible?.rows?.length===0&&<p>No application runs are bound to these published tests yet.</p>}<ul>{visible?.rows?.map(r=><li key={`${r.binding.result_ref.kind}:${r.binding.result_ref.id}:${r.binding.artifact_id}`}><VibeButton onClick={()=>{if(life.current())p.onSelectResult(r.binding)}}>{r.binding.result_ref.kind==="eval_session"?"Evaluation":"Run"} {r.name} · {r.status}</VibeButton></li>)}</ul>{selected&&<article aria-label="Canonical result detail" className="space-y-3"><h4>{selected.name}</h4><p role="status">Canonical status: {selected.status}</p><p className="break-all">{selected.binding.result_ref.id}</p>{selected.participants.map(participant=><div key={participant.run_agent_id} className="space-y-2"><p>{participant.label} · {participant.adapter} · {participant.status}</p><p>{participant.cases.length} selected cases · {participant.events.length} persisted events</p><details><summary>Admitted case and snapshot identities</summary><p className="break-all">Snapshot {participant.snapshot_id}</p><ul>{participant.cases.map(c=><li key={c.source_case_id} className="break-all">{c.input_set_key} / {c.challenge_key} / {c.case_key} · {c.source_case_id}</li>)}</ul></details></div>)}<p>Model and execution-config identities are unknown when their effective settings were not captured at admission. External application build identity is unknown. Black-box output does not establish complete tool or retrieval capture.</p>{selected.omissions.map(note=><p className="text-sm" key={note}>{note}</p>)}<VibeButton onClick={()=>p.onExport(selected)}>Export canonical result</VibeButton></article>}</section>
}
