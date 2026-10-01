// Tab-local unsent text only. Never an execution receipt or a source of rules.
export type BuildDrafts = { version: 1; guide: string; trials: Record<string, string> };
export function readBuildDrafts(id: string): BuildDrafts | undefined {
  try {
    const value = JSON.parse(sessionStorage.getItem("vibe-build-drafts:" + id) || "null");
    if (value?.version !== 1 || typeof value.guide !== "string" || value.guide.length > 65536 || !value.trials || typeof value.trials !== "object" || Array.isArray(value.trials)) return;
    const trials: Record<string, string> = {};
    for (const [key, text] of Object.entries(value.trials))
      if (key.startsWith(id + ":") && typeof text === "string" && text.length <= 65536) trials[key] = text;
    return { version: 1, guide: value.guide, trials };
  } catch { /* In-memory drafts still work when storage is disabled. */ }
}
export function writeBuildDrafts(id: string, guide: string, allTrials: Record<string, string>) {
  try {
    const trials = Object.fromEntries(Object.entries(allTrials).filter(([key]) => key.startsWith(id + ":")).slice(-12));
    sessionStorage.setItem("vibe-build-drafts:" + id, JSON.stringify({ version: 1, guide, trials }));
  } catch { /* Never block sending on storage availability. */ }
}
