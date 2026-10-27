import React, {act} from "react";
import {createRoot} from "react-dom/client";
import {afterEach, expect, it, vi} from "vitest";
import {ConnectorSetup} from "./connector-setup";
import type {ConnectorSettingsProps} from "./connector-settings";
const mocks=vi.hoisted(()=>({mapping:vi.fn(),get:vi.fn(),settings:vi.fn()}));
vi.mock("@/lib/vibe-reports",()=>({loadArtifactCaseMapping:mocks.mapping}));
vi.mock("@/lib/api/client",()=>({createApiClient:()=>({get:mocks.get})}));
vi.mock("./connector-settings",()=>({ConnectorSettings:(p:ConnectorSettingsProps)=>{mocks.settings(p);return <p>{p.publication?.challenge_input_set_id || "No publication selected"}</p>}}));
const publication={draft_id:"retained-deleted-draft",challenge_pack_version_id:"version",input_set_key:"exact-key",published_case_id:"case"};
const mapping=[{published:[publication]}];
const props=():Omit<ConnectorSettingsProps,"publication">=>({sessionId:"project",workspaceId:"workspace",artifactId:"artifact",actorScope:"actor",token:async()=>"token",access:{status:"ready",canRead:true,canWrite:true,canAdmin:true},onConfigured:vi.fn(),onRunCreated:vi.fn()});
afterEach(()=>{vi.clearAllMocks();vi.unstubAllGlobals()});
async function render(p=props()){
 vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT",true);mocks.mapping.mockResolvedValue(mapping);
 const node=document.createElement("div"),root=createRoot(node);document.body.append(node);
 await act(async()=>root.render(<ConnectorSetup {...p}/>));
 return {node,root,p,close:async()=>{await act(async()=>root.unmount());node.remove()}};
}
async function choose(node:HTMLElement){await act(async()=>{const el=node.querySelector("select")!;el.selectedIndex=1;el.dispatchEvent(new Event("change",{bubbles:true}))})}
it("resolves the exact retained publication input key and rejects ambiguous UUIDs",async()=>{
 const view=await render();try{
  mocks.get.mockResolvedValue({items:[{id:"wrong",challenge_pack_version_id:"version",input_key:"other"},{id:"right",challenge_pack_version_id:"version",input_key:"exact-key"}]});
  await choose(view.node);expect(mocks.settings.mock.lastCall?.[0].publication).toMatchObject({draft_id:"retained-deleted-draft",challenge_input_set_id:"right",case_count:1});
  mocks.get.mockResolvedValue({items:[{id:"one",challenge_pack_version_id:"version",input_key:"exact-key"},{id:"two",challenge_pack_version_id:"version",input_key:"exact-key"}]});
  await choose(view.node);expect(view.node.textContent).toContain("missing or ambiguous");expect(mocks.settings.mock.lastCall?.[0].publication).toBeUndefined();
 }finally{await view.close()}
});
it("does not fetch old mappings after an async token survives an artifact round trip",async()=>{
 let resolve!:(token:string)=>void;const pending=new Promise<string>(r=>{resolve=r});const p={...props(),token:()=>pending};const view=await render(p);
 try{await act(async()=>view.root.render(<ConnectorSetup {...p} artifactId="other"/>));await act(async()=>view.root.render(<ConnectorSetup {...p}/>));await act(async()=>resolve("token"));expect(mocks.mapping).toHaveBeenCalledTimes(1);expect(mocks.mapping).toHaveBeenCalledWith("project","artifact","token")}finally{await view.close()}
});
