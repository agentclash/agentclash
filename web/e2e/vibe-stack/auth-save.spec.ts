import { expect, test } from "@playwright/test";

const api = `http://127.0.0.1:${process.env.VIBE_BROWSER_API_PORT ?? "55441"}`;
const description = "Answer shop return questions. Only unopened items bought within 30 days are eligible. Ask only for missing purchase age or item condition. Never claim to process a refund. Prepare exactly three tests.";

test("guest setup resumes the exact selected prototype and baseline before explicit Save", async ({ page, context }, info) => {
  const errors: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  await context.addCookies([{ name: "fixture-identity", value: "unonboarded", url: "http://127.0.0.1:55442", sameSite: "Lax" }]);
  for (const viewport of [{ width: 1440, height: 900 }, { width: 390, height: 844 }, { width: 360, height: 800 }]) {
    await page.setViewportSize(viewport);
    await page.goto("/");
    if (viewport.width < 1024) await page.getByText("Menu", { exact: true }).click();
    await expect(page.getByRole("link", { name: "Try Vibe Evals", exact: true })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({ path: info.outputPath(`landing-navigation-${viewport.width}.png`) });
    await page.getByRole("link", { name: "Try Vibe Evals", exact: true }).click();
    await expect(page).toHaveURL(/\/vibe-evals$/);
    await expect(page.getByRole("button", { name: /Build an agent/ })).toBeVisible();
    if (viewport.width < 1024) await page.getByRole("button", { name: "Open agents", exact: true }).click();
    await page.getByRole("link", { name: "Home", exact: true }).click();
    await expect(page).toHaveURL(/\/$/);
  }
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto("/vibe-evals");
  await page.getByRole("button", { name: /Build an agent/ }).click();
  await page.getByRole("textbox", { name: "Message Vibe Evals" }).fill(description);
  await expect(page.getByRole("button", { name: "Send message", exact: true })).toBeEnabled();
  await page.getByRole("button", { name: "Send message", exact: true }).click();
  const session = new URL(page.url()).searchParams.get("session")!;
  const evidence = async () => (await page.request.get(`${api}/__fixture/evidence?session=${session}`)).json();
  await expect.poll(async () => (await evidence()).session.document.build?.phase, { timeout: 90_000 }).toBe("results");
  const before = await evidence();
  const artifact = before.session.document.artifacts.at(-1).id;
  await page.getByRole("button", { name: "Download / save", exact: true }).click();
  // Save intent selects the current immutable prototype, not a later result.
  await page.getByRole("button", { name: "Save to my account", exact: true }).click();
  const baseline = new URL((await page.getByRole("link", { name: /Sign in/ }).getAttribute("href"))!, "http://localhost").searchParams.get("returnTo")!;
  const baselineID = new URL(baseline, "http://localhost").searchParams.get("keep_run");
  expect(baselineID).toBe(before.session.operations.at(-1).id);
  await page.getByRole("link", { name: /Sign in/ }).click();
  await context.addCookies([{ name: "fixture-auth-failure", value: "once", url: "http://127.0.0.1:55442", sameSite: "Lax" }]);
  await page.getByRole("button", { name: /Continue with/ }).first().click();
  await expect(page).toHaveURL(/error=callback_failed/);
  const retryDestination = new URL(new URL(page.url()).searchParams.get("returnTo")!, "http://localhost");
  expect(retryDestination.pathname).toBe("/vibe-evals");
  expect(Object.fromEntries(retryDestination.searchParams)).toEqual(Object.fromEntries(new URL(baseline, "http://localhost").searchParams));
  expect((await evidence()).calls).toHaveLength(before.calls.length);
  await page.getByRole("button", { name: /Continue with/ }).first().click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.getByRole("link", { name: "Open your workspace" }).click();
  await expect(page).toHaveURL(/\/onboard\?/);
  await page.getByRole("textbox").first().fill(`Vibe onboarding ${Date.now()}`);
  await page.getByRole("button", { name: /Continue/ }).click();
  await page.getByRole("textbox").first().fill("Saved prototypes");
  await page.getByRole("button", { name: /Create/ }).click();
  await expect(page).toHaveURL(/\/vibe-evals\?/);
  expect(new URL(page.url()).searchParams.get("session")).toBe(session);
  expect(new URL(page.url()).searchParams.get("agent")).toBe(artifact);
  await expect(page.getByRole("dialog")).toBeVisible();
  expect((await evidence()).calls).toHaveLength(before.calls.length);
  expect((await evidence()).session.saved_draft_id).toBeFalsy();
  const persistence = page.waitForResponse(response => response.url().endsWith(`/sessions/${session}/save`) && response.request().method() === "POST");
  await page.getByRole("dialog").getByRole("button", { name: "Keep these tests", exact: true }).click();
  const response = await persistence;
  expect(response.ok(), await response.text()).toBeTruthy();
  expect(response.request().postDataJSON()).toMatchObject({ artifact_id: artifact, baseline_operation_id: baselineID });
  const receipt = await response.json();
  expect(receipt.draft_id).toBeTruthy();
  await expect.poll(async () => (await evidence()).session.saved_artifact_id).toBe(artifact);
  expect((await evidence()).session.saved_draft_id).toBe(receipt.draft_id);
  expect((await evidence()).calls).toHaveLength(before.calls.length);
  for (const viewport of [{ width: 1440, height: 900 }, { width: 390, height: 844 }, { width: 360, height: 800 }]) {
    await page.setViewportSize(viewport);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({ path: info.outputPath(`saved-dialog-${viewport.width}.png`), fullPage: true });
  }
  expect(errors).toEqual([]);
});
