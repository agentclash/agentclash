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
