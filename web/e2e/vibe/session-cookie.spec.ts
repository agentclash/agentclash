import { randomUUID } from "node:crypto";
import { expect, test, type Page } from "@playwright/test";

// Explicitly opt into the local API. Only session creation and reads reach it;
// every model/operation mutation is intercepted, so this uses zero AI calls.
test.skip(process.env.VIBE_COOKIE_SMOKE !== "1", "Requires the local Vibe API; enable VIBE_COOKIE_SMOKE=1. No AI calls are made.");

async function protectModelCalls(page: Page) {
  const created: string[] = [];
  const interceptedMessages: unknown[] = [];
  await page.route("**/v1/vibe/**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    if (request.method() === "POST" && path.endsWith("/messages")) {
      interceptedMessages.push(request.postDataJSON());
      return route.fulfill({ status: 400, contentType: "application/json", headers: { "Access-Control-Allow-Origin": new URL(page.url()).origin, "Access-Control-Allow-Credentials": "true" }, body: JSON.stringify({ error: { code: "invalid_message", message: "Cookie smoke test: inference intentionally intercepted." } }) });
    }
    if (request.method() === "POST" && path === "/v1/vibe/sessions") {
      created.push(request.postDataJSON().id);
      return route.continue();
    }
    if (request.method() === "GET" || request.method() === "OPTIONS") return route.continue();
    return route.abort();
  });
  return { created, interceptedMessages };
}

test("a real Strict private cookie survives session creation, SSE and reload", async ({ page, context }) => {
  const { created, interceptedMessages } = await protectModelCalls(page);
  await page.goto("/vibe-evals");
  await page.getByRole("button", { name: "Help me build an agent", exact: true }).click();
  const brief = "Build a text receptionist for a bike repair shop using supplied facts.";
  await page.getByRole("textbox", { name: "Message Vibe Evals" }).fill(brief);
  const eventsReady = page.waitForResponse((response) => new URL(response.url()).pathname.endsWith("/events") && response.status() === 200);
  await page.getByRole("button", { name: "Send message", exact: true }).click();
  await expect(page.getByRole("main").getByRole("alert")).toHaveText("Cookie smoke test: inference intentionally intercepted.");
  const events = await eventsReady;
  expect(new URL(events.url()).hostname).toBe(new URL(page.url()).hostname);
  expect(created).toHaveLength(1);
  expect(interceptedMessages).toHaveLength(1);
  expect(interceptedMessages[0]).toMatchObject({ content: brief });
  await expect(page.getByRole("textbox", { name: "Message Vibe Evals" })).toHaveValue(brief);
  const cookie = (await context.cookies()).find((item) => item.name === "vibe_trial");
  expect(cookie && { domain: cookie.domain, httpOnly: cookie.httpOnly, sameSite: cookie.sameSite }).toEqual({ domain: "127.0.0.1", httpOnly: true, sameSite: "Strict" });
  const savedURL = page.url();
  expect(new URL(savedURL).searchParams.get("session")).toBe(created[0]);
  const readReady = page.waitForResponse((response) => new URL(response.url()).pathname === `/v1/vibe/sessions/${created[0]}` && response.status() === 200);
  await page.reload();
  const saved = await (await readReady).json();
  expect(saved.id).toBe(created[0]);
  expect(saved.operations).toHaveLength(0);
  expect(saved.document.messages).toHaveLength(0);
  expect(saved.document.artifacts).toHaveLength(0);
  expect(page.url()).toBe(savedURL);
  expect(created).toHaveLength(1);
  expect(interceptedMessages).toHaveLength(1);
  await expect(page.getByRole("main").getByRole("alert")).toHaveCount(0);
  await expect(page.getByText("Reconnecting to saved progress…", { exact: true })).toHaveCount(0);
});

test("an inaccessible saved session can recover without losing or automatically sending the brief", async ({ page }) => {
  const { created, interceptedMessages } = await protectModelCalls(page);
  await page.goto(`/vibe-evals?session=${randomUUID()}`);
  await expect(page.getByText("This browser can’t access the saved session. Your unsent message is still here.", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Retry loading conversation", exact: true })).toHaveCount(0);
  const brief = "Build a text receptionist for a bike repair shop. Use supplied facts; no booking tools are connected.";
  await page.getByRole("textbox", { name: "Message Vibe Evals" }).fill(brief);
  await expect(page.getByRole("button", { name: "Send message", exact: true })).toBeDisabled();
  await page.getByRole("button", { name: "Keep message in a new conversation", exact: true }).click();
  await expect(page).toHaveURL(/\/vibe-evals$/);
  await expect(page.getByRole("textbox", { name: "Message Vibe Evals" })).toHaveValue(brief);
  expect(created).toHaveLength(0);
  expect(interceptedMessages).toHaveLength(0);
  await page.getByRole("button", { name: "Help me build an agent", exact: true }).click();
  await expect(page.getByText("What should your agent help with?", { exact: true })).toBeVisible();
  await expect(page.getByRole("textbox", { name: "Message Vibe Evals" })).toHaveValue(brief);
  await page.getByRole("button", { name: "Send message", exact: true }).click();
  await expect(page.getByRole("main").getByRole("alert")).toHaveText("Cookie smoke test: inference intentionally intercepted.");
  expect(created).toHaveLength(1);
  expect(interceptedMessages).toHaveLength(1);
  expect(interceptedMessages[0]).toMatchObject({ content: brief, journey_mode: "idea" });
  await expect(page.getByRole("textbox", { name: "Message Vibe Evals" })).toHaveValue(brief);
});
