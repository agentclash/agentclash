import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { expect, it, vi } from "vitest";
import { useVibeExport } from "@/lib/use-vibe-export";
import { exportDocument, type ExportPreparation } from "@/lib/vibe-export";
import type { Artifact, Session } from "@/lib/vibe";
import type { SDKReport } from "@/lib/vibe-reports";
const transport=vi.hoisted(()=>({fetch:vi.fn(),bytes:vi.fn()}));
vi.mock("@/lib/vibe",async()=>({...await vi.importActual("@/lib/vibe"),vibeFetch:transport.fetch,vibeBytes:transport.bytes}));
it("collects source-qualified SDK exports without passing SDK IDs as operations",async()=>{
 const bytes=new TextEncoder().encode(' {"input":"hello","output":"\\u0041"}\n');const hash=Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256",bytes)),b=>b.toString(16).padStart(2,"0")).join("");
 const ref={kind:"sdk_report" as const,id:"report"};const report={id:"report",binding:{session_id:"project",artifact_id:"artifact",result_ref:ref},content_sha256:hash} as SDKReport;
 const preparation={format:"agentclash-evaluation-v2",session_id:"project",artifact_id:"artifact",selection:{scope:"selected",artifact_id:"artifact",result_ref:ref},sdk_reports:[report],items:[{kind:"sdk_report",id:"report",result_ref:ref,artifact_id:"artifact",hash}],runs:[],artifacts:[],rules:[],case_mappings:[],omissions:[]} as ExportPreparation;
 transport.fetch.mockResolvedValue(preparation);transport.bytes.mockResolvedValue(bytes);
 let controller!:ReturnType<typeof useVibeExport>;
 function Probe(){const value=useVibeExport("actor",{id:"project"} as Session,async()=>"token");React.useEffect(()=>{controller=value});return null}
 const node=document.createElement("div");const root=createRoot(node);
 try{await act(async()=>root.render(<React.StrictMode><Probe/></React.StrictMode>));await act(async()=>controller.startResult({id:"artifact"} as Artifact,ref));const request=JSON.parse(transport.fetch.mock.calls[0][2].body);expect(request.result_ref).toEqual(ref);expect(request.result_id).toBeUndefined();expect(controller.state?.error).toBe("");const doc=exportDocument(controller.state!.job!);expect(doc.runs).toEqual([]);expect(doc.manifest.completeness).toBe("exact");expect(doc.sdk_reports).toEqual([report]);expect(atob((doc.evidence[0].value as {content:string}).content)).toBe(new TextDecoder().decode(bytes));}finally{await act(async()=>root.unmount())}
});
