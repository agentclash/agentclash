import { draftMemory } from "./vibe-build-drafts";
import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { defaultModels, type Session } from "./vibe";
import { useVibeDrafts } from "./use-vibe-drafts";
import { useVibeNavigation } from "./use-vibe-navigation";
import { useVibeSession } from "./use-vibe-session";

const network = vi.hoisted(() => ({ get: vi.fn(), watch: vi.fn() }));
vi.mock("./vibe", async original => ({ ...await original<typeof import("./vibe")>(), vibeFetch: network.get, watchVibe: network.watch }));
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
 network.get.mockReset();network.watch.mockReset();
 network.watch.mockImplementation(()=>new Promise(()=>{}));
});
afterEach(async()=>{await act(async()=>root.unmount());element.remove();vi.restoreAllMocks();vi.unstubAllGlobals();});
it("restores both drafts before writing B and preserves them across switching and refresh",async()=>{
 sessionStorage.setItem("vibe-build-drafts:B",JSON.stringify({version:1,guide:"B guide",trials:{"B:prototype:new:model":"precious B trial"}}));
 let drafts!:ReturnType<typeof useVibeDrafts>;
 function Harness(){drafts=useVibeDrafts("A");return <span>{drafts.content}</span>}
 await act(async()=>root.render(<Harness/>));
 const trials = Object.fromEntries(Array.from({length:13},(_,i)=>[`A:prototype:thread-${i}`,`A unsent trial ${i}`]));
 await act(async()=>{drafts.setContent("A guide");drafts.setTrialBuffers(trials)});
 await act(async()=>drafts.activate("B"));
 expect(drafts.content).toBe("B guide");expect(drafts.trialBuffers["B:prototype:new:model"]).toBe("precious B trial");
 expect(JSON.parse(sessionStorage.getItem("vibe-build-drafts:B")!).composers["B:prototype:composer:prototype:new:model"]).toBe("precious B trial");
 await act(async()=>drafts.activate("A"));expect(drafts.content).toBe("A guide");expect(drafts.trialBuffers).toEqual(trials);
 await act(async()=>root.unmount());root=createRoot(element);
 function Refresh(){drafts=useVibeDrafts("B");return null}
 await act(async()=>root.render(<Refresh/>));expect(drafts.content).toBe("B guide");expect(drafts.trialBuffers["B:prototype:new:model"]).toBe("precious B trial");
});
it("keeps typed drafts during repeated hydration and unavailable storage",async()=>{
 vi.spyOn(Storage.prototype,"getItem").mockImplementation(()=>{throw Error("blocked")});
 vi.spyOn(Storage.prototype,"setItem").mockImplementation(()=>{throw Error("blocked")});
 let drafts!:ReturnType<typeof useVibeDrafts>;
 function Harness(){drafts=useVibeDrafts("A");return null}
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
 function Harness(){controller=useVibeSession("A",token,false,0);return null}
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
 function Harness(){navigation=useVibeNavigation();return null}
 await act(async()=>root.render(<Harness/>));
 await act(async()=>navigation.update({view:"checks",run:"result-B",artifact:"v2",thread:"thread-B"}));
 expect(new URL(window.location.href).searchParams.get("run")).toBe("result-B");
 await act(async()=>root.unmount());root=createRoot(element);
 await act(async()=>root.render(<Harness/>));expect(navigation.run).toBe("result-B");
 await act(async()=>{window.history.replaceState(null,"","/vibe-evals?session=A&view=try&agent=v1&thread=old");window.dispatchEvent(new PopStateEvent("popstate"))});
 expect(navigation.view).toBe("agent");expect(navigation.utility).toBe("trial");expect(navigation.thread).toBe("old");expect(navigation.run).toBeUndefined();
});

it("rejects same-project late GET, SSE and captured mutation/action ingress from the prior actor", async () => {
  const oldGET = deferred<Session>();
  let actor = "first";
  network.get.mockImplementation(() => actor === "first" ? oldGET.promise : Promise.resolve({ ...snapshot(), title: "Second actor" }));
  let controller!: ReturnType<typeof useVibeSession>;
  let receive!: (value: Session) => void;
  network.watch.mockImplementation((_id, _token, _signal, callback) => { receive = callback; return new Promise(() => {}); });
  function Harness() { controller = useVibeSession("A", token, false, 0, actor); return null; }
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
  function Harness() { nav = useVibeNavigation(); return null; }
  await act(async () => root.render(<Harness />));
  expect(nav.view).toBe("agent"); expect(nav.utility).toBe("trial");
  expect(new URLSearchParams(location.search).get("view")).toBe("agent");
  const push = vi.spyOn(history, "pushState"); const replace = vi.spyOn(history, "replaceState");
  await act(async () => nav.update({ view: "tests", case: "case-one" }));
  expect(push).toHaveBeenCalledTimes(1); expect(nav.case).toBe("case-one");
  await act(async () => nav.update({ case: undefined }, "replace"));
  expect(replace).toHaveBeenCalledTimes(1);
  await act(async () => { history.back(); await new Promise(resolve => setTimeout(resolve, 30)); });
  expect(nav.view).toBe("agent"); expect(nav.utility).toBe("trial"); expect(nav.thread).toBe("t");
  await act(async () => { history.forward(); await new Promise(resolve => setTimeout(resolve, 30)); });
  expect(nav.view).toBe("tests");
});
