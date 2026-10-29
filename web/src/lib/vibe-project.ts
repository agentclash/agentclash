import type { Session } from "./vibe";
export function projectTitle(session: Pick<Session, "id" | "title" | "document" | "updated_at">) {
  const title = session.title?.trim();
  if (title) return title;
  const fallback = session.document.evaluation?.door === "build" ? "Untitled prototype" : "Untitled project";
  // A short, stable tag keeps untitled projects apart; a date would repeat and change on every edit.
  const tag = session.id.replace(/[^0-9a-z]/gi, "").slice(0, 4).toUpperCase();
  return tag ? `${fallback} · ${tag}` : fallback;
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
