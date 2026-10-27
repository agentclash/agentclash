import { draftMemory } from "./vibe-build-drafts";
import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { defaultModels, type Session } from "./vibe";
import { useVibeDrafts } from "./use-vibe-drafts";
import { useVibeNavigation } from "./use-vibe-navigation";
import { useVibeSession } from "./use-vibe-session";

const network = vi.hoisted(() => ({ get: vi.fn(), bytes: vi.fn(), watch: vi.fn() }));
vi.mock("./vibe", async original => ({ ...await original<typeof import("./vibe")>(), vibeFetch: network.get, vibeBytes:network.bytes, watchVibe: network.watch }));
vi.mock("next/navigation", () => ({ useSearchParams: () => new URLSearchParams(window.location.search) }));
let root: Root;
let element: HTMLDivElement;
const token = async () => undefined;
const snapshot = (id="A",revision=1,event_cursor=1): Session => ({ id, revision,event_cursor,anonymous:true,document:{messages:[],requirements:[],artifacts:[],models:defaultModels},operations:[] });
function deferred<T>() { let resolve!: (v:T)=>void; const promise=new Promise<T>(r=>resolve=r);return {promise,resolve}; }
beforeEach(() => {
 vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT",true);
 draftMemory.clear();sessionStorage.clear();window.history.replaceState(null,"","/vibe-evals?session=A");
 element=document.createElement("div");document.body.append(element);root=createRoot(element);
 network.get.mockReset();network.bytes.mockReset();network.watch.mockReset();
 network.watch.mockImplementation(()=>new Promise(()=>{}));
});
afterEach(async()=>{await act(async()=>root.unmount());element.remove();vi.restoreAllMocks();vi.unstubAllGlobals();});
it("restores both drafts before writing B and preserves them across switching and refresh",async()=>{
 sessionStorage.setItem("vibe-build-drafts:B",JSON.stringify({version:1,guide:"B guide",trials:{"B:prototype:new:model":"precious B trial"}}));
 let drafts!:ReturnType<typeof useVibeDrafts>;
 function Harness(){const value=useVibeDrafts("A");React.useEffect(()=>{drafts=value});return <span>{value.content}</span>}
 await act(async()=>root.render(<Harness/>));
 const trials = Object.fromEntries(Array.from({length:13},(_,i)=>[`A:prototype:thread-${i}`,`A unsent trial ${i}`]));
 await act(async()=>{drafts.setContent("A guide");drafts.setTrialBuffers(trials)});
 await act(async()=>drafts.activate("B"));
 expect(drafts.content).toBe("B guide");expect(drafts.trialBuffers["B:prototype:new:model"]).toBe("precious B trial");
 expect(JSON.parse(sessionStorage.getItem("vibe-build-drafts:B")!).composers["B:prototype:composer:prototype:new:model"]).toBe("precious B trial");
 await act(async()=>drafts.activate("A"));expect(drafts.content).toBe("A guide");expect(drafts.trialBuffers).toEqual(trials);
 await act(async()=>root.unmount());root=createRoot(element);
 function Refresh(){const value=useVibeDrafts("B");React.useEffect(()=>{drafts=value});return null}
 await act(async()=>root.render(<Refresh/>));expect(drafts.content).toBe("B guide");expect(drafts.trialBuffers["B:prototype:new:model"]).toBe("precious B trial");
});
it("keeps typed drafts during repeated hydration and unavailable storage",async()=>{
 vi.spyOn(Storage.prototype,"getItem").mockImplementation(()=>{throw Error("blocked")});
 vi.spyOn(Storage.prototype,"setItem").mockImplementation(()=>{throw Error("blocked")});
 let drafts!:ReturnType<typeof useVibeDrafts>;
 function Harness(){const value=useVibeDrafts("A");React.useEffect(()=>{drafts=value});return null}
 await act(async()=>root.render(<Harness/>));
 await act(async()=>drafts.setContent("typing while HTTP loads"));
 await act(async()=>drafts.activate("A"));expect(drafts.content).toBe("typing while HTTP loads");
 await act(async()=>drafts.activate("B"));await act(async()=>drafts.activate("A"));expect(drafts.content).toBe("typing while HTTP loads");
 await act(async()=>root.unmount());root=createRoot(element);await act(async()=>root.render(<Harness/>));
 expect(drafts.content).toBe("typing while HTTP loads"); expect(drafts.storageAvailable).toBe(false);
});
it("rejects delayed GET and mutation/action snapshots after newer stream progress",async()=>{
 const first=deferred<Session>();network.get.mockReturnValueOnce(first.promise);
 let controller!:ReturnType<typeof useVibeSession>;let stream!:(s:Session)=>void;
 network.watch.mockImplementation((_id,_token,_signal,receive)=>{stream=receive;return new Promise(()=>{})});
 function Harness(){const value=useVibeSession("A",token,false,0);React.useEffect(()=>{controller=value});return null}
 await act(async()=>root.render(<Harness/>));
 await act(async()=>controller.accept(snapshot("A",3,9)));
 await act(async()=>stream(snapshot("A",4,10)));
 await act(async()=>first.resolve(snapshot("A",1,1)));
 expect(controller.session?.revision).toBe(4);
 await act(async()=>{controller.accept(snapshot("A",2,11));controller.accept(snapshot("A",5,8));controller.accept(snapshot("B",10,10))});
 expect(controller.session?.revision).toBe(4);
 await act(async()=>controller.select("B"));
 await act(async()=>stream(snapshot("A",20,20)));expect(controller.session).toBeNull();
});
it("keeps result B in the URL on refresh and restores pane/thread on browser navigation",async()=>{
 let navigation!:ReturnType<typeof useVibeNavigation>;
 function Harness(){const value=useVibeNavigation();React.useEffect(()=>{navigation=value});return null}
 await act(async()=>root.render(<Harness/>));
 await act(async()=>navigation.update({view:"checks",run:"result-B",artifact:"v2",thread:"thread-B"}));
 expect(new URL(window.location.href).searchParams.get("run")).toBe("result-B");
 await act(async()=>root.unmount());root=createRoot(element);
 await act(async()=>root.render(<Harness/>));expect(navigation.run).toBe("result-B");
 await act(async()=>navigation.update({thread:""}));
 expect(new URL(window.location.href).searchParams.has("thread")).toBe(true);
 await act(async()=>navigation.writeURL(new URL(window.location.href)));
 expect(navigation.thread).toBe("");
 await act(async()=>{window.history.replaceState(null,"","/vibe-evals?session=A&view=try&agent=v1&thread=old");window.dispatchEvent(new PopStateEvent("popstate"))});
 expect(navigation.view).toBe("agent");expect(navigation.section).toBe("trial");expect(navigation.thread).toBe("old");expect(navigation.run).toBeUndefined();
});

