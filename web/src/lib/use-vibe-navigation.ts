"use client";

import { useCallback, useEffect, useMemo, useRef, useSyncExternalStore } from "react";
import { useSearchParams } from "next/navigation";

export type VibePane = "build" | "try" | "checks";
type Selection = { session?: string; view: VibePane; artifact: string | null; thread: string; run?: string };
const navigationEvent = "vibe-navigation";
function read(params: URLSearchParams): Selection {
  const view = params.get("view");
  return { session: params.get("session") || undefined, view: view === "try" || view === "checks" ? view : "build", artifact: params.get("agent"), thread: params.get("thread") || "", run: params.get("run") || undefined };
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
  // The URL is the selection owner. Subscribe to it instead of mirroring it
  // into component state and reconciling another copy after every navigation.
  const search = useSyncExternalStore(subscribe, browserQuery, () => query);
  const selection = useMemo(() => read(new URLSearchParams(search)), [search]);
  const remembered = useRef(new Map<string, Selection>());
  const writeURL = useCallback((url: URL) => {
    window.history.replaceState(window.history.state, "", url.pathname+url.search);
    notify();
  }, []);
  const update = useCallback((patch: Partial<Selection>) => {
    const url = new URL(window.location.href);
    const next = { ...read(url.searchParams), ...patch };
    for (const [key, value] of Object.entries({ session: next.session, view: next.view, agent: next.artifact, thread: next.thread, run: next.run })) {
      if (value) url.searchParams.set(key, value);
      else url.searchParams.delete(key);
    }
    writeURL(url);
  }, [writeURL]);
  const selectSession = useCallback((session: string) => {
    const current = read(new URL(window.location.href).searchParams);
    if (current.session) remembered.current.set(current.session, current);
    const next = remembered.current.get(session) || { session, view: "build" as const, artifact: null, thread: "", run: undefined };
    update(next);
    return next;
  }, [update]);
  // Next's router can change the URL without a native popstate event.
  useEffect(notify, [query]);
  return { ...selection, update, writeURL, selectSession };
}
