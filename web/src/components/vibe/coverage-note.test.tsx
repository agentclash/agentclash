import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { expect, it, vi } from "vitest";
import { CoverageNote } from "./coverage-note";

it("keeps coverage collapsed and offers only one gap without dispatching work", async () => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  const node=document.createElement("div"); document.body.append(node);
  const root=createRoot(node); const suggest=vi.fn();
  const rows=[{rule_id:"r1",statement:"Preserve headings",case_keys:["one"]},{rule_id:"r2",statement:"Preserve tables",case_keys:[]},{rule_id:"r3",statement:"Keep links",case_keys:[]}];
  try {
    await act(async()=>root.render(<CoverageNote rows={rows} busy={false} onSuggest={suggest} />));
    expect(node.querySelector("details")?.open).toBe(false);
    expect(node.querySelectorAll("button")).toHaveLength(1);
    expect(suggest).not.toHaveBeenCalled();
    await act(async()=>node.querySelector("button")!.click());
    expect(suggest).toHaveBeenCalledExactlyOnceWith("Preserve tables");
    await act(async()=>root.render(<CoverageNote rows={rows} busy onSuggest={suggest} />));
    expect(node.querySelector("button")?.disabled).toBe(true);
  } finally { await act(async()=>root.unmount()); node.remove(); vi.unstubAllGlobals(); }
});
