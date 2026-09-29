import React, {act, useLayoutEffect} from "react";
import {createRoot, type Root} from "react-dom/client";
import {beforeEach, afterEach, expect, it, vi} from "vitest";
import {useVibeInputs} from "./use-vibe-inputs";
import {uploadMaterial, type TaskMaterial} from "./vibe-inputs";
import {vibeFetch} from "./vibe";

vi.mock("./vibe-inputs",async load=>({...await load<typeof import("./vibe-inputs")>(),uploadMaterial:vi.fn()}));
vi.mock("./vibe",async load=>({...await load<typeof import("./vibe")>(),vibeFetch:vi.fn()}));
const token=async()=>undefined;
const material: TaskMaterial={id:"file-a",session_id:"a",kind:"text",name:"Text",status:"ready",content_hash:"hash-a",pages:[{number:1,text:"Original notes"}],warnings:[],size_bytes:14,created_at:"2026-09-29T00:00:00Z"};
let node:HTMLDivElement, root:Root, state:ReturnType<typeof useVibeInputs>;
function Probe({session,scope}:{session:string;scope:string}){const current=useVibeInputs(session,scope,token);useLayoutEffect(()=>{state=current;},[current]);return null;}
beforeEach(()=>{vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT",true);vi.clearAllMocks();sessionStorage.clear();node=document.createElement("div");document.body.append(node);root=createRoot(node);});
afterEach(async()=>{await act(async()=>root.unmount());node.remove();vi.unstubAllGlobals();});
async function render(session="a",scope="guide"){await act(async()=>root.render(<Probe session={session} scope={scope}/>));}

it("finishes an upload in the original context and deduplicates double clicks",async()=>{
  let resolve!:(value:TaskMaterial)=>void;
  vi.mocked(uploadMaterial).mockReturnValue(new Promise(done=>{resolve=done;}));
  await render();
  await act(async()=>{void state.add("Original notes");void state.add("Original notes");});
  expect(uploadMaterial).toHaveBeenCalledTimes(1);
  await render("b");
  await act(async()=>resolve(material));
  expect(state.items).toHaveLength(0);
  await render("a");expect(state.bindings).toEqual([{input_id:"file-a",content_hash:"hash-a",usage:"task_input"}]);
  await render("a","trial:v2:thread");expect(state.items).toHaveLength(0);
  expect(vibeFetch).not.toHaveBeenCalled(); // No paid request, even across switches.
});

it("uses the same upload identity on retry and requires partial-reading acknowledgement",async()=>{
  vi.mocked(uploadMaterial).mockRejectedValueOnce(new Error("Connection lost")).mockResolvedValueOnce({...material,warnings:["Page 2 contains an unread image"]});
  await render();await act(async()=>state.add("Original notes"));
  const identity=vi.mocked(uploadMaterial).mock.calls[0][1];
  await act(async()=>state.retry());
  expect(vi.mocked(uploadMaterial).mock.calls[1][1]).toBe(identity);
  expect(state.blocked).toBe(true);
  await act(async()=>state.acknowledge("file-a",true));
  expect(state.blocked).toBe(false);expect(state.bindings[0].accept_partial).toBe(true);
});

it("requires explicit exact policy selection and distinguishes detach from deletion",async()=>{
  vi.mocked(uploadMaterial).mockResolvedValue(material);
  await render();await act(async()=>state.add("Original notes"));
  expect(state.adoptions).toEqual([]);
  await act(async()=>state.configure("file-a",{usage:"rules",quote:"invented policy",page:1}));
  expect(state.blocked).toBe(true);expect(state.bindings).toEqual([]);
  await act(async()=>state.configure("file-a",{quote:"Original notes"}));
  expect(state.blocked).toBe(false);
  await act(async()=>state.remove("file-a"));expect(vibeFetch).not.toHaveBeenCalled();
  await act(async()=>state.add("Original notes"));
  vi.mocked(vibeFetch).mockResolvedValue({});
  await act(async()=>state.remove("file-a",true));
  expect(vibeFetch).toHaveBeenCalledWith("/sessions/a/inputs/file-a",undefined,{method:"DELETE"});
});
