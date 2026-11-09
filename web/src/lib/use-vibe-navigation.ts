"use client";

import { useCallback, useEffect, useMemo, useRef, useSyncExternalStore } from "react";
import { useSearchParams } from "next/navigation";

export type VibePane = "agent" | "tests" | "results";
export type LegacyVibePane = "build" | "try" | "checks" | "tests";
// A missing thread selects retained history; an empty thread explicitly starts a new conversation.
type Selection = { session?: string; view: VibePane; section?: "trial"; artifact: string | null; thread: string | null; run?: string; case?: string };
type Patch = Omit<Partial<Selection>, "view"> & { view?: VibePane | LegacyVibePane };
const navigationEvent = "vibe-navigation";
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
function subscribe(listener: () => void) {
  window.addEventListener("popstate", listener);
  window.addEventListener(navigationEvent, listener);
  return () => {
    window.removeEventListener("popstate", listener);
    window.removeEventListener(navigationEvent, listener);
  };
}
function notify() { window.dispatchEvent(new Event(navigationEvent)); }
function browserQuery() { return window.location.search; }
export function useVibeNavigation() {
  const params = useSearchParams();
  const query = params.toString();
  // The browser URL owns selection; a stale router snapshot cannot overwrite it.
  const search = useSyncExternalStore(subscribe, browserQuery, () => query);
  const selection = useMemo(() => read(new URLSearchParams(search)), [search]);
  const remembered = useRef(new Map<string, Selection>());
  const writeURL = useCallback((url: URL, mode: "push" | "replace" = "replace") => {
    serialize(url, read(url.searchParams));
    const target = url.pathname + url.search;
    if (target !== window.location.pathname + window.location.search)
      window.history[mode === "push" ? "pushState" : "replaceState"](window.history.state, "", target);
    notify();
  }, []);
  const update = useCallback((patch: Patch, mode: "push" | "replace" = "push") => {
    const url = new URL(window.location.href);
    const current = read(url.searchParams);
    const view = patch.view;
    const next: Selection = { ...current, ...patch, view: view === "build" || view === "try" ? "agent" : view === "checks" ? "results" : view || current.view,
      ...(view === "try" ? { section: "trial" as const } : Object.hasOwn(patch, "section") ? { section: patch.section } : view === "build" || view === "agent" ? { section: undefined } : {}) };
    writeURL(serialize(url, next), mode);
  }, [writeURL]);
  const selectSession = useCallback((session: string) => {
    const current = read(new URL(window.location.href).searchParams);
    if (current.session) remembered.current.set(current.session, current);
    const next = remembered.current.get(session) || { session, view: "agent" as const, artifact: null, thread: null, run: undefined, case: undefined, section: undefined };
    update(next);
    return next;
  }, [update]);
  // Canonicalize legacy links and observe Next router changes using the live URL.
  useEffect(() => { writeURL(new URL(window.location.href)); }, [query, writeURL]);
  const legacyView: LegacyVibePane = selection.section === "trial" && selection.view === "agent" ? "try" : selection.view === "results" ? "checks" : selection.view === "tests" ? "tests" : "build";
  return { ...selection, search: search ? `?${new URLSearchParams(search)}` : "", start: new URLSearchParams(search).get("start"), legacyView, update, writeURL, selectSession };
}
