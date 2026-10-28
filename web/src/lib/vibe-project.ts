import type { Session } from "./vibe";
export function projectTitle(session: Pick<Session, "id" | "title" | "document" | "updated_at">) {
  const title = session.title?.trim();
  if (title) return title;
  const fallback = session.document.evaluation?.door === "build" ? "Untitled prototype" : "Untitled project";
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
