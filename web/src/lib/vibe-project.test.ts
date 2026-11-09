import { expect, it } from "vitest";
import type { Operation, Session } from "./vibe";
import { projectTitle, recentProjectSummary } from "./vibe-project";

const project = (door?: "build" | "test", extra: Partial<Session> = {}) => ({
  id: "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  document: { ...(door ? { evaluation: { id: "e", chat_id: "c", door } } : {}) } as Session["document"],
  ...extra,
});

it("keeps a project's own title", () => {
  expect(projectTitle(project("build", { title: "  Refund desk  " }))).toBe("Refund desk");
});

it("names every untitled project the same way, with a short stable tag", () => {
  expect(projectTitle(project("build"))).toBe("Untitled project · 3FA8");
  expect(projectTitle(project("test"))).toBe("Untitled project · 3FA8");
  expect(projectTitle(project())).toBe("Untitled project · 3FA8");
  expect(projectTitle(project("build", { title: " " }))).not.toContain("3fa85f64");
});

it("keeps the name when the project changes and tells two projects apart", () => {
  const before = projectTitle(project("build", { updated_at: "2026-10-28T12:00:00Z" }));
  expect(projectTitle(project("build", { updated_at: "2026-10-29T12:00:00Z" }))).toBe(before);
  expect(projectTitle(project("build", { id: "9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d" }))).not.toBe(before);
});

const run = (state: Operation["state"], scorecard?: Partial<NonNullable<Operation["scorecard"]>>, kind: Operation["kind"] = "check") =>
  ({ id: state, kind, state, results: [], ...(scorecard ? { scorecard: { passed: 0, failed: 0, unknown: 0, total: 0, evaluated: 0, pass_rate: null, coverage: 1, ...scorecard } } : {}) }) as unknown as Operation;

it("summarises the latest run of a project for the Continue row", () => {
  expect(recentProjectSummary({ operations: [] })).toEqual({ detail: "No tests have run yet.", tone: "idle" });
  expect(recentProjectSummary({ operations: [run("RUNNING")] })).toEqual({ detail: "A run is in progress.", tone: "running" });
  expect(recentProjectSummary({ operations: [run("COMPLETED", { passed: 2, failed: 1, total: 3 })] })).toEqual({ detail: "1 of 3 tests failed in the last run.", tone: "issue" });
  expect(recentProjectSummary({ operations: [run("COMPLETED", { passed: 3, total: 3 })] })).toEqual({ detail: "All 3 tests passed in the last run.", tone: "pass" });
  expect(recentProjectSummary({ operations: [run("COMPLETED", { passed: 2, unknown: 1, total: 3 })] }).tone).toBe("idle");
  expect(recentProjectSummary({ operations: [run("FAILED")] })).toEqual({ detail: "The last run stopped before it finished.", tone: "issue" });
  // The latest run decides; a later chat operation does not.
  expect(recentProjectSummary({ operations: [run("COMPLETED", { passed: 3, total: 3 }, "retest"), run("COMPLETED", undefined, "chat" as Operation["kind"])] }).tone).toBe("pass");
});