it("rejects same-project late GET, SSE and captured mutation/action ingress from the prior actor", async () => {
  const oldGET = deferred<Session>();
  let actor = "first";
  network.get.mockImplementation(() => actor === "first" ? oldGET.promise : Promise.resolve({ ...snapshot(), title: "Second actor" }));
  let controller!: ReturnType<typeof useVibeSession>;
  let receive!: (value: Session) => void;
  network.watch.mockImplementation((_id, _token, _signal, callback) => { receive = callback; return new Promise(() => {}); });
  function Harness() { const value = useVibeSession("A", token, false, 0, actor); React.useEffect(() => { controller = value; }); return null; }
  await act(async () => root.render(<Harness />));
  await act(async () => controller.accept({ ...snapshot(), title: "First actor" }));
  const oldSSE = receive; const oldMutation = controller.accept;
  actor = "second"; await act(async () => root.render(<Harness />));
  expect(controller.session?.title).toBe("Second actor");
  const foreign = { ...snapshot("A", 99, 99), title: "First actor private history" };
  await act(async () => { oldGET.resolve(foreign); oldSSE(foreign); expect(oldMutation(foreign)).toBe(false); });
  expect(controller.session?.title).toBe("Second actor"); expect(controller.session?.revision).toBe(1);
});

it("canonicalizes legacy trial links and pushes deliberate navigation while corrections replace", async () => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  history.replaceState({}, "", "/vibe-evals?session=p&view=try&agent=a&thread=t");
  let nav!: ReturnType<typeof useVibeNavigation>;
  function Harness() { const value = useVibeNavigation(); React.useEffect(() => { nav = value; }); return null; }
  await act(async () => root.render(<Harness />));
  expect(nav.view).toBe("agent"); expect(nav.section).toBe("trial");
  expect(new URLSearchParams(location.search).get("view")).toBe("agent");
  const push = vi.spyOn(history, "pushState"); const replace = vi.spyOn(history, "replaceState");
  await act(async () => nav.update({ view: "tests", case: "case-one" }));
  expect(push).toHaveBeenCalledTimes(1); expect(nav.case).toBe("case-one");
  await act(async () => nav.update({ case: undefined }, "replace"));
  expect(replace).toHaveBeenCalledTimes(1);
  await act(async () => { history.back(); await new Promise(resolve => setTimeout(resolve, 30)); });
  expect(nav.view).toBe("agent"); expect(nav.section).toBe("trial"); expect(nav.thread).toBe("t");
  await act(async () => { history.forward(); await new Promise(resolve => setTimeout(resolve, 30)); });
  expect(nav.view).toBe("tests");
});

