import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { expect, it, vi } from "vitest";
import { EvaluationNavigation } from "./evaluation-navigation";
import type { Session } from "@/lib/vibe";
import type { CollectionStatus } from "@/lib/use-vibe-collection";
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
    await act(async () => Array.from(node.querySelectorAll("summary")).find(summary => summary.textContent === "Project actions")!.click());
    const archive = () => Array.from(node.querySelectorAll("button")).find(b => b.textContent === "Archive project")!;
    await act(async () => archive().click());
    expect(archive().disabled).toBe(true);
    await act(async () => render({ ...session, id: "second" }));
    expect(archive().disabled).toBe(false);
    await act(async () => reject(Error("Old project denied")));
    expect(node.textContent).not.toContain("Old project denied");
  } finally { await act(async () => root.unmount()); node.remove(); vi.unstubAllGlobals(); }
});

it("keeps guest project tools out of an empty list and preserves real load errors", async () => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  const node = document.createElement("div"), root = createRoot(node);
  document.body.append(node);
  const retry = vi.fn();
  const render = (actorScope?: string, status: CollectionStatus = "forbidden") => root.render(<EvaluationNavigation actorScope={actorScope} enabled contexts={[]} session={null} choosing disabled={false} listStatus={status} onRetryList={retry} onArchiveMode={()=>{}} onNew={()=>{}} onSwitch={()=>{}} onSettings={()=>{}} onSavedWork={()=>{}}>{()=>null}</EvaluationNavigation>);
  try {
    await act(async () => render("guest", "ready"));
    expect(node.textContent).toContain("No projects yet.");
    expect(node.textContent).not.toContain("Retry");
    await act(async () => render("guest", "loading"));
    expect(node.textContent).toContain("Loading projects…");
    expect(node.textContent).not.toContain("Retry");
    await act(async () => render());
    expect(node.querySelector('input[type="search"]')).toBeNull();
    expect(node.querySelector('select')).toBeNull();
    expect(node.textContent).toContain("Couldn’t load your projects.");
    expect(node.textContent).not.toContain("access to this workspace");
    const retryButton = Array.from(node.querySelectorAll("button")).find(button => button.textContent?.includes("Retry"))!;
    await act(async () => retryButton.click());
    expect(retry).toHaveBeenCalledOnce();
    await act(async () => render("user:one"));
    expect(node.textContent).toContain("access to this workspace");
  } finally { await act(async () => root.unmount()); node.remove(); vi.unstubAllGlobals(); }
});

it("keeps list tools reachable after search misses and switching to an empty archive", async () => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  const node = document.createElement("div"), root = createRoot(node);
  document.body.append(node);
  const session = { id: "first", document: { artifacts: [] }, operations: [] } as unknown as Session;
  const render = (contexts: Session[], archiveMode: "exclude" | "only") => root.render(<EvaluationNavigation enabled contexts={contexts} session={null} choosing disabled={false} archiveMode={archiveMode} onArchiveMode={()=>{}} onRetryList={()=>{}} onNew={()=>{}} onSwitch={()=>{}} onSettings={()=>{}} onSavedWork={()=>{}}>{()=>null}</EvaluationNavigation>);
  const projects = Array.from({ length: 7 }, (_, i) => ({ ...session, id: `project-${i}` }));
  try {
    // A short list shows no search, but the archive filter and refresh stay reachable.
    await act(async () => render(projects.slice(0, 6), "exclude"));
    expect(node.querySelector('input[type="search"]')).toBeNull();
    expect(node.querySelector('select[aria-label="Project archive filter"]')).not.toBeNull();
    expect(node.querySelector('button[aria-label="Refresh projects"]')).not.toBeNull();
    expect(node.textContent).not.toContain("The server returns the latest 100 projects");
    await act(async () => render(projects, "exclude"));
    const search = node.querySelector<HTMLInputElement>('input[type="search"]')!;
    await act(async () => {
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!.call(search, "no matching title");
      search.dispatchEvent(new Event("input", { bubbles: true }));
    });
    expect(node.querySelector('[data-evaluation-id]')).toBeNull();
    expect(node.querySelector('input[type="search"]')).toBe(search);
    expect(search.placeholder).toBe("Search projects");
    expect(document.getElementById(search.getAttribute("aria-describedby")!)?.textContent).toContain("latest 100 projects");
    await act(async () => render([], "only"));
    expect(node.querySelector<HTMLSelectElement>('select')?.value).toBe("only");
    expect(node.querySelector('input[type="search"]')).toBe(search);
  } finally { await act(async () => root.unmount()); node.remove(); vi.unstubAllGlobals(); }
});

it("groups project actions and retains rename input and failure recovery", async () => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  const node = document.createElement("div"), root = createRoot(node);
  document.body.append(node);
  const session = { id: "first", document: { artifacts: [] }, operations: [] } as unknown as Session;
  const metadata = vi.fn().mockRejectedValue(Error("Could not save this title"));
  const button = (label: string) => Array.from(node.querySelectorAll("button")).find(button => button.textContent === label)!;
  try {
    await act(async () => root.render(<EvaluationNavigation enabled contexts={[session]} session={session} choosing={false} disabled={false} onMetadata={metadata} onNew={()=>{}} onSwitch={()=>{}} onSettings={()=>{}} onSavedWork={()=>{}}>{()=>null}</EvaluationNavigation>));
    const summary = Array.from(node.querySelectorAll("summary")).find(summary => summary.textContent === "Project actions")!;
    expect(summary).toBeDefined();
    const details = summary.parentElement as HTMLDetailsElement;
    expect(details.open).toBe(false);
    await act(async () => summary.click());
    await act(async () => button("Rename project").click());
    const input = node.querySelector<HTMLInputElement>('[aria-label="Project title"]')!;
    await act(async () => {
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!.call(input, "Renamed project");
      input.dispatchEvent(new Event("input", { bubbles: true }));
    });
    await act(async () => summary.click());
    await act(async () => summary.click());
    expect(input.value).toBe("Renamed project");
    await act(async () => button("Save title").click());
    expect(details.open).toBe(true);
    expect(node.querySelector('[role="alert"]')?.textContent).toBe("Could not save this title");
    expect(input.value).toBe("Renamed project");
    expect(metadata).toHaveBeenCalledWith(session, { title: "Renamed project" });
  } finally { await act(async () => root.unmount()); node.remove(); vi.unstubAllGlobals(); }
});
