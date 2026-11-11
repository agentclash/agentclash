import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, expect, it, vi } from "vitest";
import { SDKReportImportForm } from "./sdk-report-import";
import { SDKReportDetail } from "./sdk-report-detail";
import type { SDKReport } from "@/lib/vibe-reports";
const transport=vi.hoisted(()=>({fetch:vi.fn(),bytes:vi.fn()}));
vi.mock("@/lib/vibe",async()=>({...await vi.importActual("@/lib/vibe"),vibeFetch:transport.fetch,vibeBytes:transport.bytes}));
afterEach(()=>vi.resetAllMocks());
const props={sessionId:"s",workspaceId:"w",artifactId:"a",actorScope:"user-a",access:{status:"ready" as const,canRead:true,canWrite:true,canAdmin:false},token:async()=>"token"};
const config={sdk_report_import_formats:[{envelope_version:1,source_kind:"eval_report",source_schema_version:2}]};
const report={id:"r",binding:{session_id:"s",artifact_id:"a",result_ref:{kind:"sdk_report",id:"r"}},source_schema_version:2,source_kind:"eval_report",external_report_id:"demo",cases:[{source_case_id:"demo",case_key:"real"}],content_sha256:"hash"} as SDKReport;
const source=JSON.stringify({schema_version:2,report_id:"demo",target_revision:null,cases:[{case:{case_id:"demo",name:"<img src=x onerror=alert(1)>"},status:"incomplete",target_invoked:false,checks:[{check_id:"outcome",passed:false}],evidence:{prompt:"PRIVATE TRACE",tool_calls:[{arguments:"PRIVATE TRACE"}]}}]});
const button=(node:HTMLElement,label:string)=>Array.from(node.querySelectorAll("button")).find(b=>b.textContent===label)!;
async function change(el:HTMLTextAreaElement|HTMLSelectElement,value:string){await act(async()=>{const proto=el instanceof HTMLSelectElement?HTMLSelectElement.prototype:HTMLTextAreaElement.prototype;Object.getOwnPropertyDescriptor(proto,"value")!.set!.call(el,value);el.dispatchEvent(new Event(el instanceof HTMLSelectElement?"change":"input",{bubbles:true}));});}
async function setup(node:HTMLElement){await change(node.querySelector('select[aria-label="SDK source format"]')!,"eval_report_v2");await change(node.querySelector("textarea")!,source);await act(async()=>button(node,"Read source tests").click());const select=node.querySelector<HTMLSelectElement>('select[aria-label^="Test for"]')!;expect(select.value).toBe("");await change(select,select.options[1].value);}
function mapping(){return [{artifact_id:"a",cases:[{case_key:"real",input_set_key:"set",challenge_key:"challenge"}],published:[]}]}
it("gates v2 on server discovery, escapes preview and saves only explicit mapped original with an immutable retry",async()=>{
 const writes:string[]=[];transport.fetch.mockImplementation(async(path:string,auth:unknown,init?:RequestInit)=>{expect(auth).toBe("token");if(path==="/config")return config;if(path.endsWith("case-mapping"))return mapping();writes.push(String(init?.body));if(writes.length===1)throw Error("response lost");return {...report,content_sha256:Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256",new TextEncoder().encode(source))),b=>b.toString(16).padStart(2,"0")).join("")}});
 const node=document.createElement("div");document.body.append(node);const root=createRoot(node),done=vi.fn();
 try{await act(async()=>root.render(<SDKReportImportForm {...props} onImported={done}/>));await setup(node);expect(writes).toHaveLength(0);
 const preview=node.querySelector('[aria-label="Refund v2 safe preview"]')!;expect(preview.textContent).toContain('<img src=x onerror=alert(1)>');expect(preview.querySelector("img")).toBeNull();expect(preview.textContent).not.toContain("PRIVATE TRACE");expect(preview.textContent).toContain('"cost_usd": null');
 await act(async()=>button(node,"Import historical report").click());await act(async()=>button(node,"Retry this report import").click());expect(writes).toHaveLength(2);expect(writes[1]).toBe(writes[0]);const sent=JSON.parse(writes[0]);expect(sent).toMatchObject({envelope_version:1,source_kind:"eval_report",source_schema_version:2,source_json:source,metadata:{application_revision:null,capture_scope:{tools:{scope:"unknown",instrumentation:"unknown"}}}});await act(async()=>{await vi.waitFor(()=>expect(done).toHaveBeenCalledWith(report.binding))});
 }finally{await act(async()=>root.unmount());node.remove()}
});
it("keeps v1 available on an old server and never probes v2 by writing",async()=>{
 transport.fetch.mockImplementation(async(path:string)=>path==="/config"?{}:mapping());const node=document.createElement("div");document.body.append(node);const root=createRoot(node);
 try{await act(async()=>root.render(<SDKReportImportForm {...props} onImported={()=>{}}/>));expect(node.querySelector<HTMLOptionElement>('option[value="eval_report_v2"]')!.disabled).toBe(true);await change(node.querySelector("textarea")!,source);await act(async()=>button(node,"Read source tests").click());expect(node.textContent).toContain("explicit source format");expect(button(node,"Import historical report").disabled).toBe(true);expect(transport.fetch.mock.calls.every(c=>!c[2]?.method)).toBe(true)}finally{await act(async()=>root.unmount());node.remove()}
});
it.each(["actor","workspace","write access"])("drops a pending v2 receipt when %s changes",async(kind)=>{
 let finish!:(v:SDKReport)=>void;transport.fetch.mockImplementation(async(path:string)=>path==="/config"?config:path.endsWith("case-mapping")?mapping():new Promise<SDKReport>(r=>finish=r));
 const node=document.createElement("div");document.body.append(node);const root=createRoot(node),done=vi.fn();
 try{await act(async()=>root.render(<SDKReportImportForm {...props} onImported={done}/>));await setup(node);await act(async()=>button(node,"Import historical report").click());await act(async()=>root.render(<SDKReportImportForm {...props} {...(kind==="actor"?{actorScope:"user-b"}:kind==="workspace"?{workspaceId:"different"}:{access:{...props.access,canWrite:false}})} onImported={done}/>));await act(async()=>finish(report));expect(done).not.toHaveBeenCalled();expect(node.querySelector("textarea")?.value ?? "").toBe("")}finally{await act(async()=>root.unmount());node.remove()}
});
it("reopens v2 provenance and disables v1-only hosted actions before a quote",async()=>{
 transport.fetch.mockResolvedValue([]);const node=document.createElement("div");document.body.append(node);const root=createRoot(node);
 try{await act(async()=>root.render(<SDKReportDetail {...props} report={{...report,source_provenance:{target_revision:"source-revision",cost_usd:null}}} revision={1} models={{assistant:"a",target:"t",evaluator:"j"}} inferenceAvailable/>));expect(node.textContent).toContain("source-revision");expect(node.textContent).toContain("Imported source v2");expect(button(node,"Estimate hosted regrade").disabled).toBe(true);expect(node.textContent).toContain("does not support hosted regrading");expect(transport.fetch.mock.calls.some(c=>c[0].includes("run-quote"))).toBe(false);expect(button(node,"Download exact original").disabled).toBe(false)}finally{await act(async()=>root.unmount());node.remove()}
});
it("retains an unconfirmed v2 request when the returned original digest differs",async()=>{
 transport.fetch.mockImplementation(async(path:string)=>path==="/config"?config:path.endsWith("case-mapping")?mapping():report);
 const node=document.createElement("div");document.body.append(node);const root=createRoot(node),done=vi.fn();
 try{await act(async()=>root.render(<SDKReportImportForm {...props} onImported={done}/>));await setup(node);await act(async()=>button(node,"Import historical report").click());await vi.waitFor(async()=>{await act(async()=>{await Promise.resolve()});expect(node.textContent).toContain("different original report identity")});expect(done).not.toHaveBeenCalled();expect(button(node,"Retry this report import")).toBeDefined();expect(node.querySelector("textarea")!.value).toBe(source)}finally{await act(async()=>root.unmount());node.remove()}
});
