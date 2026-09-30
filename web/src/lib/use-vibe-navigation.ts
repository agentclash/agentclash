"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useSearchParams } from "next/navigation";

export type VibePane = "build" | "try" | "checks";
type Selection = { session?: string; view: VibePane; artifact: string | null; thread: string; run?: string };
function read(params: URLSearchParams): Selection {
  const view = params.get("view");
  return { session: params.get("session") || undefined, view: view === "try" || view === "checks" ? view : "build", artifact: params.get("agent"), thread: params.get("thread") || "", run: params.get("run") || undefined };
}
export function useVibeNavigation() {
  const params = useSearchParams();
  const query = params.toString();
  const [selection, setSelection] = useState(() => read(new URLSearchParams(query)));
  const current = useRef(selection);
  const remembered = useRef(new Map<string, Selection>());
  const writeURL = useCallback((url: URL) => {
    const next = read(url.searchParams);
    current.current = next; setSelection(next);
    window.history.replaceState(window.history.state, "", url.pathname+url.search);
  }, []);
  const update = useCallback((patch: Partial<Selection>) => {
    const next = { ...current.current, ...patch };
    const url = new URL(window.location.href);
    for (const [key, value] of Object.entries({ session: next.session, view: next.view, agent: next.artifact, thread: next.thread, run: next.run }))
      value ? url.searchParams.set(key, value) : url.searchParams.delete(key);
    writeURL(url);
  }, [writeURL]);
  const selectSession = useCallback((session: string) => {
    if (current.current.session) remembered.current.set(current.current.session, current.current);
    const next = remembered.current.get(session) || { session, view: "build" as const, artifact: null, thread: "", run: undefined };
    update(next);
    return next;
  }, [update]);
  useEffect(() => {
    const next = read(new URLSearchParams(query));
    current.current = next; setSelection(next);
  }, [query]);
  useEffect(() => {
    const restore = () => { const next = read(new URL(window.location.href).searchParams); current.current = next; setSelection(next); };
    window.addEventListener("popstate", restore);
    return () => window.removeEventListener("popstate", restore);
  }, []);
  return { ...selection, update, writeURL, selectSession, current };
}