it("invalidates captured final downloads after closing an export intent, even for the same actor", async () => {
  const {useVibeExport}=await import("./use-vibe-export");
  const artifact={id:"version-a",kind:"test_suite",title:"Version A"} as Session["document"]["artifacts"][number];
  const preparation={format:"agentclash-evaluation-v2",session_id:"A",title:"Original title",captured_at:"2026-10-26T12:00:00Z",artifact_id:artifact.id,selection:{scope:"selected",artifact_id:artifact.id},artifacts:[artifact],rules:[],runs:[],items:[],case_mappings:[],omissions:[]};
  network.get.mockResolvedValue(preparation);
  vi.stubGlobal("URL",Object.assign(URL,{createObjectURL:vi.fn(()=>"blob:test"),revokeObjectURL:vi.fn()}));
  const click=vi.spyOn(HTMLAnchorElement.prototype,"click").mockImplementation(()=>{});
  let c!:ReturnType<typeof useVibeExport>;
  function Harness(){const value=useVibeExport("actor",snapshot(),token);React.useEffect(()=>{c=value});return null}
  await act(async()=>root.render(<Harness/>));await act(async()=>c.start(artifact));
  const obsoleteDownload=c.download;
  await act(async()=>c.close());
  obsoleteDownload();expect(click).not.toHaveBeenCalled();
  await act(async()=>c.start(artifact));obsoleteDownload();expect(click).not.toHaveBeenCalled();
  c.download();expect(click).toHaveBeenCalledTimes(1);
});

it("rejects a late private export body after actor A to B to A and pins the originally selected identity", async () => {
  const {useVibeExport}=await import("./use-vibe-export");
  const artifact={id:"version-a",kind:"test_suite",title:"Version A"} as Session["document"]["artifacts"][number];
  const operation={id:"run-a",results:[],state:"COMPLETED"} as Session["operations"][number];
  const body=deferred<unknown>();let actor="A";
  const preparation={format:"agentclash-evaluation-v2",session_id:"A",title:"Original title",captured_at:"2026-10-26T12:00:00Z",artifact_id:artifact.id,selection:{scope:"selected",artifact_id:artifact.id,result_id:operation.id},artifacts:[artifact],rules:[],runs:[operation],items:[{kind:"case",id:"run-a:case",operation_id:operation.id,artifact_id:artifact.id,case_key:"case",hash:"frozen"}],case_mappings:[],omissions:[]};
  network.get.mockImplementation((path:string)=>path.endsWith("/export")?Promise.resolve(preparation):body.promise);
  let c!:ReturnType<typeof useVibeExport>;
  function Harness(){const value=useVibeExport(actor,snapshot(),token);React.useEffect(()=>{c=value});return null}
  await act(async()=>root.render(<Harness/>));let pending!:Promise<void>;
  await act(async()=>{pending=c.start(artifact,operation);await Promise.resolve()});
  expect(JSON.parse(network.get.mock.calls[0][2].body)).toEqual({scope:"selected",artifact_id:"version-a",result_id:"run-a"});
  const obsoleteDownload=c.download;
  actor="B";await act(async()=>root.render(<Harness/>));actor="A";await act(async()=>root.render(<Harness/>));
  await act(async()=>{body.resolve({case_key:"case",version:"version-a",evidence_hash:"frozen",output:"private"});await pending});
  expect(c.state).toBeUndefined();
  const click=vi.spyOn(HTMLAnchorElement.prototype,"click").mockImplementation(()=>{});obsoleteDownload();expect(click).not.toHaveBeenCalled();
});

