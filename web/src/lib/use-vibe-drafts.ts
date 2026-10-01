"use client";

import { useCallback, useEffect, useRef, useState, type SetStateAction } from "react";
import { readBuildDrafts, writeBuildDrafts, type BuildDrafts } from "./vibe-build-drafts";

const empty = (): BuildDrafts => ({ version: 1, guide: "", trials: {} });
// One atomic row owns both composers. Hydration cannot persist half a row or
// replace edits made while HTTP is loading. Storage is only an optional mirror.
export function useVibeDrafts(initialID?: string) {
  const [draft, setDraft] = useState(() => ({ id: initialID, value: initialID ? readBuildDrafts(initialID) || empty() : empty() }));
  const rows = useRef(new Map<string, BuildDrafts>());
  const active = useRef(initialID);
  const composerEdits = useRef(0);
  const trialEdits = useRef<Record<string, number>>({});
  const activate = useCallback((id?: string) => {
    if (active.current !== id) composerEdits.current++;
    active.current = id;
    setDraft(old => {
      if (old.id === id) return old;
      if (old.id) rows.current.set(old.id, old.value);
      return { id, value: id ? rows.current.get(id) || readBuildDrafts(id) || empty() : empty() };
    });
  }, []);
  const setContent = useCallback((next: SetStateAction<string>) => {
    setDraft(old => ({ ...old, value: { ...old.value, guide: typeof next === "function" ? next(old.value.guide) : next } }));
  }, []);
  const setTrialBuffers = useCallback((next: SetStateAction<Record<string,string>>) => {
    setDraft(old => ({ ...old, value: { ...old.value, trials: typeof next === "function" ? next(old.value.trials) : next } }));
  }, []);
  useEffect(() => {
    if (!draft.id) return;
    rows.current.set(draft.id, draft.value);
    writeBuildDrafts(draft.id, draft.value.guide, draft.value.trials);
  }, [draft]);
  const discard = useCallback((id: string) => {
    rows.current.delete(id);
    try { sessionStorage.removeItem("vibe-build-drafts:"+id); } catch { /* Optional storage. */ }
  }, []);
  return { content: draft.value.guide, trialBuffers: draft.value.trials, setContent, setTrialBuffers, composerEdits, trialEdits, activate, discard };
}
