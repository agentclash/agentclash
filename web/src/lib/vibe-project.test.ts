import { expect, it } from "vitest";
import type { Session } from "./vibe";
import { projectTitle } from "./vibe-project";

const project = (door?: "build" | "test", extra: Partial<Session> = {}) => ({
  id: "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  document: { ...(door ? { evaluation: { id: "e", chat_id: "c", door } } : {}) } as Session["document"],
  ...extra,
});

it("keeps a project's own title", () => {
  expect(projectTitle(project("build", { title: "  Refund desk  " }))).toBe("Refund desk");
});

it("names an untitled project by door without showing its ID", () => {
  expect(projectTitle(project("build"))).toBe("Untitled prototype");
  expect(projectTitle(project("test"))).toBe("Untitled project");
  expect(projectTitle(project())).toBe("Untitled project");
  expect(projectTitle(project("build", { title: " " }))).not.toContain("3fa85f64");
});

it("adds a short date when the project has one", () => {
  const updated_at = "2026-10-28T12:00:00Z";
  const day = new Date(updated_at).toLocaleDateString(undefined, { month: "short", day: "numeric" });
  expect(projectTitle(project("build", { updated_at }))).toBe(`Untitled prototype · ${day}`);
  expect(projectTitle(project("test", { updated_at: "not a date" }))).toBe("Untitled project");
});
