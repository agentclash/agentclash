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
  await act(async()=>button(node,"Read source tests").click());
  await change(node.querySelector('input[aria-label="Standalone source test ID"]')!,"arbitrary local id");
  const select=node.querySelector('select[aria-label="Test for Agent result"]')! as HTMLSelectElement;
  expect(select.value).toBe("");await change(select,select.options[1].value);
  await act(async()=>button(node,"Import historical report").click());
  expect(done).not.toHaveBeenCalled();expect(node.textContent).toContain("Try again");fail=false;
  await act(async()=>button(node,"Retry this report import").click());
  expect(calls).toHaveLength(2);expect(calls[1]).toEqual(calls[0]);expect((calls[0] as {source_json:string}).source_json).toBe(raw);expect(done).toHaveBeenCalledWith(report.binding);
 }finally{await act(async()=>root.unmount());node.remove()}
});
it("shows imported results with zero operations and selects the server binding only on request",async()=>{
 transport.fetch.mockResolvedValue([report]);const node=document.createElement("div");document.body.append(node);const root=createRoot(node);const select=vi.fn();
 try{await act(async()=>root.render(<SDKReports {...props} onSelectResult={select}/>));expect(node.textContent).toContain("Imported, unverified");expect(node.textContent).toContain("local");expect(select).not.toHaveBeenCalled();
 const history=node.querySelector<HTMLDetailsElement>('details[aria-label="SDK report history"]');expect(history).not.toBeNull();expect(history!.open).toBe(false);await act(async()=>history!.querySelector("summary")!.click());expect(history!.open).toBe(true);
 await act(async()=>button(node,"Open report local").click());expect(select).toHaveBeenCalledWith(report.binding);}finally{await act(async()=>root.unmount());node.remove()}
});
it("omits empty project SDK history and reveals it when a report exists",async()=>{
 transport.fetch.mockResolvedValue([]);const node=document.createElement("div");document.body.append(node);const root=createRoot(node);const select=vi.fn();
 try{
  await act(async()=>root.render(<SDKReports {...props} hideEmptyHistory refresh={1} onSelectResult={select}/>));
  expect(node.querySelector('[aria-label="SDK report history"]')).toBeNull();
  transport.fetch.mockResolvedValue([report]);
  await act(async()=>root.render(<SDKReports {...props} hideEmptyHistory refresh={2} onSelectResult={select}/>));
  const history=node.querySelector<HTMLDetailsElement>('[aria-label="SDK report history"]')!;expect(history).not.toBeNull();expect(history.open).toBe(false);
  await act(async()=>history.querySelector<HTMLElement>("summary")!.click());await act(async()=>button(node,"Open report local").click());expect(select).toHaveBeenCalledWith(report.binding);
 }finally{await act(async()=>root.unmount());node.remove()}
});
it.each(["signed-out","no-workspace"])("keeps %s SDK report guidance inside optional history",async(state)=>{
 const node=document.createElement("div");document.body.append(node);const root=createRoot(node);
 try{
  await act(async()=>root.render(<SDKReports {...props} selectedReport={report} workspaceId={state==="no-workspace"?undefined:props.workspaceId} access={{...access,status:state==="signed-out"?"signed_out":"ready",canRead:false,canWrite:false}}/>));
  const history=node.querySelector<HTMLDetailsElement>('details[aria-label="SDK report history"]');expect(history).not.toBeNull();expect(history!.open).toBe(false);expect(history!.textContent).toContain(state==="signed-out"?"Sign in to read":"Save to a workspace");expect(node.querySelector('article[aria-label="Imported report detail"]')).toBeNull();
 }finally{await act(async()=>root.unmount());node.remove()}
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
  await act(async()=>root.render(render()));await change(node.querySelector('select[aria-label="SDK source format"]')!,"agent_result");await change(node.querySelector('textarea')!,'{"input":"original","output":"answer"}');await act(async()=>button(node,"Read source tests").click());await change(node.querySelector('input[aria-label="Standalone source test ID"]')!,"local");
  const select=node.querySelector('select[aria-label="Test for Agent result"]')! as HTMLSelectElement;expect(select.options).toHaveLength(2);await change(select,select.options[1].value);
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
  await change(node.querySelector('select[aria-label="SDK detail source test"]')!,"local");expect(button(node,"Create version with this expectation").disabled).toBe(true);
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
 try{await act(async()=>root.render(<SDKReports {...props} selectedReport={selected} refresh={1}/>));await change(node.querySelector('select[aria-label="SDK detail source test"]')!,"local");await change(node.querySelector('textarea[aria-label="SDK grade review reason"]')!,"My unsent review");
 await act(async()=>root.render(<SDKReports {...props} selectedReport={selected} refresh={2}/>));expect(node.querySelector<HTMLTextAreaElement>('textarea[aria-label="SDK grade review reason"]')?.value).toBe("My unsent review");
 await act(async()=>resolve([]));expect(node.querySelector<HTMLTextAreaElement>('textarea[aria-label="SDK grade review reason"]')?.value).toBe("My unsent review");
 }finally{await act(async()=>root.unmount());node.remove()}
});

