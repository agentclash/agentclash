import React,{act} from "react";
import {createRoot} from "react-dom/client";
import {afterEach,expect,it,vi} from "vitest";
import {ConnectorSettings,type ConnectorSettingsProps} from "./connector-settings";
const client=vi.hoisted(()=>({list:vi.fn(),save:vi.fn(),approve:vi.fn(),revoke:vi.fn(),run:vi.fn()}));
vi.mock("@/lib/connector-client",()=>({connectorClient:()=>client}));
const binding={snapshot_id:"snapshot",deployment_id:"deployment",workspace_id:"workspace",target_origin:"https://example.com",secret_id:"secret",secret_revision:2,approved:true,config:{adapter:"http_json",endpoint_url:"https://example.com/invoke",secret_key:"APP_TOKEN",auth:"bearer",response_pointer:"/answer",timeout_seconds:60}};
function props(overrides:Partial<ConnectorSettingsProps>={}):ConnectorSettingsProps{return {sessionId:"session",workspaceId:"workspace",artifactId:"artifact",actorScope:"user",token:async()=>"token",access:{status:"ready",canRead:true,canWrite:true,canAdmin:true},onRunCreated:vi.fn(),...overrides}}
afterEach(()=>{vi.clearAllMocks();vi.unstubAllGlobals()});
async function render(p:ConnectorSettingsProps, connectors:unknown[]=[binding]){vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT",true);client.list.mockResolvedValue({connectors});const node=document.createElement("div");document.body.append(node);const root=createRoot(node);await act(async()=>root.render(<ConnectorSettings {...p}/>));return {node,root,close:async()=>{await act(async()=>root.unmount());node.remove()}}}
async function select(node:HTMLElement,index:number,value:string){await act(async()=>{const el=node.querySelectorAll("select")[index];el.value=value;el.dispatchEvent(new Event("change",{bubbles:true}))})}
it("keeps guests text-only without accessing the connector API",async()=>{const view=await render(props({token:async()=>null,access:{status:"signed_out",canRead:false,canWrite:false,canAdmin:false}}));try{expect(view.node.textContent).toContain("Guest projects support text only");expect(client.list).not.toHaveBeenCalled();expect(view.node.querySelector("select")).toBeNull()}finally{await view.close()}});
it("lets members select approved connectors but never configure or approve",async()=>{const p=props({access:{status:"ready",canRead:true,canWrite:true,canAdmin:false}}),view=await render(p);try{await select(view.node,0,"http_json");await select(view.node,1,"snapshot");expect(view.node.querySelector("form")).toBeNull();expect(view.node.textContent).toContain("Select an exact published test version");expect(view.node.querySelectorAll("select")[1].value).toBe("snapshot");expect(client.run).not.toHaveBeenCalled();expect(client.approve).not.toHaveBeenCalled()}finally{await view.close()}});
it("separates saving and approval from run admission",async()=>{client.save.mockResolvedValue({...binding,approved:false});client.approve.mockResolvedValue(binding);const p=props(),view=await render(p);try{await select(view.node,0,"http_json");await act(async()=>view.node.querySelector("form")!.dispatchEvent(new Event("submit",{bubbles:true,cancelable:true})));expect(client.save).toHaveBeenCalledOnce();expect(client.run).not.toHaveBeenCalled();const approve=Array.from(view.node.querySelectorAll("button")).find(b=>b.textContent==="Approve connector")!;expect(approve.disabled).toBe(true);await act(async()=>view.node.querySelector<HTMLInputElement>('input[type="checkbox"]')!.click());await act(async()=>approve.click());expect(client.approve).toHaveBeenCalledWith(expect.objectContaining({secret_revision:2,target_origin:"https://example.com"}));expect(view.node.textContent).toContain("Connector approved. Running tests is a separate action.");expect(client.run).not.toHaveBeenCalled()}finally{await view.close()}});
it("does not repeat run admission or report success after a project round trip",async()=>{let resolve!:(v:{id:string})=>void;client.run.mockImplementation(()=>new Promise(r=>{resolve=r}));const p=props({publication:{draft_id:"retained",artifact_id:"artifact",challenge_pack_version_id:"version",challenge_input_set_id:"inputs",input_set_key:"default",case_count:2}}),view=await render(p);try{await select(view.node,0,"http_json");await select(view.node,1,"snapshot");const run=Array.from(view.node.querySelectorAll("button")).find(b=>b.textContent==="Run published tests")!;await act(async()=>run.click());await act(async()=>run.click());expect(client.run).toHaveBeenCalledOnce();await act(async()=>view.root.render(<ConnectorSettings {...p} artifactId="different"/>));await act(async()=>view.root.render(<ConnectorSettings {...p}/>));await act(async()=>resolve({id:"created"}));expect(p.onRunCreated).not.toHaveBeenCalled()}finally{await view.close()}});

it("discards pending approval completion after workspace permission loss",async()=>{let resolve!:(v:typeof binding)=>void;client.approve.mockImplementation(()=>new Promise(r=>{resolve=r}));const p=props(),view=await render(p);try{client.save.mockResolvedValue({...binding,approved:false});await select(view.node,0,"http_json");await act(async()=>view.node.querySelector("form")!.dispatchEvent(new Event("submit",{bubbles:true,cancelable:true})));await act(async()=>view.node.querySelector<HTMLInputElement>('input[type="checkbox"]')!.click());const approve=Array.from(view.node.querySelectorAll("button")).find(b=>b.textContent==="Approve connector")!;await act(async()=>approve.click());await act(async()=>view.root.render(<ConnectorSettings {...p} access={{status:"ready",canRead:true,canWrite:false,canAdmin:false}}/>));await act(async()=>resolve(binding));expect(view.node.textContent).not.toContain("Connector approved. Running tests is a separate action.")}finally{await view.close()}});

// Approval and execution must describe the saved snapshot, not the independent new form.
it("reviews the exact frozen contract when switching same-origin saved snapshots", async()=>{
 const first={...binding,approved:false,snapshot_id:"snapshot-orders",config:{...binding.config,endpoint_url:"https://example.com/orders/invoke",input_fields:{order:"/order_id"},literal_json:{region:"us",retries:0},response_pointer:"/result/order",timeout_seconds:75}};
 const second={...binding,approved:false,snapshot_id:"snapshot-refunds",secret_revision:9,config:{...binding.config,endpoint_url:"https://example.com/refunds/invoke",secret_key:"REFUND_TOKEN",auth:"api_key",input_fields:{refund:"/refund_id"},literal_json:{region:"eu",sandbox:true},response_pointer:"/result/refund",timeout_seconds:120}};
 const view=await render(props(),[first,second]);
 try{
  await select(view.node,0,"http_json");
  const choices=Array.from(view.node.querySelectorAll("select")[1].options).map(option=>option.textContent);
  expect(choices[1]).toContain("https://example.com/orders/invoke");expect(choices[1]).toContain("snapshot-orders");
  expect(choices[2]).toContain("https://example.com/refunds/invoke");expect(choices[2]).toContain("snapshot-refunds");
  for(const [snapshot,values,absent] of [
   [first,["https://example.com/orders/invoke","Bearer token","APP_TOKEN","revision 2",'"order": "/order_id"','"region": "us"','"retries": 0',"/result/order","75 seconds","snapshot-orders"],"snapshot-refunds"],
   [second,["https://example.com/refunds/invoke","X-API-Key header","REFUND_TOKEN","revision 9",'"refund": "/refund_id"','"region": "eu"','"sandbox": true',"/result/refund","120 seconds","snapshot-refunds"],"snapshot-orders"],
  ] as const){
   await select(view.node,1,snapshot.snapshot_id);
   const summary=view.node.querySelector('[aria-label="Selected connector contract"]');expect(summary).not.toBeNull();
   for(const value of values)expect(summary!.textContent).toContain(value);
   expect(summary!.textContent).not.toContain(absent);
   expect(view.node.querySelector<HTMLInputElement>('input[type="checkbox"]')!.checked).toBe(false);
   // Unsaved form defaults remain independent and cannot masquerade as the review.
   expect(view.node.querySelector<HTMLInputElement>('input[type="url"]')!.value).toBe("");
   await act(async()=>view.node.querySelector<HTMLInputElement>('input[type="checkbox"]')!.click());
  }
  expect(client.approve).not.toHaveBeenCalled();expect(client.run).not.toHaveBeenCalled();
 }finally{await view.close()}
});
