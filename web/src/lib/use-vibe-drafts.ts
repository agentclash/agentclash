"use client";

import { useCallback, useMemo, useRef, useState, useSyncExternalStore, type SetStateAction } from "react";
import { useVibeBrowserReady } from "./use-vibe-browser-ready";
import { readBuildDrafts, writeBuildDrafts, discardDrafts, draftMemory, emptyDrafts, type BuildDrafts } from "./vibe-build-drafts";

const empty = emptyDrafts;
const subscribeStorage = () => () => {};
function readStorageAvailable() {
  try { sessionStorage.getItem("vibe-build-drafts:"); return true; }
  catch { return false; }
}
const serverStorageAvailable = () => true;
// One atomic row owns both composers. Hydration cannot persist half a row or
// replace edits made while HTTP is loading. Storage is only an optional mirror.
export function useVibeDrafts(initialID?: string) {
  const ready = useVibeBrowserReady();
  const [firstID] = useState(initialID);
  const restored = useMemo(() => ({ id: firstID, value: ready && firstID ? draftMemory.get(firstID) || readBuildDrafts(firstID) || empty() : empty() }), [ready, firstID]);
  const [edited, setDraft] = useState<typeof restored>();
  const draft = edited || restored;
  const current = useRef<typeof restored | undefined>(undefined);
  const readable = useSyncExternalStore(subscribeStorage, readStorageAvailable, serverStorageAvailable);
  const [written, setStorageAvailable] = useState<boolean>();
  const storageAvailable = written ?? readable;
  const composerEdits = useRef(0);
  const trialEdits = useRef<Record<string, number>>({});
  const publish = useCallback((next: { id: string | undefined; value: BuildDrafts }) => {
    current.current = next;
    if (next.id) setStorageAvailable(writeBuildDrafts(next.id, next.value.guide, next.value.trials));
    setDraft(next);
  }, []);
  const activate = useCallback((id?: string) => {
    const previous = current.current || restored;
    if (previous.id === id) return;
    composerEdits.current++;
    if (previous.id) writeBuildDrafts(previous.id, previous.value.guide, previous.value.trials);
    publish({ id, value: id ? draftMemory.get(id) || readBuildDrafts(id) || empty() : empty() });
  }, [publish, restored]);
  const setContent = useCallback((next: SetStateAction<string>) => {
    const previous = current.current || restored;
    publish({ ...previous, value: { ...previous.value, guide: typeof next === "function" ? next(previous.value.guide) : next } });
  }, [publish, restored]);
  const setTrialBuffers = useCallback((next: SetStateAction<Record<string,string>>) => {
    const previous = current.current || restored;
    publish({ ...previous, value: { ...previous.value, trials: typeof next === "function" ? next(previous.value.trials) : next } });
  }, [publish, restored]);
  const discard = useCallback((id: string) => {
    discardDrafts(id);
  }, []);
  return { storageAvailable, content: draft.value.guide, trialBuffers: draft.value.trials, setContent, setTrialBuffers, composerEdits, trialEdits, activate, discard };
}
