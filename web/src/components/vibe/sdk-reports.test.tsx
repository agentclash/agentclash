import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, expect, it, vi } from "vitest";
import { SDKReportImportForm } from "./sdk-report-import";
import { SDKReports } from "./sdk-reports";
import { VibeError } from "@/lib/vibe";
import type { SDKReport } from "@/lib/vibe-reports";
const transport=vi.hoisted(()=>({fetch:vi.fn(),bytes:vi.fn()}));
vi.mock("@/lib/vibe",async()=>({...await vi.importActual("@/lib/vibe"),vibeFetch:transport.fetch,vibeBytes:transport.bytes}));
afterEach(()=>{vi.clearAllMocks()});
const access={status:"ready" as const,canRead:true,canWrite:true,canAdmin:false};
const props={sessionId:"s",workspaceId:"w",artifactId:"a",actorScope:"user-a",access,token:async()=>"token"};
const report={id:"r",binding:{session_id:"s",artifact_id:"a",result_ref:{kind:"sdk_report",id:"r"}},evidence_metadata:{origin:"imported",fingerprints:{model:null,build:null,test:null,judge:null,config:null},capture_scope:{}},source_kind:"agent_result",content_sha256:"hash",external_report_id:"local",cases:[],diagnostics:["This agent result has no imported grade."]} as unknown as SDKReport;
const button=(node:HTMLElement,label:string)=>Array.from(node.querySelectorAll("button")).find(b=>b.textContent===label)!;
async function change(el:HTMLInputElement|HTMLTextAreaElement|HTMLSelectElement,value:string){await act(async()=>{const proto=el instanceof HTMLSelectElement?HTMLSelectElement.prototype:el instanceof HTMLTextAreaElement?HTMLTextAreaElement.prototype:HTMLInputElement.prototype;Object.getOwnPropertyDescriptor(proto,"value")!.set!.call(el,value);el.dispatchEvent(new Event(el instanceof HTMLSelectElement?"change":"input",{bubbles:true}));});}
it("imports exact bytes with explicit mapping and recovers the same retry without executing",async()=>{
 const calls:unknown[]=[];let fail=true;
 transport.fetch.mockImplementation(async(path:string,_auth:unknown,init?:RequestInit)=>{if(path.endsWith("case-mapping"))return [{artifact_id:"a",cases:[{case_key:"real",input_set_key:"set",challenge_key:"challenge"}],published:[]}];if(path.endsWith("/reports")){calls.push(JSON.parse(String(init?.body)));if(fail)throw new VibeError("unavailable","Try again",503);return report;}throw Error("Unexpected execution request "+path)});
 const node=document.createElement("div");document.body.append(node);const root=createRoot(node);const done=vi.fn();
 try{
  await act(async()=>root.render(<SDKReportImportForm {...props} onImported={done}/>));
  await change(node.querySelector('select[aria-label="SDK source format"]')!,"agent_result");
  const raw=' {"input":"original","output":"\\u0041"}\n';await change(node.querySelector('textarea')!,raw);
  await act(async()=>button(node,"Read source cases").click());
  await change(node.querySelector('input[aria-label="Standalone source case ID"]')!,"arbitrary local id");
  const select=node.querySelector('select[aria-label="Test case for Agent result"]')! as HTMLSelectElement;
  expect(select.value).toBe("");await change(select,select.options[1].value);
  await act(async()=>button(node,"Import historical report").click());
  expect(done).not.toHaveBeenCalled();expect(node.textContent).toContain("Try again");fail=false;
  await act(async()=>button(node,"Retry this report import").click());
  expect(calls).toHaveLength(2);expect(calls[1]).toEqual(calls[0]);expect((calls[0] as {source_json:string}).source_json).toBe(raw);expect(done).toHaveBeenCalledWith(report.binding);
 }finally{await act(async()=>root.unmount());node.remove()}
});
it("shows imported results with zero operations and selects the server binding only on request",async()=>{
 transport.fetch.mockResolvedValue([report]);const node=document.createElement("div");document.body.append(node);const root=createRoot(node);const select=vi.fn();
 try{await act(async()=>root.render(<SDKReports {...props} onSelectResult={select}/>));expect(node.textContent).toContain("Imported, unverified");expect(node.textContent).toContain("local");expect(select).not.toHaveBeenCalled();await act(async()=>button(node,"Open report local").click());expect(select).toHaveBeenCalledWith(report.binding);}finally{await act(async()=>root.unmount());node.remove()}
});
it("drops an old actor's late report list after A to B to A",async()=>{
 let resolve!:(v:SDKReport[])=>void;const pending=new Promise<SDKReport[]>(r=>resolve=r);transport.fetch.mockReturnValueOnce(pending).mockResolvedValue([]);
 const node=document.createElement("div");document.body.append(node);const root=createRoot(node);
 try{await act(async()=>root.render(<SDKReports {...props}/>));await act(async()=>root.render(<SDKReports {...props} actorScope="user-b"/>));await act(async()=>root.render(<SDKReports {...props}/>));await act(async()=>resolve([report]));expect(node.textContent).not.toContain("Open report local");}finally{await act(async()=>root.unmount());node.remove()}
});
it("works after StrictMode replay and invalidates late import completion on artifact change",async()=>{
 let resolve!:(v:SDKReport)=>void;const pending=new Promise<SDKReport>(r=>resolve=r);
 transport.fetch.mockImplementation(async(path:string)=>path.endsWith("case-mapping")?[{artifact_id:path.includes("/artifacts/b/")?"b":"a",cases:[{case_key:"real",input_set_key:"set",challenge_key:"challenge"}],published:[]}]:pending);
 const node=document.createElement("div");document.body.append(node);const root=createRoot(node);const done=vi.fn();
 const render=(artifactId="a")=><React.StrictMode><SDKReportImportForm {...props} artifactId={artifactId} onImported={done}/></React.StrictMode>;
 try{
  await act(async()=>root.render(render()));await change(node.querySelector('select[aria-label="SDK source format"]')!,"agent_result");await change(node.querySelector('textarea')!,'{"input":"original","output":"answer"}');await act(async()=>button(node,"Read source cases").click());await change(node.querySelector('input[aria-label="Standalone source case ID"]')!,"local");
  const select=node.querySelector('select[aria-label="Test case for Agent result"]')! as HTMLSelectElement;expect(select.options).toHaveLength(2);await change(select,select.options[1].value);
  await act(async()=>button(node,"Import historical report").click());await act(async()=>root.render(render("b")));await act(async()=>root.render(render()));await act(async()=>resolve(report));expect(done).not.toHaveBeenCalled();
 }finally{await act(async()=>root.unmount());node.remove()}
});
it("keeps expectation promotion reviewed and hosted regrading separately quoted",async()=>{
 const {SDKReportDetail}=await import("./sdk-report-detail");const calls:{path:string;body:Record<string,unknown>}[]=[];
 const full={...report,source_kind:"eval_report" as const,cases:[{source_case_id:"local",case_key:"real"}]};
 transport.fetch.mockImplementation(async(path:string,_auth:unknown,init?:RequestInit)=>{
  const body=init?.body?JSON.parse(String(init.body)):undefined;if(body)calls.push({path,body});
  if(path.includes("/case?"))return {case_key:"real",version:"a",verdict:"FAIL",output:"original reply",expected:"Imported expectation",checks:[],evidence_hash:"case-hash"};
  if(path.includes("grade-reviews"))return [];
  if(path.endsWith("run-quote"))return {id:"quote",cases:1,calls:1,max_cost_nano_usd:1000,expires_at:"2027-01-01T00:00:00Z"};
  if(path.endsWith("messages"))return {id:"new-hosted-run"};if(path.endsWith("promote"))return {id:"new-version"};throw Error("Unexpected path "+path);
 });
 const node=document.createElement("div");document.body.append(node);const root=createRoot(node);const promoted=vi.fn(),regraded=vi.fn();
 try{
  await act(async()=>root.render(<SDKReportDetail {...props} report={full} revision={2} models={{assistant:"a",target:"t",evaluator:"j"}} inferenceAvailable onPromoted={promoted} onRegraded={regraded}/>));
  await change(node.querySelector('select[aria-label="SDK detail source case"]')!,"local");expect(button(node,"Create version with this expectation").disabled).toBe(true);
  await act(async()=>{(node.querySelector('input[type="checkbox"]') as HTMLInputElement).click()});await act(async()=>button(node,"Create version with this expectation").click());expect(promoted).toHaveBeenCalledWith({id:"new-version"});expect(calls[0].body).toMatchObject({reviewed:true,source_case_id:"local",source_evidence_hash:"hash",expected:"Imported expectation"});
  await act(async()=>button(node,"Estimate hosted regrade").click());expect(regraded).not.toHaveBeenCalled();expect(calls.some(c=>c.path.endsWith("messages"))).toBe(false);
  await act(async()=>button(node,"Confirm separate hosted regrade").click());const submitted=calls.find(c=>c.path.endsWith("messages"))!.body;expect(submitted).toMatchObject({purpose:"regrade",regrade_source_result_ref:report.binding.result_ref,run_quote_id:"quote",source_evidence_hash:"hash"});expect(submitted.baseline_id).toBeUndefined();expect(regraded).toHaveBeenCalledWith({id:"new-hosted-run"});expect(full.source_kind).toBe("eval_report");
 }finally{await act(async()=>root.unmount());node.remove()}
});
it("keeps a newer pasted source when an earlier file read finishes late",async()=>{
 transport.fetch.mockResolvedValue([{artifact_id:"a",cases:[],published:[]}]);let resolve!:(v:ArrayBuffer)=>void;
 const file=new File(['{"input":"old"}'],"old.json",{type:"application/json"});Object.defineProperty(file,"arrayBuffer",{value:()=>new Promise<ArrayBuffer>(r=>resolve=r)});
 const node=document.createElement("div");document.body.append(node);const root=createRoot(node);
 try{await act(async()=>root.render(<SDKReportImportForm {...props} onImported={()=>{}}/>));await change(node.querySelector('select[aria-label="SDK source format"]')!,"agent_result");const input=node.querySelector('input[type="file"]')!;Object.defineProperty(input,"files",{value:[file]});await act(async()=>input.dispatchEvent(new Event("change",{bubbles:true})));await change(node.querySelector("textarea")!,'{"input":"newer"}');await act(async()=>resolve(new TextEncoder().encode('{"input":"old"}').buffer));expect(node.querySelector("textarea")!.value).toBe('{"input":"newer"}');}finally{await act(async()=>root.unmount());node.remove()}
});
it("preserves the selected review draft while a session event refreshes report history",async()=>{
 const selected={...report,cases:[{source_case_id:"local",case_key:"real"}]};let reads=0;let resolve!:(v:SDKReport[])=>void;
 transport.fetch.mockImplementation(async(path:string)=>{if(path.endsWith("/reports")){if(++reads===1)return [selected];return new Promise<SDKReport[]>(r=>resolve=r)}if(path.includes("/case?"))return {case_key:"real",version:"a",verdict:"FAIL",output:"reply",checks:[],evidence_hash:"hash"};return []});
 const node=document.createElement("div");document.body.append(node);const root=createRoot(node);
 try{await act(async()=>root.render(<SDKReports {...props} selectedResult={selected.binding} refresh={1}/>));await change(node.querySelector('select[aria-label="SDK detail source case"]')!,"local");await change(node.querySelector('textarea[aria-label="SDK grade review reason"]')!,"My unsent review");
 await act(async()=>root.render(<SDKReports {...props} selectedResult={selected.binding} refresh={2}/>));expect(node.querySelector<HTMLTextAreaElement>('textarea[aria-label="SDK grade review reason"]')?.value).toBe("My unsent review");
 await act(async()=>resolve([selected]));expect(node.querySelector<HTMLTextAreaElement>('textarea[aria-label="SDK grade review reason"]')?.value).toBe("My unsent review");
 }finally{await act(async()=>root.unmount());node.remove()}
});