it.each(["actor change", "unmount"])("does not download retained original bytes after %s during the fetch",async(lifetime)=>{
 const {useVibeExport}=await import('./use-vibe-export');let actor='A';let c!:ReturnType<typeof useVibeExport>;
 const body=deferred<Uint8Array>();network.bytes.mockReturnValue(body.promise);
 const artifact={id:'version',original:{filename:'exact.json',media_type:'application/json'}} as Session['document']['artifacts'][number];
 function Harness(){const value=useVibeExport(actor,snapshot(),token);React.useEffect(()=>{c=value});return null}
 await act(async()=>root.render(<Harness/>));let pending!:Promise<void>;await act(async()=>{pending=c.original(artifact);await Promise.resolve()});
 if(lifetime==='unmount')await act(async()=>root.render(null));else {actor='B';await act(async()=>root.render(<Harness/>));}
 vi.stubGlobal('URL',Object.assign(URL,{createObjectURL:vi.fn(()=>'blob:original'),revokeObjectURL:vi.fn()}));
 const click=vi.spyOn(HTMLAnchorElement.prototype,'click').mockImplementation(()=>{});
 await act(async()=>{body.resolve(new Uint8Array([0xef,0xbb,0xbf,123,125]));await pending});expect(click).not.toHaveBeenCalled();expect(c.state).toBeUndefined();
});

it("invalidates captured explicit export downloads when the owner unmounts",async()=>{
 const {useVibeExport}=await import("./use-vibe-export");
 const artifact={id:"version",kind:"test_suite"} as Session["document"]["artifacts"][number];
 network.get.mockResolvedValue({format:"agentclash-evaluation-v2",session_id:"A",title:"Frozen",captured_at:"now",artifact_id:artifact.id,selection:{scope:"selected",artifact_id:artifact.id},artifacts:[artifact],rules:[],runs:[],items:[],case_mappings:[],omissions:[]});
 vi.stubGlobal("URL",Object.assign(URL,{createObjectURL:vi.fn(()=>"blob:export"),revokeObjectURL:vi.fn()}));
 const click=vi.spyOn(HTMLAnchorElement.prototype,"click").mockImplementation(()=>{});
 let c!:ReturnType<typeof useVibeExport>;
 function Harness(){const value=useVibeExport("actor",snapshot(),token);React.useEffect(()=>{c=value});return null}
 await act(async()=>root.render(<React.StrictMode><Harness/></React.StrictMode>));
 await act(async()=>c.start(artifact));
 c.download();expect(click).toHaveBeenCalledOnce();click.mockClear();
 const captured=c.download;
 await act(async()=>root.render(null));
 captured();expect(click).not.toHaveBeenCalled();
});

it.each(["sdk_report","run","eval_session"] as const)("freezes a %s return selection without inventing a save baseline",async(kind)=>{
 const {useVibeSaving}=await import("./use-vibe-saving");
 const ref={kind,id:"a1000000-0000-4000-8000-000000000001"};
 const artifact={id:"a1000000-0000-4000-8000-000000000002",kind:"test_suite" as const,title:"Pinned tests",agent_prompt:"",accepted:false,source_message_id:"",blueprint:{cases:[]}};
 const project={...snapshot(),document:{...snapshot().document,artifacts:[artifact]}};
 history.replaceState({},"",`/vibe-evals?session=A&agent=${artifact.id}&view=results&run=${kind}:${ref.id}&case=local&thread=retained`);
 let saving!:ReturnType<typeof useVibeSaving>;
 const noop=vi.fn();
 function Harness(){const value=useVibeSaving({session:project,artifact,artifacts:[artifact],selectedResult:{session_id:"A",artifact_id:artifact.id,result_ref:ref},content:"",models:defaultModels,workspace:"",savedChecks:[],token,account:{status:"signed_out",workspaces:[],access:{status:"signed_out",canRead:false,canWrite:false,canAdmin:false},retry:noop},setContent:noop,setModels:noop,setWorkspace:noop,setError:noop,setPending:noop,setSavedChecks:noop,setSelectedArtifactID:noop,reload:async()=>project,writeURL:noop});React.useEffect(()=>{saving=value});return null}
 await act(async()=>root.render(<Harness/>));await act(async()=>saving.openSave());
 const captured=saving.saveReturnPath!;const url=new URL(captured,"https://local.invalid");
 expect(url.searchParams.get("run")).toBe(`${kind}:${ref.id}`);expect(url.searchParams.get("agent")).toBe(artifact.id);
 expect(url.searchParams.get("case")).toBe("local");expect(url.searchParams.get("thread")).toBe("retained");expect(saving.saveBaseline).toBeUndefined();
 history.replaceState({},"","/vibe-evals?session=A&view=agent");await act(async()=>root.render(<Harness/>));expect(saving.saveReturnPath).toBe(captured);
});
