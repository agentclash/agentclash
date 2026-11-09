// Tab-local unsent work. Storage is an optional mirror, never a server receipt.
export type EditorDraft = { source: string; value: string; artifact: string };
export type BuildDrafts = { version: 2; guide: string; trials: Record<string, string>; editors: Record<string, EditorDraft> };
export const draftMemory = new Map<string, BuildDrafts>();
export const emptyDrafts = (): BuildDrafts => ({ version: 2, guide: "", trials: {}, editors: {} });
export function readBuildDrafts(id: string): BuildDrafts | undefined {
  try {
    const value = JSON.parse(sessionStorage.getItem("vibe-build-drafts:" + id) || "null");
    if (![1, 2].includes(value?.version)) return draftMemory.get(id);
    const trials: Record<string, string> = {};
    let guide = "";
    if (value.version === 1) {
      if (typeof value.guide !== "string" || value.guide.length > 65536 || !value.trials || typeof value.trials !== "object" || Array.isArray(value.trials)) return draftMemory.get(id);
      guide = value.guide;
      for (const [key, text] of Object.entries(value.trials))
        if (key.startsWith(id + ":") && typeof text === "string" && text.length <= 65536) trials[key] = text;
    } else {
      if (!value.composers || typeof value.composers !== "object" || Array.isArray(value.composers)) return draftMemory.get(id);
      for (const [key, text] of Object.entries(value.composers)) {
        if (!key.startsWith(id + ":") || typeof text !== "string" || text.length > 65536) continue;
        if (key === `${id}:project:composer:guide`) guide = text;
        else {
          const marker = key.indexOf(":composer:prototype:", id.length + 1);
          if (marker >= 0) trials[key.slice(0, marker) + ":" + key.slice(marker + ":composer:prototype:".length)] = text;
        }
      }
    }
    const editors: Record<string, EditorDraft> = {};
    for (const [key, row] of Object.entries(value.version === 2 ? value.editors || {} : {})) {
      const item = row as EditorDraft;
      if (key.startsWith(id + ":") && typeof item?.source === "string" && typeof item.value === "string" && item.value.length <= 1048576 && typeof item.artifact === "string") editors[key] = item;
    }
    return { version: 2, guide, trials, editors };
  } catch { return draftMemory.get(id); }
}
export function persistDrafts(id: string, row: BuildDrafts): boolean {
  draftMemory.set(id, row);
  try {
    const composers: Record<string, string> = { [`${id}:project:composer:guide`]: row.guide };
    for (const [key, text] of Object.entries(row.trials)) {
      if (!key.startsWith(id + ":")) continue;
      const marker = key.indexOf(":", id.length + 1);
      if (marker >= 0) composers[key.slice(0, marker) + ":composer:prototype:" + key.slice(marker + 1)] = text;
    }
    sessionStorage.setItem("vibe-build-drafts:" + id, JSON.stringify({ version: 2, composers, editors: row.editors })); return true;
  }
  catch { return false; }
}
export function writeBuildDrafts(id: string, guide: string, allTrials: Record<string, string>) {
  const previous = draftMemory.get(id) || readBuildDrafts(id) || emptyDrafts();
  return persistDrafts(id, { ...previous, guide, trials: Object.fromEntries(Object.entries(allTrials).filter(([key]) => key.startsWith(id + ":"))) });
}
export function discardDrafts(id: string) {
  draftMemory.delete(id);
  try { sessionStorage.removeItem("vibe-build-drafts:" + id); } catch { /* Optional. */ }
}
// Canonical serialization preserves full editable source, including imported fields.
// Store the canonical content itself: no lossy display projection or SSE revision.
export function editorSource(value: unknown): string {
  if (Array.isArray(value)) return "[" + value.map(editorSource).join(",") + "]";
  if (value && typeof value === "object") return "{" + Object.entries(value).sort(([a], [b]) => a.localeCompare(b)).map(([key, item]) => JSON.stringify(key) + ":" + editorSource(item)).join(",") + "}";
  return JSON.stringify(value) ?? "null";
}

export const editorDraftKey = (project: string, artifact: string, editor: string) => `${project}:${artifact}:editor:${editor}`;
export function readEditorDraft(project: string, artifact: string, editor: string): EditorDraft | undefined {
  const row = (draftMemory.get(project) || readBuildDrafts(project))?.editors[editorDraftKey(project, artifact, editor)];
  return row?.artifact === artifact ? row : undefined;
}
// Acknowledgements can only clear the exact submitted row. Newer edits survive.
export function discardEditorDraft(project: string, artifact: string, editor: string, expected?: EditorDraft): boolean {
  const all = draftMemory.get(project) || readBuildDrafts(project) || emptyDrafts();
  const key = editorDraftKey(project, artifact, editor);
  const row = all.editors[key];
  if (!row || expected && (row.artifact !== expected.artifact || row.source !== expected.source || row.value !== expected.value)) return false;
  const editors = { ...all.editors }; delete editors[key];
  persistDrafts(project, { ...all, editors }); return true;
}
