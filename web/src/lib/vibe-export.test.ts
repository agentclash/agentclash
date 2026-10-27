import { describe, expect, it, vi } from "vitest";
import { exportRuns } from "./vibe-export";
import type { Session, CaseResult } from "./vibe";

const session = {operations: [{id: "run-a", results: [{case_key: "one"}, {case_key: "two"}]}]} as Session;

describe("saved evidence export", () => {
  it("never reads unrelated historical evidence when a run is chosen", async () => {
    const chosen = {...session, operations: [...session.operations, {id:"unrelated",results:[{case_key:"private"}]}]} as Session;
    const runs = await exportRuns(chosen, async (id,key) => { if(id === "unrelated") throw new Error("Unrelated history was read"); return {case_key:key,output:"Selected reply"} as CaseResult; }, {operationIDs:["run-a"]});
    expect(runs.map(run=>run.id)).toEqual(["run-a"]);
  });
  it("retrieves the original bodies without modifying the redacted snapshot", async () => {
    const load = vi.fn(async (_: string, key: string) => ({case_key: key, input: "actual input", output: "actual reply"}) as CaseResult);
    const runs = await exportRuns(session, load);
    expect(load).toHaveBeenCalledWith("run-a", "one");
    expect(load).toHaveBeenCalledWith("run-a", "two");
    expect(runs[0].results.map(r => r.output)).toEqual(["actual reply", "actual reply"]);
    expect(session.operations[0].results[0].output).toBeUndefined();
  });

  it("does not present an incomplete archive as a successful full export", async () => {
    await expect(exportRuns(session, async () => {throw new Error("Unavailable");})).rejects.toThrow("Unavailable");
  });
});

it("requires partial consent, retries only missing frozen identities, and never changes original totals",async()=>{
 const {newExportJob,collectExport,exportDocument}=await import("./vibe-export");
 const preparation={format:"agentclash-evaluation-v2",session_id:"project",title:"Chosen",captured_at:"2026-10-26T00:00:00Z",artifact_id:"version",selection:{scope:"selected",artifact_id:"version",result_id:"run"},artifacts:[],rules:[],runs:[{id:"run",results:[],scorecard:{total:2,passed:1,failed:1,unknown:0}}],items:["one","two"].map(key=>({kind:"case",id:`run:${key}`,case_key:key,operation_id:"run",artifact_id:"version",hash:`hash-${key}`})),case_mappings:[],omissions:["Unselected versions are omitted."]} as import("./vibe-export").ExportPreparation;
 const job=newExportJob(preparation);let fail=true;
 const load=vi.fn(async(item:import("./vibe-export").ExportItem)=>{if(item.case_key==="two"&&fail)throw Error("Unavailable");return {case_key:item.case_key,version:"version",evidence_hash:item.hash,output:"original"}});
 await collectExport(job,load,()=>true,()=>{});
 expect(()=>exportDocument(job)).toThrow("Confirm");
 const partial=exportDocument(job,true);expect(partial.manifest.completeness).toBe("partial");expect(partial.manifest.missing.map(i=>i.case_key)).toEqual(["two"]);expect(partial.runs[0].scorecard?.total).toBe(2);
 preparation.title="Later title";load.mockClear();fail=false;
 await collectExport(job,load,()=>true,()=>{});expect(load).toHaveBeenCalledTimes(1);expect(load.mock.calls[0][0].case_key).toBe("two");
 const exact=exportDocument(job);expect(exact.title).toBe("Chosen");expect(exact.manifest.completeness).toBe("exact");expect(exact.runs[0].results).toHaveLength(2);
});
it("keeps a late case body with a changed hash explicitly missing",async()=>{
 const {newExportJob,collectExport,exportDocument}=await import("./vibe-export");
 const p={format:"agentclash-evaluation-v2",session_id:"project",title:"Chosen",captured_at:"now",artifact_id:"v",selection:{scope:"selected",artifact_id:"v"},artifacts:[],rules:[],runs:[],items:[{kind:"case",id:"one",artifact_id:"v",case_key:"one",hash:"old"}],case_mappings:[],omissions:[]} as import("./vibe-export").ExportPreparation;
 const job=newExportJob(p);await collectExport(job,async()=>({case_key:"one",version:"v",evidence_hash:"new"}),()=>true,()=>{});
 expect(job.values.size).toBe(0);expect(()=>exportDocument(job)).toThrow("Confirm");expect(exportDocument(job,true).manifest.missing[0].reason).toContain("evidence changed");
});

it("preserves canonical pane case thread and exact version through auth and advanced return",async()=>{
 const {keepReturnURL,advancedURL,safeVibeReturn}=await import("./vibe-keep");
 const path="/vibe-evals?session=s&agent=v&view=tests&case=case-a&thread=t&run=r";
 const auth=new URL(keepReturnURL("s","v","ws","r",path,true),"https://local.invalid");
 for(const [key,value] of Object.entries({session:"s",agent:"v",view:"tests",case:"case-a",thread:"t",run:"r",keep_run:"r",keep_advanced:"1"}))expect(auth.searchParams.get(key)).toBe(value);
 const advanced=new URL(advancedURL("ws","draft",path,"Canonical title"),"https://local.invalid");expect(advanced.searchParams.get("return_to")).toBe(path);expect(advanced.searchParams.get("return_title")).toBe("Canonical title");
 for(const unsafe of ["//foreign.test/vibe-evals","https://foreign.test/vibe-evals","/vibe-evals-other"])expect(safeVibeReturn(unsafe)).toBeUndefined();
});
