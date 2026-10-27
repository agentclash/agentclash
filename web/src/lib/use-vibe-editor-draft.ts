"use client";
import { useState } from "react";
import { draftMemory, emptyDrafts, persistDrafts, readBuildDrafts, editorDraftKey, readEditorDraft, discardEditorDraft } from "./vibe-build-drafts";

export function useVibeEditorDraft(project: string, artifact: string, editor: string, source: string, initial: string) {
  const key = editorDraftKey(project, artifact, editor);
  const [, render] = useState(0);
  const refresh = () => render(value => value + 1);
  const row = readEditorDraft(project, artifact, editor);
  const value = row?.value ?? initial;
  const conflict = !!row && row.source !== source;
  const setValue = (next: string) => {
    const nextRow = { artifact, source: row?.source ?? source, value: next };
    const all = draftMemory.get(project) || readBuildDrafts(project) || emptyDrafts();
    persistDrafts(project, { ...all, editors: { ...all.editors, [key]: nextRow } });
    refresh();
  };
  const discard = (submittedValue?: string) => {
    const current = readEditorDraft(project, artifact, editor);
    if (submittedValue !== undefined && current?.value !== submittedValue) return;
    discardEditorDraft(project, artifact, editor); refresh();
  };
  const preserve = () => {
    const all = draftMemory.get(project) || readBuildDrafts(project) || emptyDrafts();
    const nextRow = { artifact, source, value };
    persistDrafts(project, { ...all, editors: { ...all.editors, [key]: nextRow } }); refresh();
  };
  return { value, setValue, discard, preserve, conflict, dirty: !!row && value !== initial };
}