it("retains unsent review and exact pending receipt through failed list refresh and successful retry",async()=>{
 const selected={...report,cases:[{source_case_id:"local",case_key:"real"}]};let failList=false;let loseReview!:(e:Error)=>void;const writes:string[]=[];
 transport.fetch.mockImplementation(async(path:string,_auth:unknown,init?:RequestInit)=>{
  if(path.endsWith("/reports")){if(failList)throw new VibeError("unavailable","History temporarily unavailable",503);return [selected]}
  if(path.includes("/case?"))return {case_key:"real",version:"a",verdict:"FAIL",output:"reply",checks:[],evidence_hash:"case-hash"};
  if(init?.method==="POST"){writes.push(String(init.body));if(writes.length===1)return new Promise((_resolve,reject)=>{loseReview=reject});return {id:"recorded-review"}}
  return [];
 });
 const node=document.createElement("div");document.body.append(node);const root=createRoot(node);
 const render=(refresh:number)=>root.render(<SDKReports {...props} selectedReport={selected} refresh={refresh}/>);
 const reason=()=>node.querySelector<HTMLTextAreaElement>('textarea[aria-label="SDK grade review reason"]');
 try{
  await act(async()=>render(1));
  const detail=node.querySelector('article[aria-label="Imported report detail"]')!,history=node.querySelector<HTMLDetailsElement>('details[aria-label="SDK report history"]');
  expect(history).not.toBeNull();expect(history!.open).toBe(false);expect(detail.closest("details")).toBeNull();expect(detail.compareDocumentPosition(history!) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  await change(node.querySelector('select')!,"local");await change(reason()!,"Keep this exact review");const retainedReason=reason();
  await act(async()=>history!.querySelector("summary")!.click());expect(history!.open).toBe(true);await act(async()=>history!.querySelector("summary")!.click());expect(history!.open).toBe(false);expect(reason()).toBe(retainedReason);
  failList=true;await act(async()=>render(2));expect(node.textContent).toContain("History temporarily unavailable");expect(reason()?.value).toBe("Keep this exact review");
  expect(node.querySelector('[role="alert"]')!.closest('details[aria-label="SDK report history"]')).toBeNull();
  await act(async()=>history!.querySelector("summary")!.click());failList=false;await act(async()=>button(node,"Reload SDK reports").click());expect(reason()?.value).toBe("Keep this exact review");
  await act(async()=>node.querySelector('form')!.dispatchEvent(new Event("submit",{bubbles:true,cancelable:true})));expect(writes).toHaveLength(1);
  await act(async()=>history!.querySelector("summary")!.click());expect(history!.open).toBe(false);expect(reason()).toBe(retainedReason);
  failList=true;await act(async()=>render(3));expect(reason()?.value).toBe("Keep this exact review");expect(reason()?.disabled).toBe(true);
  await act(async()=>history!.querySelector("summary")!.click());failList=false;await act(async()=>button(node,"Reload SDK reports").click());await act(async()=>history!.querySelector("summary")!.click());expect(history!.open).toBe(false);expect(reason()).toBe(retainedReason);
  await act(async()=>loseReview(new Error("Response lost after submission")));expect(reason()?.value).toBe("Keep this exact review");
  await act(async()=>button(node,"Retry this grade review").click());expect(writes).toHaveLength(2);expect(writes[1]).toBe(writes[0]);expect(node.textContent).toContain("Review recorded");
 }finally{await act(async()=>root.unmount());node.remove()}
});

it.each(["forbidden","not_found","source-removed","revoked-access"])("clears cached private report after authoritative %s",async(loss)=>{
 const selected={...report,cases:[{source_case_id:"local",case_key:"real"}]};let reads=0;let restore!:(v:SDKReport[])=>void;
 transport.fetch.mockImplementation(async(path:string)=>{
  if(path.endsWith("/reports")){
   if(++reads===1)return [selected];
   if(loss==="revoked-access")return new Promise<SDKReport[]>(resolve=>restore=resolve);
   if(loss==="source-removed")return [];
   throw new VibeError(loss,"Report access lost",loss==="forbidden"?403:404);
  }
  return [];
 });
 const node=document.createElement("div");document.body.append(node);const root=createRoot(node);
 try{
  await act(async()=>root.render(<SDKReports {...props} selectedReport={selected} refresh={1}/>));expect(node.querySelector('article[aria-label="Imported report detail"]')).not.toBeNull();
  if(loss==="revoked-access"){
   await act(async()=>root.render(<SDKReports {...props} access={{...access,canRead:false,canWrite:false}} selectedReport={selected} refresh={1}/>));
   await act(async()=>root.render(<SDKReports {...props} refresh={1}/>));
  }else if(loss==="source-removed") await act(async()=>root.render(<SDKReports {...props} refresh={2}/>));
  else await act(async()=>root.render(<SDKReports {...props} selectedReport={selected} refresh={2}/>));
  expect(node.querySelector('article[aria-label="Imported report detail"]')).toBeNull();
  if(restore)await act(async()=>restore([]));
 }finally{await act(async()=>root.unmount());node.remove()}
});

it.each(["unsupported_capability","revision_conflict"])("allows explicit review again after promotion %s without automatic resubmission",async(code)=>{
 const {SDKReportDetail}=await import("./sdk-report-detail");const writes:Record<string,unknown>[]=[];
 const selected={...report,cases:[{source_case_id:"local",case_key:"first"},{source_case_id:"other",case_key:"second"}]};
 transport.fetch.mockImplementation(async(path:string,_auth:unknown,init?:RequestInit)=>{
  if(path.includes("/case?")){const other=path.includes("source_case_id=other");return {case_key:other?"second":"first",version:"a",verdict:"FAIL",output:"reply",expected:other?"Other expectation":"First expectation",checks:[],evidence_hash:"case-hash"}}
  if(path.endsWith("/promote")){writes.push(JSON.parse(String(init?.body)));if(writes.length===1)throw new VibeError(code,"Review again before retrying",code==="revision_conflict"?409:400);return {id:"new-version"}}
  return [];
 });
 const node=document.createElement("div");document.body.append(node);const root=createRoot(node);const done=vi.fn();
 const render=(revision:number)=>root.render(<SDKReportDetail {...props} report={selected} revision={revision} onPromoted={done}/>);
 try{
  await act(async()=>render(2));await change(node.querySelector('select')!,"local");await change(node.querySelector('textarea')!,"Draft stays with the source");
  await act(async()=>node.querySelector<HTMLInputElement>('input[type="checkbox"]')!.click());await act(async()=>button(node,"Create version with this expectation").click());expect(done).not.toHaveBeenCalled();
  await act(async()=>render(3));expect(writes).toHaveLength(1);
  expect(button(node,"Review expectation again")).toBeDefined();await act(async()=>button(node,"Review expectation again").click());
  expect(node.querySelector('textarea')!.value).toBe("Draft stays with the source");expect(node.querySelector('select')!.value).toBe("local");expect(writes).toHaveLength(1);
  if(code==="unsupported_capability")await change(node.querySelector('select')!,"other");
  expect(button(node,"Create version with this expectation").disabled).toBe(true);
  await act(async()=>node.querySelector<HTMLInputElement>('input[type="checkbox"]')!.click());await act(async()=>button(node,"Create version with this expectation").click());
  expect(writes).toHaveLength(2);expect(writes[1].client_id).not.toBe(writes[0].client_id);expect(writes[1]).toMatchObject({revision:3,source_case_id:code==="unsupported_capability"?"other":"local",expected:code==="unsupported_capability"?"Other expectation":"First expectation",reviewed:true});expect(done).toHaveBeenCalledWith({id:"new-version"});
 }finally{await act(async()=>root.unmount());node.remove()}
});

it("lets a rejected grade review be corrected after fresh evidence without losing its text or source",async()=>{
 const {SDKReportDetail}=await import("./sdk-report-detail");const writes:Record<string,unknown>[]=[];let caseReads=0;
 const selected={...report,cases:[{source_case_id:"local",case_key:"real"}]};
 transport.fetch.mockImplementation(async(path:string,_auth:unknown,init?:RequestInit)=>{
  if(path.includes("/case?"))return {case_key:"real",version:"a",verdict:"FAIL",output:"reply",checks:[],evidence_hash:++caseReads===1?"stale":"fresh"};
  if(init?.method==="POST"){writes.push(JSON.parse(String(init.body)));if(writes.length===1)throw new VibeError("evidence_changed","Reload the evidence",409);return {id:"recorded-review"}}
  return [];
 });
 const node=document.createElement("div");document.body.append(node);const root=createRoot(node);
 try{
  await act(async()=>root.render(<SDKReportDetail {...props} report={selected}/>));await change(node.querySelector('select')!,"local");await change(node.querySelector('textarea')!,"Original draft");await act(async()=>button(node,"Record grade review").click());
  expect(button(node,"Edit rejected grade review")).toBeDefined();await act(async()=>button(node,"Edit rejected grade review").click());expect(writes).toHaveLength(1);expect(node.querySelector('textarea')!.value).toBe("Original draft");expect(node.querySelector('select')!.value).toBe("local");
  await change(node.querySelector('textarea')!,"Corrected draft");await act(async()=>button(node,"Record grade review").click());expect(writes).toHaveLength(2);expect(writes[1].client_id).not.toBe(writes[0].client_id);expect(writes[1]).toMatchObject({reason:"Corrected draft",source_case_id:"local",source_evidence_hash:"fresh"});expect(node.textContent).toContain("Review recorded");
 }finally{await act(async()=>root.unmount());node.remove()}
});

it.each([new Error("Response lost"),new VibeError("unauthenticated","Sign in again",401),new VibeError("idempotency_conflict","Receipt differs",409),new VibeError("request_failed","Unknown conflict",409)])("keeps the exact promotion request for uncertain outcome $message",async(failure)=>{
 const {SDKReportDetail}=await import("./sdk-report-detail");const writes:string[]=[];
 const selected={...report,cases:[{source_case_id:"local",case_key:"real"}]};
 transport.fetch.mockImplementation(async(path:string,_auth:unknown,init?:RequestInit)=>{
  if(path.includes("/case?"))return {case_key:"real",version:"a",verdict:"FAIL",output:"reply",expected:"Imported expectation",checks:[],evidence_hash:"case-hash"};
  if(path.endsWith("/promote")){writes.push(String(init?.body));if(writes.length===1)throw failure;return {id:"already-created-version"}}
  return [];
 });
 const node=document.createElement("div");document.body.append(node);const root=createRoot(node);const done=vi.fn();
 try{
  await act(async()=>root.render(<SDKReportDetail {...props} report={selected} revision={2} onPromoted={done}/>));await change(node.querySelector('select')!,"local");await act(async()=>node.querySelector<HTMLInputElement>('input[type="checkbox"]')!.click());await act(async()=>button(node,"Create version with this expectation").click());
  await act(async()=>root.render(<SDKReportDetail {...props} report={selected} revision={9} onPromoted={done}/>));expect(button(node,"Review expectation again")).toBeUndefined();expect(node.querySelector('select')!.disabled).toBe(true);expect(writes).toHaveLength(1);
  await act(async()=>button(node,"Retry expectation promotion").click());expect(writes).toHaveLength(2);expect(writes[1]).toBe(writes[0]);expect(done).toHaveBeenCalledWith({id:"already-created-version"});
 }finally{await act(async()=>root.unmount());node.remove()}
});

it("retries an unavailable project history without showing an empty SDK report history",async()=>{
 vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT",true);transport.fetch.mockRejectedValue(new Error("History unavailable"));const node=document.createElement("div");document.body.append(node);const root=createRoot(node);
 try{
  await act(async()=>root.render(<SDKReports {...props} selectedReport={undefined} hideEmptyHistory/>));
  expect(node.querySelector('[aria-label="SDK report history"]')).toBeNull();expect(node.querySelector('[role="alert"]')!.textContent).toBe("History unavailable");
  transport.fetch.mockResolvedValue([]);await act(async()=>[...node.querySelectorAll("button")].find(b=>b.textContent==="Reload SDK reports")!.click());
  expect(node.textContent).toBe("");
 }finally{await act(async()=>root.unmount());node.remove()}
});
