import type { Session } from "./vibe";
export function projectTitle(session: Pick<Session, "id" | "title" | "document" | "updated_at">) {
  const title = session.title?.trim();
  if (title) return title;
  const fallback = "Untitled project";
  const updated = session.updated_at ? new Date(session.updated_at) : undefined;
  return updated && !Number.isNaN(updated.getTime()) ? `${fallback} · ${updated.toLocaleDateString(undefined, { month: "short", day: "numeric" })}` : fallback;
}
let sidebarCollapsed = false;
export function sidebarPreference() {
  try {
    return localStorage.getItem("vibe-sidebar:v1") === "collapsed";
  } catch {
    return sidebarCollapsed;
  }
}
export function setSidebarPreference(collapsed: boolean) {
  sidebarCollapsed = collapsed;
  try {
    localStorage.setItem("vibe-sidebar:v1", collapsed ? "collapsed" : "expanded");
  } catch {}
}
// Only a Close on the wide Guide panel sets this; closing the narrow sheet is not a preference.
let guideClosed = false;
export function guidePreference() {
  try {
    return localStorage.getItem("vibe-guide:v1") === "closed";
  } catch {
    return guideClosed;
  }
}
export function setGuidePreference(closed: boolean) {
  guideClosed = closed;
  try {
    localStorage.setItem("vibe-guide:v1", closed ? "closed" : "open");
  } catch {}
}
