import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { expect, it, vi } from "vitest";
import { EvaluationNavigation } from "./evaluation-navigation";
import type { Session } from "@/lib/vibe";
vi.mock("framer-motion", () => ({ useReducedMotion: () => true, motion: { aside: ({children}: React.PropsWithChildren) => <aside>{children}</aside>, span: ({children}: React.PropsWithChildren) => <span>{children}</span> } }));
vi.mock("@/components/ui/sheet", () => ({ Sheet: () => null, SheetContent: () => null, SheetDescription: () => null, SheetTitle: () => null, SheetTrigger: () => null }));
it("discards obsolete metadata failures and resets pending controls on project switch", async () => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  const node = document.createElement("div"), root = createRoot(node);
  document.body.append(node);
  const session = { id: "first", document: { artifacts: [] }, operations: [] } as unknown as Session;
  let reject!: (e: Error) => void;
  const metadata = () => new Promise<boolean>((_, no) => { reject = no; });
  const render = (selected: Session) => root.render(<EvaluationNavigation enabled contexts={[selected]} session={selected} choosing={false} disabled={false} onMetadata={metadata} onNew={()=>{}} onSwitch={()=>{}} onSettings={()=>{}} onSavedWork={()=>{}}>{()=>null}</EvaluationNavigation>);
  try {
    await act(async () => render(session));
    const archive = () => Array.from(node.querySelectorAll("button")).find(b => b.textContent === "Archive project")!;
    await act(async () => archive().click());
    expect(archive().disabled).toBe(true);
    await act(async () => render({ ...session, id: "second" }));
    expect(archive().disabled).toBe(false);
    await act(async () => reject(Error("Old project denied")));
    expect(node.textContent).not.toContain("Old project denied");
  } finally { await act(async () => root.unmount()); node.remove(); vi.unstubAllGlobals(); }
});
