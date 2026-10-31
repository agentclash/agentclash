import { expect, test } from "@playwright/test";
import { description, fakeVibe, instructions, project } from "./entry-fake";
import { startComposer, startCopy, startScreen, describeInChat, guideComposer, sendButton, projectReady } from "./entry-model";

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
  await projectReady(page);
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

for (const door of ["build", "improve"] as const) test(`existing empty ${door}: lost first response offers an exact retry`, async ({ page }) => {
  const fake = await fakeVibe(page);
  const id = "a8000000-0000-4000-8000-000000000078";
  fake.sessions.set(id, project(id, door === "build" ? "build" : "test"));
  const bodies: string[] = [];
  await page.route(`**/v1/vibe/sessions/${id}/messages`, async route => {
    bodies.push(route.request().postData()!);
    if (bodies.length === 1) return route.abort("failed");
    return route.fallback();
  });
  await page.route(`**/v1/vibe/sessions/${id}`, route => bodies.length === 1 ? route.abort("failed") : route.fallback());
  await page.goto(`/vibe-evals?session=${id}`);
  await startComposer(page, door).fill(door === "build" ? description : instructions);
  await startScreen(page).getByRole("button", { name: door === "build" ? startCopy.build.primary : startCopy.primaries.instructions, exact: true }).click();
  const retry = page.getByRole("button", { name: "Retry submission", exact: true });
  await expect(retry).toBeVisible();
  await expect(page.locator(".vibe-project-main")).toContainText(door === "build" ? description : instructions);
  await retry.click();
  await expect(retry).toHaveCount(0);
  expect(bodies).toHaveLength(2); expect(bodies[1]).toBe(bodies[0]);
  expect(fake.posts.filter(request => request.path === "/sessions" || request.method === "DELETE")).toEqual([]);
  expect(fake.sessions.size).toBe(1);
});

test("trial rejection permits an edited message with Enter and a new conversation with Send", async ({ page }) => {
  const fake = await fakeVibe(page);
  const id = "a8000000-0000-4000-8000-000000000079";
  fake.sessions.set(id, project(id, "build"));
  await page.goto(`/vibe-evals?session=${id}`);
  await startComposer(page, "build").fill(description);
  await startScreen(page).getByRole("button", { name: startCopy.build.primary, exact: true }).click();
  const input = page.getByRole("textbox", { name: "Message your agent", exact: true });
  await expect(input).toBeVisible();
  const bodies: Record<string, unknown>[] = [];
  await page.route(`**/v1/vibe/sessions/${id}/messages`, async route => {
    const body = route.request().postDataJSON();
    if (body.kind !== "playground") return route.fallback();
    bodies.push(body);
    return route.fulfill({ status: 400, headers: { "Access-Control-Allow-Origin": new URL(page.url()).origin, "Access-Control-Allow-Credentials": "true" }, json: { error: { code: "invalid_message", admission: "rejected", message: `Rejected trial ${bodies.length}.` } } });
  });
  const alert = page.locator('.vibe-prototype-trial [role="alert"]');
  await input.fill("First trial"); await page.getByRole("button", { name: "Send to agent", exact: true }).click();
  await expect(alert).toHaveText("Rejected trial 1.");
  await input.fill("Edited trial"); await input.press("Enter");
  await expect(alert).toHaveText("Rejected trial 2.");
  await page.getByRole("button", { name: "New conversation", exact: true }).click();
  await input.fill("Fresh trial"); await page.getByRole("button", { name: "Send to agent", exact: true }).click();
  await expect(alert).toHaveText("Rejected trial 3.");
  expect(bodies.map(body => body.content)).toEqual(["First trial", "Edited trial", "Fresh trial"]);
});


test("existing empty Improve chat keeps the new start and sends its description to the same project", async ({ page }) => {
  const fake = await fakeVibe(page);
  const id = "a8000000-0000-4000-8000-000000000080";
  fake.sessions.set(id, project(id, "test"));
  await page.goto(`/vibe-evals?session=${id}`);
  await describeInChat(page);
  const description = "Help test my returns agent using our supplied rules.";
  await guideComposer(page).fill(description);
  expect(fake.posts).toEqual([]);
  await sendButton(page, "chat").click();
  await projectReady(page);
  const sent = fake.posts.filter(request => request.path.endsWith("/messages"));
  expect(sent).toHaveLength(1);
  expect(sent[0].path).toBe(`/sessions/${id}/messages`);
  expect(sent[0].body.content).toBe(description);
  expect(sent[0].body.instructions).toBeUndefined();
  expect(fake.posts.filter(request => request.path === "/sessions" || request.method === "DELETE")).toEqual([]);
  expect(fake.sessions.size).toBe(1);
  expect(fake.errors).toEqual([]); expect(fake.unknown).toEqual([]);
});
