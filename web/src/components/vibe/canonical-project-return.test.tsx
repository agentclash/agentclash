import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { expect, it, vi } from "vitest";
import { CanonicalProjectReturn } from "./canonical-project-return";
const mocks=vi.hoisted(()=>({fetch:vi.fn(),user:{id:"owner"} as {id:string}|undefined,token:async()=>"token"}));
vi.mock("@/lib/vibe",()=>({vibeFetch:mocks.fetch}));
vi.mock("@workos-inc/authkit-nextjs/components",()=>({useAuth:()=>({user:mocks.user}),useAccessToken:()=>({getAccessToken:mocks.token})}));
it("renders the authorized exact source return and discards it across actor changes",async()=>{
 vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT",true);
 const node=document.createElement("div"),root=createRoot(node);document.body.append(node);
 const binding={session_id:"private-project",artifact_id:"version",result_ref:{kind:"run",id:"canonical"}};
 mocks.fetch.mockResolvedValue([{binding,url:"https://untrusted.invalid/ignored"}]);
 const render=()=>root.render(<CanonicalProjectReturn runId="canonical" workspaceId="workspace"/>);
 try{
  await act(async()=>render());
  const link=node.querySelector("a")!;
  const target=new URL(link.href);
  expect(target.pathname).toBe("/vibe-evals");expect(target.searchParams.get("session")).toBe("private-project");expect(target.searchParams.get("agent")).toBe("version");expect(target.searchParams.get("run")).toBe("run:canonical");
  let resolve!:(v:unknown)=>void;
  mocks.fetch.mockReturnValue(new Promise(r=>{resolve=r}));mocks.user={id:"other"};
  await act(async()=>render());expect(node.querySelector("a")).toBeNull();
  mocks.user=undefined;await act(async()=>render());
  await act(async()=>resolve([{binding}]));expect(node.querySelector("a")).toBeNull();
 }finally{await act(async()=>root.unmount());node.remove();vi.unstubAllGlobals();mocks.user={id:"owner"};mocks.fetch.mockReset()}
});
