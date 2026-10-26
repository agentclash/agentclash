import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { expect, it, vi } from "vitest";
import { useVibeWorkspaceAccess } from "@/lib/use-vibe-workspace-access";
const api=vi.hoisted(()=>({get:vi.fn()}));
vi.mock("@/lib/api/client",()=>({createApiClient:()=>api}));
it("fails closed across actor changes and ignores stale workspace permissions",async()=>{
 let resolve!:(v:unknown)=>void;api.get.mockReturnValueOnce(new Promise(r=>resolve=r)).mockResolvedValue({organizations:[{role:"org_member",workspaces:[{id:"workspace",role:"workspace_viewer"}]}]});
 let result!:ReturnType<typeof useVibeWorkspaceAccess>;
 function Probe({actor}:{actor:string}){result=useVibeWorkspaceAccess(actor,"workspace",async()=>actor);return <p>{result.access.status}:{String(result.access.canWrite)}</p>}
 const node=document.createElement("div");document.body.append(node);const root=createRoot(node);
 try{await act(async()=>root.render(<Probe actor="a"/>));expect(result.access).toMatchObject({status:"loading",canRead:false,canWrite:false});await act(async()=>root.render(<Probe actor="b"/>));await act(async()=>root.render(<Probe actor="a"/>));expect(result.access).toMatchObject({status:"ready",canRead:true,canWrite:false});await act(async()=>resolve({organizations:[{role:"org_admin",workspaces:[{id:"workspace",role:"workspace_admin"}]}]}));expect(result.access.canWrite).toBe(false);}finally{await act(async()=>root.unmount());node.remove()}
});
