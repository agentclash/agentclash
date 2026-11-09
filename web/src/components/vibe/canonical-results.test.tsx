import React,{act} from "react";
import {createRoot} from "react-dom/client";
import {afterEach,expect,it,vi} from "vitest";
import {CanonicalResults} from "./canonical-results";
import {canonicalExportValue,type CanonicalResult} from "@/lib/vibe-canonical";
import {VibeError} from "@/lib/vibe";
const mocks=vi.hoisted(()=>({fetch:vi.fn()}));
vi.mock("@/lib/vibe",async importOriginal=>({...await importOriginal<typeof import("@/lib/vibe")>(),vibeFetch:mocks.fetch}));
const binding={session_id:"project",artifact_id:"artifact",result_ref:{kind:"run" as const,id:"run"}};
const result={binding,evidence_metadata:{result_ref:binding.result_ref,origin:"live",fingerprints:{model:null,build:null,test:"tests",judge:null,config:"config"},capture_scope:{messages:{scope:"unknown",instrumentation:"unknown"},tools:{scope:"unknown",instrumentation:"unknown"},retrieval:{scope:"unknown",instrumentation:"unknown"}},application_revision:null},name:"Application fixture",status:"completed",created_at:"2026-10-26",participants:[],evidence_hash:"frozen",omissions:["Raw canonical event bodies omitted"]} satisfies CanonicalResult;
const props=()=>({sessionId:"project",workspaceId:"workspace",actorScope:"actor",token:async()=>"token",access:{status:"ready" as const,canRead:true,canWrite:true,canAdmin:true},selected:result,onSelectResult:vi.fn(),onExport:vi.fn()});
afterEach(()=>{vi.clearAllMocks();vi.unstubAllGlobals()});
async function render(){vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT",true);const node=document.createElement("div"),root=createRoot(node),p=props();document.body.append(node);await act(async()=>root.render(<CanonicalResults {...p}/>));return {node,root,p,close:async()=>{await act(async()=>root.unmount());node.remove()}}}
it("reads an exact selected result even when bounded history cannot load",async()=>{
 mocks.fetch.mockRejectedValue(new Error("Choose an exact result"));const view=await render();try{
  expect(view.node.textContent).toContain("Status: completed");expect(mocks.fetch).toHaveBeenCalledOnce();expect(mocks.fetch).toHaveBeenCalledWith("/sessions/project/canonical-results","token");
  const detail=view.node.querySelector('article[aria-label="Application result detail"]')!,history=view.node.querySelector<HTMLDetailsElement>('details[aria-label="Application run history"]');
  expect(history).not.toBeNull();expect(history!.open).toBe(false);expect(detail.closest("details")).toBeNull();expect(detail.compareDocumentPosition(history!) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  expect(view.node.querySelector('[role="alert"]')!.closest("details")).toBeNull();
 }finally{await view.close()}
});
it("retains same-owner metadata on transient refresh but clears definitive revocation",async()=>{
 mocks.fetch.mockResolvedValue([result]);const view=await render();
 try{const refresh=Array.from(view.node.querySelectorAll("button")).find(b=>b.textContent==="Refresh application runs")!;mocks.fetch.mockRejectedValue(new Error("temporarily unavailable"));await act(async()=>refresh.click());expect(view.node.textContent).toContain("Status: completed");mocks.fetch.mockRejectedValue(new VibeError("forbidden","Revoked"));await act(async()=>refresh.click());expect(view.node.textContent).not.toContain("Status: completed");expect(view.node.textContent).not.toContain("Application fixture")}finally{await view.close()}
});
it("exports the frozen full source identity and rejects conflicting descriptors",()=>{
 const item={kind:"canonical_result" as const,id:"run:run:artifact",result_ref:binding.result_ref,artifact_id:"artifact",hash:"frozen"};
 const value=canonicalExportValue(item,[result],"project");expect(value).toEqual(result);expect(value).not.toBe(result);
 for(const invalid of [{...item,hash:"changed"},{...item,artifact_id:"foreign"},{...item,result_ref:{kind:"eval_session" as const,id:"run"}},{...item,id:"run"}])expect(()=>canonicalExportValue(invalid,[result],"project")).toThrow("frozen preparation");
});
it("omits empty project application history and keeps existing results selectable",async()=>{
 mocks.fetch.mockResolvedValue([]);const view=await render();try{
  await act(async()=>view.root.render(<CanonicalResults {...view.p} selected={undefined} hideEmptyHistory/>));
  expect(view.node.querySelector('[aria-label="Application run history"]')).toBeNull();
  mocks.fetch.mockResolvedValue([result]);
  await act(async()=>view.root.render(<CanonicalResults {...view.p} actorScope="next-actor" selected={undefined} hideEmptyHistory/>));
  const history=view.node.querySelector<HTMLDetailsElement>('[aria-label="Application run history"]')!;expect(history).not.toBeNull();expect(history.open).toBe(false);
  await act(async()=>history.querySelector<HTMLElement>("summary")!.click());
  await act(async()=>[...history.querySelectorAll("button")].find(b=>b.textContent?.startsWith("Run Application fixture"))!.click());expect(view.p.onSelectResult).toHaveBeenCalledWith(result.binding);
 }finally{await view.close()}
});

it("retries an unavailable project history without showing an empty Application run history",async()=>{
 vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT",true);mocks.fetch.mockRejectedValue(new Error("History unavailable"));const node=document.createElement("div");document.body.append(node);const root=createRoot(node);
 try{
  await act(async()=>root.render(<CanonicalResults {...props()} selected={undefined} hideEmptyHistory/>));
  expect(node.querySelector('[aria-label="Application run history"]')).toBeNull();expect(node.querySelector('[role="alert"]')!.textContent).toBe("History unavailable");
  mocks.fetch.mockResolvedValue([]);await act(async()=>[...node.querySelectorAll("button")].find(b=>b.textContent==="Refresh application runs")!.click());
  expect(node.textContent).toBe("");
 }finally{await act(async()=>root.unmount());node.remove()}
});
