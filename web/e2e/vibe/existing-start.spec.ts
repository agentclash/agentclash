import { expect, test } from "@playwright/test";
import { description, fakeVibe, instructions, project } from "./entry-fake";
import { startComposer, startCopy, startScreen } from "./entry-model";

for (const door of ["build", "improve"] as const) test(`existing empty ${door}: first send uses its project`, async ({ page }) => {
  const fake = await fakeVibe(page);
  const id = "a8000000-0000-4000-8000-000000000077";
  fake.sessions.set(id, project(id, door === "build" ? "build" : "test"));
  await page.goto(`/vibe-evals?session=${id}`);
  await expect(startScreen(page).getByRole("heading", { level: 1, name: startCopy[door].heading })).toBeVisible();
  await expect(page.locator("h1:visible")).toHaveCount(1);
  await expect(page.locator(".vibe-project-header, .vibe-composer-dock")).toHaveCount(0);
  if (door === "build") {
    await expect(page.getByText("Free trial", { exact: true })).toBeVisible();
    await expect(startScreen(page).getByRole("button", { name: "Answer return questions", exact: true })).toBeVisible();
    await expect(startScreen(page).getByRole("button", { name: "Sort support emails", exact: true })).toBeVisible();
    await expect(startScreen(page).getByRole("button", { name: "Attach a file", exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "Add a file or text", exact: true })).toBeVisible();
  }
  await startComposer(page, door).fill(door === "build" ? description : instructions);
  expect(fake.posts).toEqual([]);
  await startScreen(page).getByRole("button", { name: door === "build" ? startCopy.build.primary : startCopy.primaries.instructions, exact: true }).click();
  await expect(page.locator(".vibe-project-header")).toBeVisible();
  const sent = fake.posts.filter(request => request.path.endsWith("/messages"));
  expect(sent).toHaveLength(1);
  expect(sent[0].path).toBe(`/sessions/${id}/messages`);
  expect(fake.posts.filter(request => request.path === "/sessions")).toEqual([]);
  expect(fake.sessions.size).toBe(1);
  if (door === "build") expect(sent[0].body.cycle_id).toBeTruthy();
  else expect(sent[0].body.instructions).toBe(instructions);
  expect(fake.errors).toEqual([]);
  expect(fake.unknown).toEqual([]);
});
