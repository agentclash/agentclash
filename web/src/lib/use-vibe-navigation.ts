"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useSearchParams } from "next/navigation";

export type VibePane = "agent" | "tests" | "results";
export type LegacyVibePane = "build" | "try" | "checks" | "tests";
// A missing thread selects retained history; an empty thread explicitly starts a new conversation.
type Selection = { session?: string; view: VibePane; section?: "trial"; artifact: string | null; thread: string | null; run?: string; case?: string };
type Patch = Omit<Partial<Selection>, "view"> & { view?: VibePane | LegacyVibePane };
function read(params: URLSearchParams): Selection {
  const view = params.get("view");
  return { session: params.get("session") || undefined, view: view === "checks" || view === "results" ? "results" : view === "tests" ? "tests" : "agent", section: view === "try" || (params.get("section") === "trial" || params.get("utility") === "trial") ? "trial" : undefined, artifact: params.get("agent"), thread: params.get("thread"), run: params.get("run") || undefined, case: params.get("case") || undefined };
}
function serialize(url: URL, next: Selection) {
  url.searchParams.delete("utility");
  for (const [key, value] of Object.entries({ session: next.session, view: next.view, agent: next.artifact, thread: next.thread, run: next.run, case: next.case, section: next.section }))
    (value || key === "thread" && value === "") ? url.searchParams.set(key, value!) : url.searchParams.delete(key);
  return url;
}
export function useVibeNavigation() {
  const params = useSearchParams();
  const query = params.toString();
  const [selection, setSelection] = useState(() => read(new URLSearchParams(query)));
  const current = useRef(selection);
  const remembered = useRef(new Map<string, Selection>());
  const writeURL = useCallback((url: URL, mode: "push" | "replace" = "replace") => {
    const next = read(url.searchParams);
    serialize(url, next);
    current.current = next; setSelection(next);
    const target = url.pathname + url.search;
    if (target !== window.location.pathname + window.location.search)
      window.history[mode === "push" ? "pushState" : "replaceState"](window.history.state, "", target);
  }, []);
  const update = useCallback((patch: Patch, mode: "push" | "replace" = "push") => {
    const view = patch.view;
    const next: Selection = { ...current.current, ...patch, view: view === "build" || view === "try" ? "agent" : view === "checks" ? "results" : view || current.current.view,
      ...(view === "try" ? { section: "trial" as const } : Object.hasOwn(patch, "section") ? { section: patch.section } : view === "build" || view === "agent" ? { section: undefined } : {}) };
    writeURL(serialize(new URL(window.location.href), next), mode);
  }, [writeURL]);
  const selectSession = useCallback((session: string) => {
    if (current.current.session) remembered.current.set(current.current.session, current.current);
    const next = remembered.current.get(session) || { session, view: "agent" as const, artifact: null, thread: null, run: undefined, case: undefined, section: undefined };
    update(next);
    return next;
  }, [update]);
  useEffect(() => {
    const url = new URL(window.location.href); url.search = query; writeURL(url);
  }, [query, writeURL]);
  useEffect(() => {
    const restore = () => writeURL(new URL(window.location.href));
    window.addEventListener("popstate", restore);
    return () => window.removeEventListener("popstate", restore);
  }, [writeURL]);
  // The old renderer is a projection, never a second writable pane store.
  const legacyView: LegacyVibePane = selection.section === "trial" && selection.view === "agent" ? "try" : selection.view === "results" ? "checks" : selection.view === "tests" ? "tests" : "build";
  return { ...selection, legacyView, update, writeURL, selectSession, current };
}
