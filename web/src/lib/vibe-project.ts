import { terminal, type Session } from "./vibe";
export function projectTitle(session: Pick<Session, "id" | "title" | "document" | "updated_at">) {
  const title = session.title?.trim();
  if (title) return title;
  const fallback = "Untitled project";
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

/** One line about the latest run of a project, for the "Continue" row of the entry screen. */
export function recentProjectSummary(session: Pick<Session, "operations">): { detail: string; tone: "issue" | "pass" | "running" | "idle" } {
  if (session.operations.some(operation => !terminal(operation.state))) return { detail: "A run is in progress.", tone: "running" };
  const latest = session.operations.findLast(operation => operation.kind === "check" || operation.kind === "retest");
  if (!latest) return { detail: "No tests have run yet.", tone: "idle" };
  const score = latest.scorecard;
  if (latest.state === "COMPLETED" && score?.total) {
    if (score.failed) return { detail: `${score.failed} of ${score.total} tests failed in the last run.`, tone: "issue" };
    if (score.unknown) return { detail: `${score.passed} of ${score.total} tests passed in the last run. ${score.unknown} could not be graded.`, tone: "idle" };
    return { detail: `All ${score.total} tests passed in the last run.`, tone: "pass" };
  }
  if (latest.state === "FAILED") return { detail: "The last run stopped before it finished.", tone: "issue" };
  return { detail: "The last run did not finish.", tone: "idle" };
}
