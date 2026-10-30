import { resultCopy } from "./entry-model";
import { expect, test } from "@playwright/test";
import { fakeVibe, models } from "./entry-fake";
import { actionSession } from "./interaction-fixture";

for (const width of [390, 1440]) {
  test(`first Build answer continues the quoted cycle at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    const fake = await fakeVibe(page);
    const session = structuredClone(actionSession);
    session.document.models = models;
    session.document.format_version = 1;
    session.document.test_journey = true;
    session.document.evaluation = { id: session.id, chat_id: session.id, door: "build" };
    session.document.build = { cycle_id: "first-build-cycle", phase: "clarifying", clarifications_used: 1 };
    fake.sessions.set(session.id, session);
    await page.goto(`/vibe-evals?session=${session.id}`);
    const answer = page.getByRole("button", { name: "30 days", exact: true });
    await expect(answer).toBeVisible();
    await expect(page.getByRole("textbox", { name: "Message Vibe Evals", exact: true })).toBeFocused();
    await answer.click();
    await expect(page.getByRole("heading", { name: resultCopy.headline(2, 3), exact: true })).toBeVisible();
    expect(fake.posts).toHaveLength(1);
    expect(fake.posts[0]).toMatchObject({ method: "POST", path: `/sessions/${session.id}/messages`, body: { kind: "message", content: "30 days", cycle_id: "first-build-cycle", revision: 4, models: session.document.models } });
    expect(fake.posts.some(post => post.path.endsWith("/actions") || post.path.endsWith("/build-quote"))).toBe(false);
    expect(fake.unknown).toEqual([]);
    expect(fake.errors).toEqual([]);
    // The app shell requests these dev scripts; fakeVibe blocks them.
    const telemetryScripts = ["GET https://va.vercel-scripts.com/v1/script.debug.js", "GET https://va.vercel-scripts.com/v1/speed-insights/script.debug.js"];
    expect(fake.external.filter(request => !telemetryScripts.includes(request))).toEqual([]);
  });
}
