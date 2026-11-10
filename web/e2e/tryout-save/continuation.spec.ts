import { expect, test, type Page } from "@playwright/test";
const api = `http://127.0.0.1:${process.env.TRYOUT_BROWSER_API_PORT ?? "55741"}`;
const auth = `http://127.0.0.1:${process.env.TRYOUT_BROWSER_AUTH_PORT ?? "55742"}`;
const selectedWorkspace = "b8000000-0000-4000-8000-000000000003";
const newWorkspace = "b8000000-0000-4000-8000-000000000004";
const panel = (page: Page) => page.getByRole("region", { name: "Save tryout" });
async function create(page: Page) {
 await page.goto("/tryouts");
 const result = await page.evaluate(async api => {
  const catalog = await fetch(`${api}/v1/agent-tryout-templates`, { credentials: "omit", cache: "no-store" }).then(response => response.json());
  if (catalog.creator_access_protocol !== "cookie-v1") throw new Error("Fixture must announce cookie-v1");
  const response = await fetch(`${api}/v1/agent-tryouts`, { method: "POST", credentials: "include", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ creator_access_protocol: catalog.creator_access_protocol, template_slug: "meeting-minutes", input: { notes: "Synthetic signup continuation. No model execution." } }) });
  return { status: response.status, body: await response.json() };
 }, api);
 expect(result.status).toBe(201);
 const id: string = result.body.id;
 await page.goto(`/tryouts?tryout=${id}`);
 await expect(panel(page).getByRole("link", { name: "Sign in to save this result" })).toBeVisible();
 return id;
}
async function signIn(page: Page) {
 await panel(page).getByRole("link", { name: "Sign in to save this result" }).click();
 await page.getByRole("button", { name: /Continue with/ }).first().click();
}
async function evidence(page: Page, id: string) {
 const response = await page.request.get(`${api}/__fixture/result/${id}`); expect(response.status()).toBe(200); return response.json();
}
async function save(page: Page, id: string, workspace: string) {
 await expect(panel(page).getByLabel("Save to workspace")).toBeVisible();
 expect((await evidence(page, id)).workspace_id).toBeNull();
 await expect(panel(page).getByRole("button", { name: "Save result", exact: true })).toBeDisabled();
 await panel(page).getByLabel("Save to workspace").selectOption(workspace);
 const workspaceName = await panel(page).locator("option:checked").textContent();
 await panel(page).getByRole("button", { name: "Save result", exact: true }).click();
 await expect(panel(page)).toHaveAttribute("data-save-state", "saved");
 await expect(panel(page)).toHaveAttribute("data-tryout-id", id);
 await expect(panel(page)).toHaveAttribute("data-workspace-id", workspace);
 await expect(panel(page).getByRole("link", { name: workspaceName!, exact: true })).toHaveAttribute("href", `/workspaces/${workspace}`);
 await expect(page.locator('a[href="#save-tryout"]')).toHaveCount(0);
 await expect(page.getByRole("button", { name: "Save result", exact: true })).toHaveCount(0);
 const record = await evidence(page, id); expect(record.workspace_id).toBe(workspace); expect(record.id).toBe(id);
 await page.reload(); await expect(panel(page)).toHaveAttribute("data-tryout-id", id);
 await expect(panel(page).getByRole("link", { name: workspaceName!, exact: true })).toHaveAttribute("href", `/workspaces/${workspace}`);
 expect(new URL(page.url()).searchParams.get("tryout")).toBe(id);
}

