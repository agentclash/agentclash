"use client";
import {useEffect,useRef,useState} from "react";
import {connectorClient,type ConnectorBinding,type ConnectorConfig,type ConnectorPublication,type ConnectorSelection,type DeploymentAdapter} from "@/lib/connector-client";
import type {ProjectResultBinding} from "@/lib/vibe-result-evidence";
import {VibeButton} from "./vibe-button";

export type ConnectorSettingsProps={
 sessionId:string;workspaceId:string;artifactId:string;actorScope:string;
 // Structural until the reviewed SDK owner is integrated; no second access hook.
 access:{status:"loading"|"ready"|"signed_out"|"unavailable";canRead:boolean;canWrite:boolean;canAdmin:boolean};
 token:string|null;
 publication?:ConnectorPublication;
 onConfigured:(selection:ConnectorSelection)=>void;
 onRunCreated:(binding:ProjectResultBinding)=>void;
};
export function ConnectorSettings(p:ConnectorSettingsProps){
 const [source,setSource]=useState<DeploymentAdapter>("native");
 const [items,setItems]=useState<ConnectorBinding[]>([]),[selected,setSelected]=useState<ConnectorBinding|null>(null);
 const [name,setName]=useState("Application"),[endpoint,setEndpoint]=useState(""),[secret,setSecret]=useState("");
 const [auth,setAuth]=useState<"bearer"|"api_key">("bearer"),[input,setInput]=useState('{"question":"/prompt"}'),[pointer,setPointer]=useState("/answer");
 const [busy,setBusy]=useState(false),[message,setMessage]=useState(""),[reviewed,setReviewed]=useState(false),[uncertain,setUncertain]=useState(false);
 const generation=`${p.actorScope}:${p.workspaceId}:${p.sessionId}:${p.artifactId}:${p.token??""}:${p.access.status}:${p.access.canRead}:${p.access.canWrite}:${p.access.canAdmin}`;
 const current=useRef({key:generation,epoch:0});
 if(current.current.key!==generation)current.current={key:generation,epoch:current.current.epoch+1};
 const epoch=current.current.epoch;const mounted=useRef(true),locked=useRef(false);
 useEffect(()=>{mounted.current=true;return()=>{mounted.current=false}},[]);
 useEffect(()=>{
  setItems([]);setSelected(null);setReviewed(false);setMessage("");setUncertain(false);setBusy(false);locked.current=false;
  if(p.access.status!=="ready"||!p.access.canRead||!p.token||!p.workspaceId)return;
  const controller=new AbortController();
  connectorClient(p.token,p.workspaceId).list(controller.signal).then(result=>{if(current.current.epoch===epoch&&!controller.signal.aborted)setItems(result.connectors)}).catch(()=>{if(!controller.signal.aborted&&current.current.epoch===epoch)setMessage("Connectors could not be loaded. Reopen settings to try again.")});
  return()=>controller.abort();
 },[epoch,generation,p.access.status,p.access.canRead,p.token,p.workspaceId]);
 const active=()=>mounted.current&&current.current.epoch===epoch;
 const configured=(b:ConnectorBinding)=>p.onConfigured({session_id:p.sessionId,workspace_id:p.workspaceId,artifact_id:p.artifactId,deployment_id:b.deployment_id,snapshot_id:b.snapshot_id});
 async function perform(action:"save"|"approve"|"revoke"|"run"){
  if(locked.current||!p.token||p.access.status!=="ready")return;
  if(action==="run"?!p.access.canWrite:!p.access.canAdmin)return;
  locked.current=true;setBusy(true);setMessage("");
  try{
   const client=connectorClient(p.token,p.workspaceId);
   if(action==="save"){
    if(source==="native")return;
    let fields:Record<string,string>|undefined;
    if(source==="http_json"){const parsed:unknown=JSON.parse(input);if(!parsed||Array.isArray(parsed)||typeof parsed!=="object"||Object.values(parsed).some(v=>typeof v!=="string"))throw new Error("Map JSON field names to JSON pointer strings.");fields=parsed as Record<string,string>}
    const config:ConnectorConfig={adapter:source,endpoint_url:endpoint,secret_key:secret,auth,input_fields:fields,response_pointer:source==="http_json"?pointer:"",timeout_seconds:60};
    const binding=await client.save(name,config);if(!active())return;setSelected(binding);setItems(old=>[...old,binding]);setReviewed(false);setMessage("Saved. Review the target and credential binding before approval.");
   }else if(selected){
    if(action==="approve"){if(!reviewed)return;const binding=await client.approve(selected);if(!active())return;setSelected(binding);setItems(old=>old.map(item=>item.snapshot_id===binding.snapshot_id?binding:item));configured(binding);setMessage("Connector approved. Running tests is a separate action.")}
    if(action==="revoke"){await client.revoke(selected);if(!active())return;setSelected({...selected,approved:false});setItems(old=>old.map(item=>item.snapshot_id===selected.snapshot_id?{...item,approved:false}:item));setReviewed(false);setMessage("Approval revoked. Future dispatches are blocked.")}
    if(action==="run"&&selected.approved&&p.publication?.artifact_id===p.artifactId&&!uncertain){const result=await client.run(selected,p.publication);if(!active())return;p.onRunCreated({session_id:p.sessionId,artifact_id:p.artifactId,result_ref:{kind:"run",id:result.id},challenge_pack_version_id:p.publication.challenge_pack_version_id});setMessage("Canonical run created.")}
   }
  }catch(error){if(active()){setMessage(error instanceof Error?error.message:"Connector action failed.");if(action==="run"){setUncertain(true);setMessage("Run creation was not confirmed. Check run history before starting another run.")}}}
  finally{if(active()){locked.current=false;setBusy(false)}}
 }
 if(!p.token||p.access.status==="signed_out")return <p className="text-sm">Sign in to a workspace to connect an application. Guest projects support text only.</p>;
 if(p.access.status!=="ready")return <p role="status">{p.access.status==="loading"?"Checking workspace access…":"Workspace access is unavailable. Reopen settings to try again."}</p>;
 if(!p.access.canRead)return <p>Workspace access is required to view connectors.</p>;
 const inputClass="w-full min-w-0 rounded-md border border-builder-border bg-builder-bg px-3 py-2 text-sm";
 return <section aria-label="Application source" className="min-w-0 space-y-4">
  <label className="block text-sm">Application source<select className={inputClass} value={source} onChange={e=>{setSource(e.target.value as DeploymentAdapter);setSelected(null);setReviewed(false)}} disabled={busy}><option value="native">Native AgentClash agent</option><option value="http_json">HTTP JSON application</option><option value="hosted">Hosted callback application</option></select></label>
  {source==="native"?<p className="text-sm">Use the native agent and model settings for your AgentClash deployment.</p>:<>
   <label className="block text-sm">Saved connector<select className={inputClass} value={selected?.snapshot_id??""} disabled={busy} onChange={e=>{const b=items.find(item=>item.snapshot_id===e.target.value)??null;setSelected(b);setReviewed(false);if(b?.approved)configured(b)}}><option value="">Choose a connector</option>{items.filter(item=>item.config.adapter===source).map(item=><option key={item.snapshot_id} value={item.snapshot_id}>{item.target_origin} ({item.approved?"approved":"needs approval"})</option>)}</select></label>
   {p.access.canAdmin?<details><summary className="cursor-pointer text-sm">Configure a new connector</summary><form className="mt-3 space-y-3" onSubmit={e=>{e.preventDefault();void perform("save")}}>
    <label className="block text-sm">Name<input required className={inputClass} maxLength={200} value={name} onChange={e=>setName(e.target.value)} disabled={busy}/></label>
    <label className="block text-sm">HTTPS endpoint<input required type="url" className={inputClass} placeholder="https://app.example.com/invoke" value={endpoint} onChange={e=>setEndpoint(e.target.value)} disabled={busy}/></label>
    <label className="block text-sm">Workspace secret key<input required className={inputClass} placeholder="APP_TOKEN" autoComplete="off" value={secret} onChange={e=>setSecret(e.target.value)} disabled={busy}/></label><p className="text-xs">Add the credential in workspace Secrets first. Enter its key here, never the secret value.</p>
    <label className="block text-sm">Authentication<select className={inputClass} value={auth} onChange={e=>setAuth(e.target.value as typeof auth)} disabled={busy}><option value="bearer">Bearer token</option><option value="api_key">X-API-Key header</option></select></label>
    {source==="http_json"?<><label className="block text-sm">JSON input field mapping<textarea required className={inputClass} rows={3} value={input} onChange={e=>setInput(e.target.value)} disabled={busy}/></label><p className="text-xs">Map each request field to a JSON pointer in the published case payload, such as /prompt.</p><label className="block text-sm">Response JSON pointer<input className={inputClass} value={pointer} onChange={e=>setPointer(e.target.value)} disabled={busy}/></label></>:<p className="text-sm">The application receives /agentclash/runs requests and sends signed callbacks. Its original deadline applies to the whole run.</p>}
    <p className="text-xs">Requests and responses are limited to 1 MiB. This connector uses a 60-second deadline and never follows redirects.</p>
    <VibeButton type="submit" disabled={busy}>Save connector</VibeButton>
   </form></details>:<p className="text-sm">A workspace admin must configure and approve connectors.</p>}
   {selected&&<div className="space-y-3 text-sm"><p className="break-all">Target: {selected.target_origin}</p><p>Credential: {selected.config.secret_key}, revision {selected.secret_revision}. {selected.approved?"Approved":"Approval required"}.</p>
    {p.access.canAdmin&&!selected.approved&&<><label className="flex items-start gap-2"><input type="checkbox" checked={reviewed} onChange={e=>setReviewed(e.target.checked)} disabled={busy}/><span>I approve sending this workspace credential and mapped test inputs to this target.</span></label><VibeButton disabled={busy||!reviewed} onClick={()=>void perform("approve")}>Approve connector</VibeButton></>}
    {p.access.canAdmin&&selected.approved&&<VibeButton disabled={busy} onClick={()=>void perform("revoke")}>Revoke approval</VibeButton>}
    {selected.approved&&p.access.canWrite&&<><p>Canonical run charges and workspace spend limits apply. External application or provider fees are separate and excluded from the platform maximum. Cancellation stops local waiting; remote cancellation is not confirmed.</p>{p.publication?.artifact_id===p.artifactId?<><p>{p.publication.case_count} published cases selected.</p><VibeButton disabled={busy||uncertain} onClick={()=>void perform("run")}>Run published tests</VibeButton></>:<p>Select an exact published test version before running this connector.</p>}</>}
   </div>}
  </>}
  {message&&<p role="status" className="break-words text-sm">{message}</p>}
 </section>;
}
