import type { Session } from "./vibe";
export function projectTitle(session: Pick<Session, "id" | "title" | "document">) {
  return (
    session.title?.trim() ||
    `${session.document.evaluation?.door === "build" ? "New prototype" : "New evaluation"} ${session.id.slice(0, 8)}`
  );
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
