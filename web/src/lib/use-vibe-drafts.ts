"use client";

import { useCallback, useRef, useState, type SetStateAction } from "react";
import { readBuildDrafts, writeBuildDrafts, discardDrafts, draftMemory, emptyDrafts, type BuildDrafts } from "./vibe-build-drafts";

const empty = emptyDrafts;
// One atomic row owns both composers. Hydration cannot persist half a row or
// replace edits made while HTTP is loading. Storage is only an optional mirror.
export function useVibeDrafts(initialID?: string) {
  const [draft, setDraft] = useState(() => ({ id: initialID, value: initialID ? draftMemory.get(initialID) || readBuildDrafts(initialID) || empty() : empty() }));
  const current = useRef(draft);
  const [storageAvailable, setStorageAvailable] = useState(() => {
    try { if (typeof sessionStorage !== "undefined") sessionStorage.getItem("vibe-build-drafts:" + (initialID || "")); return true; }
    catch { return false; }
  });
  const composerEdits = useRef(0);
  const trialEdits = useRef<Record<string, number>>({});
  const publish = useCallback((next: { id: string | undefined; value: BuildDrafts }) => {
    current.current = next;
    if (next.id) setStorageAvailable(writeBuildDrafts(next.id, next.value.guide, next.value.trials));
    setDraft(next);
  }, []);
  const activate = useCallback((id?: string) => {
    const previous = current.current;
    if (previous.id === id) return;
    composerEdits.current++;
    if (previous.id) writeBuildDrafts(previous.id, previous.value.guide, previous.value.trials);
    publish({ id, value: id ? draftMemory.get(id) || readBuildDrafts(id) || empty() : empty() });
  }, [publish]);
  const setContent = useCallback((next: SetStateAction<string>) => {
    const previous = current.current;
    publish({ ...previous, value: { ...previous.value, guide: typeof next === "function" ? next(previous.value.guide) : next } });
  }, [publish]);
  const setTrialBuffers = useCallback((next: SetStateAction<Record<string,string>>) => {
    const previous = current.current;
    publish({ ...previous, value: { ...previous.value, trials: typeof next === "function" ? next(previous.value.trials) : next } });
  }, [publish]);
  const discard = useCallback((id: string) => {
    discardDrafts(id);
  }, []);
  return { storageAvailable, content: draft.value.guide, trialBuffers: draft.value.trials, setContent, setTrialBuffers, composerEdits, trialEdits, activate, discard };
}