test("new account creates workspace, reopens exact result, and explicitly saves", async ({ page, context }, info) => {
 await context.addCookies([{ name: "fixture-identity", value: "unonboarded", url: auth, sameSite: "Lax" }]);
 const id = await create(page);
 // Inspect attributes only. Never persist raw Cookie/Set-Cookie values or a trace.
 const cookie = (await context.cookies(`${api}/v1/agent-tryouts/${id}`)).find(cookie => cookie.name.includes(id));
 expect(Boolean(cookie)).toBe(true); expect(cookie?.httpOnly).toBe(true); expect(cookie?.path).toBe(`/v1/agent-tryouts/${id}`);
 expect(await page.evaluate(() => document.cookie.includes("agentclash_tryout_"))).toBe(false);
 await signIn(page); await expect(page).toHaveURL(/\/onboard\?/);
 await expect(async () => {
  await page.getByRole("textbox").first().fill("Fixture organization");
  await expect(page.getByRole("button", { name: "Continue", exact: true })).toBeEnabled({ timeout: 1_000 });
 }).toPass({ timeout: 15_000 });
 await page.getByRole("button", { name: "Continue", exact: true }).click();
 await page.getByRole("textbox").first().fill("Created workspace"); await page.getByRole("button", { name: /Create/ }).click();
 await expect(page).toHaveURL(new RegExp(`/tryouts\\?tryout=${id}$`));
 await save(page, id, newWorkspace);
 await page.screenshot({ path: info.outputPath("new-account-saved.png") });
});

test("existing account chooses workspace and tampered claim is rejected", async ({ page }, info) => {
 const id = await create(page);
 const membershipRequest = page.waitForRequest(request => request.url().endsWith("/v1/users/me"));
 await signIn(page); await expect(panel(page).getByLabel("Save to workspace")).toBeVisible();
 // Fixture bearer stays in memory only; no headers are written to artifacts.
 const authorization = (await membershipRequest).headers().authorization;
 const forbidden = await page.evaluate(async ({ api, id, authorization }) => {
  const response = await fetch(`${api}/v1/agent-tryouts/${id}/claim`, { method: "POST", credentials: "include", headers: { "Content-Type": "application/json", Authorization: authorization }, body: JSON.stringify({ workspace_id: "b8000000-0000-4000-8000-000000000099" }) }); return response.status;
 }, { api, id, authorization });
 expect(forbidden).toBe(403); expect((await evidence(page, id)).workspace_id).toBeNull();
 await save(page, id, selectedWorkspace);
 const first = await evidence(page, id);
 const retry = await page.evaluate(async ({ api, id, authorization, workspace }) => {
  const response = await fetch(`${api}/v1/agent-tryouts/${id}/claim`, { method: "POST", credentials: "include", headers: { "Content-Type": "application/json", Authorization: authorization }, body: JSON.stringify({ workspace_id: workspace }) }); return { status: response.status, body: await response.json() };
 }, { api, id, authorization, workspace: selectedWorkspace });
 expect(retry.status).toBe(200); expect(retry.body.claimed_at).toBe(first.claimed_at);
 expect((await page.request.get(`${api}/v1/agent-tryouts/${id}`)).status()).toBe(404);
 await page.screenshot({ path: info.outputPath("existing-account-saved.png") });
});

test("cancelled login retains selected unsaved result and can retry", async ({ page, context }) => {
 const id = await create(page);
 await context.addCookies([{ name: "fixture-auth-failure", value: "once", url: auth, sameSite: "Lax" }]);
 await signIn(page); await expect(page).toHaveURL(/error=callback_failed/);
 expect(new URL(page.url()).searchParams.get("returnTo")).toBe(`/tryouts?tryout=${id}`);
 expect((await evidence(page, id)).workspace_id).toBeNull();
 await page.goto(`/tryouts?tryout=${id}`); await expect(panel(page).getByRole("link", { name: "Sign in to save this result" })).toBeVisible();
 await signIn(page); await save(page, id, selectedWorkspace);
});

test("expired and legacy results recover without fabricated save", async ({ page }) => {
 const id = await create(page);
 await page.request.post(`${api}/__fixture/result/${id}/expired`); await page.reload();
 await expect(page.locator("main").getByRole("alert")).toContainText("expired"); await expect(page.getByRole("link", { name: "Start a new tryout" })).toBeVisible();
 expect((await evidence(page, id)).workspace_id).toBeNull();
 const legacy = await create(page); await page.request.post(`${api}/__fixture/result/${legacy}/legacy`); await page.reload();
 await expect(panel(page)).toHaveAttribute("data-save-state", "unavailable"); await expect(panel(page).getByRole("button", { name: "Save result" })).toHaveCount(0);
});

