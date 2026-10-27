import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { expect, it, vi } from "vitest";
import { ExportDialog } from "./export-dialog";
import { newExportJob, type ExportPreparation } from "@/lib/vibe-export";
import type { useVibeExport } from "@/lib/use-vibe-export";

it("keeps mixed unresolved counts and requires a separate explicit partial download", async () => {
  const job = newExportJob({
    format: "agentclash-evaluation-v2",
    session_id: "project",
    title: "Frozen project",
    captured_at: "2026-10-26T00:00:00Z",
    artifact_id: "version-one",
    selection: { scope: "selected", artifact_id: "version-one", result_id: "run-one" },
    artifacts: [],
    rules: [],
    case_mappings: [],
    omissions: [],
    runs: [
      {
        id: "run-one",
        result_scope: { artifact_id: "version-one", case_keys: ["mixed"] },
        scorecard: { total: 1, passed: 0, failed: 1, unknown: 0 },
        results: [
          {
            case_key: "mixed",
            verdict: "FAIL",
            checks: [{ verdict: "FAIL" }, { verdict: "UNKNOWN" }],
          },
        ],
      },
    ],
    items: [
      {
        kind: "case",
        id: "run-one:mixed",
        case_key: "mixed",
        artifact_id: "version-one",
        hash: "frozen",
      },
    ],
  } as ExportPreparation);
  const controller = {
    state: { owner: {}, open: true, busy: false, error: "", job },
    retry: vi.fn(),
    download: vi.fn(),
    close: vi.fn(),
    start: vi.fn(),
    original: vi.fn(),
  } as unknown as ReturnType<typeof useVibeExport>;
  const node = document.createElement("div");
  document.body.append(node);
  const root = createRoot(node);
  const button = (label: string) =>
    Array.from(document.querySelectorAll("button")).find((b) => b.textContent === label);
  try {
    await act(async () => root.render(<ExportDialog controller={controller} />));
    expect(document.body.textContent).toContain(
      "1 selected cases; 1 failed, 1 unresolved saved cases",
    );
    expect(document.body.textContent).toContain("case:run-one:mixed: Not loaded yet");
    expect(button("Download exact export")).toBeUndefined();
    expect(controller.download).not.toHaveBeenCalled();
    await act(async () => button("Retry missing evidence")!.click());
    expect(controller.retry).toHaveBeenCalledOnce();
    expect(controller.download).not.toHaveBeenCalled();
    await act(async () => button("Confirm and download partial export")!.click());
    expect(controller.download).toHaveBeenCalledExactlyOnceWith(true);
  } finally {
    await act(async () => root.unmount());
    node.remove();
  }
});

it("keeps the SDK result when reconciling or expanding the frozen export",async()=>{
 const resultRef={kind:"sdk_report" as const,id:"a1000000-0000-4000-8000-000000000001"};
 const artifact={id:"tests"};const startResult=vi.fn();
 const job=newExportJob({format:"agentclash-evaluation-v2",session_id:"project",title:"SDK export",captured_at:"2026-10-26T00:00:00Z",artifact_id:"tests",selection:{scope:"selected",artifact_id:"tests",result_ref:resultRef},artifacts:[],rules:[],runs:[],items:[],case_mappings:[],omissions:[]} as ExportPreparation);
 const controller={state:{owner:{},open:true,busy:false,error:"",job,intent:{artifact,resultRef}},close:vi.fn(),download:vi.fn(),start:vi.fn(),startResult} as unknown as ReturnType<typeof useVibeExport>;
 const node=document.createElement("div");document.body.append(node);const root=createRoot(node);
 const button=(label:string)=>Array.from(document.querySelectorAll("button")).find(b=>b.textContent===label)!;
 try {await act(async()=>root.render(<ExportDialog controller={controller}/>));expect(document.body.textContent).toContain("SDK report a1000000");
 await act(async()=>button("Reconcile and prepare selected evidence again").click());expect(startResult).toHaveBeenLastCalledWith(artifact,resultRef,"selected",undefined);
 await act(async()=>button("Prepare full project history").click());expect(startResult).toHaveBeenLastCalledWith(artifact,resultRef,"history",undefined);
 }finally{await act(async()=>root.unmount());node.remove()}
});
