import { describe, it, expect } from "vitest";
import { parseResultSelection, formatResultSelection } from "./vibe-result-selection";
import { workspaceEvidenceAccess } from "./use-vibe-workspace-access";
import { reportSourceCases, sdkExportValue } from "./vibe-reports";
import type { UserMeResponse } from "./api/types";
import type { SDKReport } from "./vibe-reports";
const id="01234567-89ab-4def-8123-456789abcdef";
describe("external evidence identity",()=>{
 it("roundtrips qualified sources while retaining legacy operation URLs",()=>{
  for(const kind of ["sdk_report","run","eval_session","vibe_operation"] as const){const ref={kind,id};expect(parseResultSelection(formatResultSelection(ref))).toEqual(ref)}
  expect(formatResultSelection({kind:"vibe_operation",id})).toBe(id);
  for(const value of ["",`sdk:${id}`,"sdk_report:foreign",`${id}:more`,`sdk_report:${id}:more`])expect(parseResultSelection(value)).toBeUndefined();
 });
 it("uses only the exact bound workspace and inherits organization admin",()=>{
  const me={organizations:[{role:"org_member",workspaces:[{id:"viewer",role:"workspace_viewer"},{id:"member",role:"workspace_member"}]},{role:"org_admin",workspaces:[{id:"admin",role:"workspace_viewer"}]}]} as UserMeResponse;
  expect(workspaceEvidenceAccess(me,"viewer")).toEqual({status:"ready",canRead:true,canWrite:false,canAdmin:false});
  expect(workspaceEvidenceAccess(me,"member").canWrite).toBe(true);
  expect(workspaceEvidenceAccess(me,"member").canAdmin).toBe(false);
  expect(workspaceEvidenceAccess(me,"admin").canAdmin).toBe(true);
  expect(workspaceEvidenceAccess(me,"foreign").canRead).toBe(false);
 });
 it("requires an explicit standalone binding and preserves arbitrary report case IDs",()=>{
  expect(reportSourceCases('{"input":"original","output":"reply"}',"agent_result")).toEqual([{id:"",label:"Agent result"}]);
  expect(reportSourceCases('{"cases":[{"case":{"case_id":"local beta","name":"Title"}},{"case":{"case_id":"local alpha"}}]}',"eval_report").map(c=>c.id)).toEqual(["local beta","local alpha"]);
 });
 it("exports frozen metadata with exact source bytes and rejects a substituted descriptor",async()=>{
  const bytes=new TextEncoder().encode(' {"input":"original","output":"\\u0041"}\n');
  const hash=Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256",bytes)),b=>b.toString(16).padStart(2,"0")).join("");
  const report={id,binding:{session_id:"session",artifact_id:"artifact",result_ref:{kind:"sdk_report",id}},content_sha256:hash} as SDKReport;
  const item={kind:"sdk_report" as const,id,artifact_id:"artifact",result_ref:report.binding.result_ref,hash};
  const value=await sdkExportValue(item,[report],bytes,"session");
  expect(atob(value.content)).toBe(' {"input":"original","output":"\\u0041"}\n');expect(value.metadata).toEqual(report);
  await expect(sdkExportValue({...item,artifact_id:"foreign"},[report],bytes,"session")).rejects.toThrow("identity");
  await expect(sdkExportValue(item,[report],new TextEncoder().encode('{}'),"session")).rejects.toThrow("hash");
 });
});
it("never treats matching uploaded fingerprint claims as a controlled comparison",async()=>{
 const {compareResultEvidence}=await import("./vibe-result-evidence");
 const base={evidence_metadata:{origin:"imported",fingerprints:{model:"m",build:"b",test:"t",judge:"j",config:"c"}},result_scope:{artifact_id:"a",case_keys:["one"]}} as unknown as import("./vibe").Operation;
 expect(compareResultEvidence(base,base).controlled).toBe(false);
});