test("UUID substitution and another result's cookie never expose a creator result", async ({ page, browser }) => {
 const id = await create(page);
 const other = await browser.newContext({ baseURL: new URL(page.url()).origin }); const otherPage = await other.newPage();
 try {
  await otherPage.goto(new URL("/tryouts", page.url()).href);
  const otherId = await create(otherPage);
  await otherPage.goto(new URL(`/tryouts?tryout=${id}`, page.url()).href);
  await expect(otherPage.locator("main").getByRole("alert")).toContainText("unavailable in this browser");
  const ownCookie = (await other.cookies(`${api}/v1/agent-tryouts/${otherId}`)).find(cookie => cookie.name.includes(otherId))!;
  await other.addCookies([{ ...ownCookie, name: ownCookie.name.replace(otherId, id), path: ownCookie.path.replace(otherId, id) }]);
  await otherPage.reload(); await expect(otherPage.locator("main").getByRole("alert")).toContainText("unavailable in this browser");
  expect((await otherPage.request.get(`${api}/v1/agent-tryouts/${id}/events`)).status()).toBe(404);
  expect((await evidence(page, id)).workspace_id).toBeNull();
 } finally { await other.close(); }
});


test("missing server capability never offers the old ID-only save", async ({ page }) => {
 const id = await create(page);
 await page.request.post(`${api}/__fixture/result/${id}/old-capability`); await page.reload();
 await expect(panel(page)).toHaveAttribute("data-save-state", "unavailable");
 await expect(panel(page).getByRole("button", { name: "Save result" })).toHaveCount(0);
 expect((await evidence(page, id)).workspace_id).toBeNull();
});


test("active creator can reply and finish before save; private results never offer anonymous turns", async ({ page }, info) => {
 const id = await create(page);
 const membershipRequest = page.waitForRequest(request => request.url().endsWith("/v1/users/me"));
 await signIn(page); await expect(panel(page).getByLabel("Save to workspace")).toBeVisible();
 const authorization = (await membershipRequest).headers().authorization;
 for (const status of ["queued", "running"]) {
  expect((await page.request.post(`${api}/__fixture/result/${id}/${status}`)).status()).toBe(204);
  await page.reload(); await expect(panel(page)).toHaveAttribute("data-save-state", "active");
  await expect(panel(page).getByRole("button")).toHaveCount(0);
  await expect(panel(page).getByRole("combobox")).toHaveCount(0);
  const attempt = await page.evaluate(async ({ api, id, authorization, workspace }) => {
   const response = await fetch(`${api}/v1/agent-tryouts/${id}/claim`, { method: "POST", credentials: "include", headers: { "Content-Type": "application/json", Authorization: authorization }, body: JSON.stringify({ workspace_id: workspace }) });
   return { status: response.status, body: await response.json() };
  }, { api, id, authorization, workspace: selectedWorkspace });
  expect(attempt.status).toBe(409); expect(attempt.body.error.code).toBe("agent_tryout_active");
  expect((await evidence(page, id)).workspace_id).toBeNull();
 }
 await page.getByPlaceholder("Reply to the agent…").fill("Finish the synthetic summary.");
 await page.getByRole("button", { name: "Send", exact: true }).click();
 await expect.poll(async () => (await evidence(page, id)).turn_roles).toEqual(["user"]);
 await page.getByRole("button", { name: "End session", exact: true }).click();
 await expect.poll(async () => (await evidence(page, id)).turn_roles).toEqual(["user", "system_end"]);
 await expect(panel(page)).toHaveAttribute("data-save-state", "ready");
 await save(page, id, selectedWorkspace);
 // A historical private active row remains readable, with no public turn controls.
 await page.request.post(`${api}/__fixture/result/${id}/running`); await page.reload();
 await expect(panel(page)).toHaveAttribute("data-save-state", "saved");
 await expect(panel(page).getByRole("link", { name: "Selected workspace", exact: true })).toBeVisible();
 await expect(page.getByRole("link", { name: "Open workspace", exact: true }).first()).toBeVisible();
 await expect(page.getByPlaceholder("Reply to the agent…")).toHaveCount(0);
 await expect(page.getByRole("button", { name: "End session", exact: true })).toHaveCount(0);
 await expect(page.locator('a[href="#save-tryout"]')).toHaveCount(0);
 await page.evaluate(() => new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
 await page.screenshot({ path: info.outputPath("saved-private-active.png"), animations: "disabled" });
});
