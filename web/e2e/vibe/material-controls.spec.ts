import { expect, test, type Page } from "@playwright/test";
import { defaultModels, type Session } from "../../src/lib/vibe";
import type { TaskMaterial } from "../../src/lib/vibe-inputs";

const id = "material-fixture";
const source = "Approval is required before publication.";
const ready: TaskMaterial = { id: "saved", session_id: id, client_id: "saved-client", kind: "pdf", name: "quarterly-report-with-a-very-long-filename-that-must-not-overflow-the-dialog.pdf", status: "ready", content_hash: "saved-hash", warnings: ["One page could not be read."], pages: [{ number: 1, text: source }] };

async function serve(page: Page, pdfAvailable = true) {
  const state: Session = { id, revision: 1, anonymous: true, operations: [], document: { format_version: 1, models: defaultModels, messages: [], requirements: [], artifacts: [], evaluation: { id, chat_id: "chat", door: "build" } } };
  const writes: string[] = [], errors: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  page.on("console", message => { if (message.type() === "error") errors.push(message.text()); });
  page.on("response", response => { if (response.status() >= 400) errors.push(`${response.status()} ${response.url()}`); });
  await page.route("**/v1/vibe/**", async route => {
    const request = route.request(), path = new URL(request.url()).pathname;
    const headers = { "Access-Control-Allow-Origin": request.headers().origin || new URL(page.url()).origin, "Access-Control-Allow-Credentials": "true" };
    if (request.method() === "OPTIONS") return route.fulfill({ status: 204, headers: { ...headers, "Access-Control-Allow-Headers": "Content-Type,Authorization,If-Match", "Access-Control-Allow-Methods": "GET,POST,DELETE,OPTIONS" } });
    if (path.endsWith("/config")) return route.fulfill({ headers, json: { enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, two_door: true, material_build: true, pdf_uploads: pdfAvailable, defaults: defaultModels, models: [...new Set(Object.values(defaultModels))].map(id => ({ id, name: id, input_nano_per_token: 0, output_nano_per_token: 0 })) } });
    if (path.endsWith("/saved-checks")) return route.fulfill({ headers, json: [] });
    if (path.endsWith("/evaluations") || path.endsWith("/sessions")) return route.fulfill({ headers, json: [state] });
    if (path.endsWith("/events")) return route.fulfill({ headers, contentType: "text/event-stream", body: `event: snapshot\ndata: ${JSON.stringify(state)}\n\n` });
    if (path.endsWith("/inputs") && request.method() === "GET") return route.fulfill({ headers, json: [ready] });
    if (path.endsWith("/inputs/saved") && request.method() === "GET") return route.fulfill({ headers, json: ready });
    if (path.includes("/inputs/") && request.method() === "DELETE") { writes.push("delete-input"); return route.fulfill({ headers, json: {} }); }
    if (path.endsWith("/inputs") && request.method() === "POST") {
      writes.push("upload-input");
      const pasted = request.headers()["content-type"]?.includes("application/json");
      const body = pasted ? request.postDataJSON() : {};
      const material: TaskMaterial = pasted
        ? { id: "pasted", session_id: id, client_id: body.client_id, kind: "text", name: "Pasted text", status: "ready", content_hash: "text-hash", warnings: [], pages: [{ number: 1, text: body.text }] }
        : { id: "broken", session_id: id, client_id: "pdf-client", kind: "pdf", name: "broken.pdf", status: "failed", content_hash: "pdf-hash", warnings: [], error: "No text found. Paste its text instead." };
      return route.fulfill({ headers, json: material });
    }
    if (request.method() !== "GET") { writes.push("model-write"); return route.fulfill({ status: 400, headers, json: { error: { message: "Unexpected model write" } } }); }
    return route.fulfill({ headers, json: state });
  });
  await page.goto(`/vibe-evals?session=${id}`);
  await expect(page.getByRole("button", { name: "Add a file or text" })).toBeVisible({ timeout: 10000 });
  return { writes, errors };
}

async function open(page: Page) {
  await page.getByRole("button", { name: "Add a file or text" }).click();
  return page.getByRole("dialog", { name: "Add material" });
}

for (const viewport of [{ width: 320, height: 740 }, { width: 390, height: 600 }]) {
  test(`material dialog fits ${viewport.width}×${viewport.height}; file focus and close preserve a paste draft`, async ({ page }) => {
    await page.setViewportSize(viewport);
    const control = await serve(page);
    const dialog = await open(page);
    await expect(dialog.getByRole("tab", { name: "Upload PDF" })).toBeVisible();
    await expect(dialog.getByRole("tab", { name: "Paste text" })).toBeVisible();
    await expect(dialog.getByRole("button", { name: "Close material dialog" })).toBeVisible();
    await expect(dialog.getByRole("heading", { name: "Add material" })).toBeFocused();
    const file = dialog.getByLabel("Choose one PDF");
    await file.focus();
    expect(await dialog.evaluate(el => el.scrollWidth <= el.clientWidth + 1)).toBe(true);
    expect(await dialog.locator(".vibe-material-tab-panel").evaluate(el => el.scrollWidth <= el.clientWidth + 1)).toBe(true);
    for (const button of [dialog.getByRole("tab", { name: "Upload PDF" }), dialog.getByRole("tab", { name: "Paste text" }), dialog.getByRole("button", { name: "Close material dialog" })]) {
      const box = await button.boundingBox(); expect(box).not.toBeNull();
      expect(await button.evaluate(el => { const box = el.getBoundingClientRect(), hit = document.elementFromPoint(box.x + box.width / 2, box.y + box.height / 2); return hit === el || el.contains(hit); })).toBe(true);
    }
    await dialog.getByRole("tab", { name: "Paste text" }).click();
    await dialog.getByRole("textbox", { name: "Text to work on" }).fill("Draft survives closing");
    await page.keyboard.press("Escape");
    await expect(dialog).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Add a file or text" })).toBeFocused();
    await open(page);
    await expect(page.getByRole("textbox", { name: "Text to work on" })).toHaveValue("Draft survives closing");
    expect(control.writes).toEqual([]);
    expect(control.errors).toEqual([]);
  });
}

test("PDF extraction failure remains visible and never starts a model operation", async ({ page }) => {
  const control = await serve(page);
  const dialog = await open(page);
  await dialog.getByLabel("Choose one PDF").setInputFiles({ name: "broken.pdf", mimeType: "application/pdf", buffer: Buffer.from("%PDF-broken") });
  await expect(dialog.getByRole("alert").filter({ hasText: "Paste its text instead" })).toBeVisible();
  await dialog.getByRole("button", { name: "Close material dialog" }).click();
  await expect(page.locator(".vibe-material-chip-status")).toContainText("Reading failed");
  await page.getByRole("button", { name: /Manage attachment/ }).click();
  await expect(page.getByRole("dialog").getByRole("alert").filter({ hasText: "Paste its text instead" })).toBeVisible();
  expect(control.writes).toEqual(["upload-input"]);
  expect(control.errors).toEqual([]);
});

test("closed source remedies focus partial and rule fields; data-use details are reachable before transport", async ({ page }) => {
  await page.setViewportSize({width:320,height:740});
  const control = await serve(page);
  const detailsLink = page.getByRole("link", {name:"Data use and retention (new tab)"}).first();
  await detailsLink.focus();
  const newTab = page.waitForEvent("popup");
  await page.keyboard.press("Enter");
  const details = await newTab;
  await expect(details.getByRole("heading", {name:"Vibe Evals data use and retention",exact:true})).toBeVisible();
  await expect(details.getByRole("heading", {name:"Guest and signed-in materials"})).toBeVisible();
  expect(control.writes).toEqual([]);
  await details.close();
  const dialog = await open(page);
  await dialog.getByRole("tab", {name:"Paste text"}).click();
  await expect(dialog).toContainText("API key [REDACTED]");
  expect(control.writes).toEqual([]);
  await dialog.locator(".vibe-material-saved summary").click();
  await expect(dialog.getByRole("list", { name: "Saved material" })).toContainText(ready.name);
  await dialog.getByRole("button", { name: "Attach", exact: true }).click();
  await expect(dialog.locator(".vibe-material-detail-head")).toContainText("Partial text");
  await page.keyboard.press("Escape");
  const partial = page.getByRole("button", {name:`Review partial text for ${ready.name}`});
  await expect(partial).toBeVisible();
  expect(await page.locator(".vibe-material-chip").evaluate(el => el.scrollWidth <= el.clientWidth + 1)).toBe(true);
  expect((await partial.boundingBox())!.height).toBeGreaterThanOrEqual(44);
  expect(await page.evaluate(()=>document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await partial.focus(); await page.keyboard.press("Enter");
  const checkbox = dialog.getByRole("checkbox", {name:/Use readable text only/});
  await expect(checkbox).toBeFocused();
  await page.keyboard.press("Space");
  await dialog.locator(".vibe-material-config summary").click();
  await dialog.getByLabel("Use as").selectOption("reference");
  await expect(dialog).toContainText("Instructions inside it aren’t rules");
  await dialog.getByLabel("Use as").selectOption("rules");
  await page.keyboard.press("Escape");
  const choose = page.getByRole("button",{name:`Choose rule text for ${ready.name}`});
  await expect(choose).toBeVisible(); await choose.focus(); await page.keyboard.press("Enter");
  await expect(dialog.getByRole("textbox", {name:"Exact text to use as rules"})).toBeFocused();
  await dialog.getByRole("textbox", { name: "Exact text to use as rules" }).fill("Not present");
  await expect(dialog.getByRole("alert")).toContainText("Copy the text exactly");
  await page.keyboard.press("Escape");
  await expect(page.locator(".vibe-material-blockers")).toContainText("Rule text must match page 1 exactly");
  const repair = page.getByRole("button",{name:`Repair rule text for ${ready.name}`});
  await repair.focus(); await page.keyboard.press("Enter");
  await expect(dialog.getByRole("textbox", {name:"Exact text to use as rules"})).toBeFocused();
  await dialog.getByRole("textbox", { name: "Exact text to use as rules" }).fill(source);
  await expect(dialog.getByRole("alert")).toHaveCount(0);
  await dialog.getByRole("textbox", { name: "Exact text to use as rules" }).fill("");
  await page.keyboard.press("Escape");
  const detach = page.getByRole("button",{name:`Detach ${ready.name} and continue`});
  await detach.focus(); await page.keyboard.press("Enter");
  await expect(page.locator(".vibe-material-chip")).toHaveCount(0);
  await expect(page.getByRole("button",{name:"Add a file or text"})).toBeFocused();
  expect(control.writes).toEqual([]);
  await open(page);
  await dialog.locator(".vibe-material-saved summary").click();
  await dialog.getByRole("button", { name: "Attach", exact: true }).click();
  await dialog.getByRole("button", { name: "Delete file", exact: true }).click();
  await expect(dialog.getByRole("alert")).toContainText("Your conversation and existing results remain");
  await dialog.getByRole("button", { name: "Delete file permanently" }).click();
  await expect(dialog.locator(".vibe-material-detail")).toHaveCount(0);
  expect(control.writes).toEqual(["delete-input"]);
  expect(control.errors).toEqual([]);
});

test("recorded Improve inputs show their format and count without opening help", async ({ page }) => {
  const { fakeVibe, transcript, expectation } = await import("./entry-fake");
  const { chooseDoor, entryReady, improveSource } = await import("./entry-model");
  const fake = await fakeVibe(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/vibe-evals"); await entryReady(page);
  await chooseDoor(page, "improve"); await improveSource(page, "recorded");
  const replies = page.getByRole("textbox", { name: "Recorded replies", exact: true });
  await expect(replies).toHaveAttribute("placeholder", /Customer:.*\nAgent:/);
  await expect(replies).toHaveAttribute("placeholder", /---/);
  await replies.fill(transcript); await page.getByLabel("Expected behavior", { exact: true }).fill(expectation);
  await expect(page.getByRole("status").filter({ hasText: "1 conversation · 2 messages" })).toBeVisible();
  expect(fake.posts.filter(post => post.path.endsWith("/evidence"))).toEqual([]);
  await page.getByRole("button", { name: "Review recorded replies", exact: true }).click();
  await expect(page.locator(".vibe-recorded-bubble")).toHaveCount(2);
  await expect(page.locator('.vibe-recorded-bubble[data-role="assistant"]')).toContainText("Agent");
  await expect(page.getByRole("button", { name: "Run recorded tests", exact: true })).toBeEnabled();
  expect(fake.posts.filter(post => post.path.endsWith("/run-quote"))).toEqual([]);
  await page.getByRole("button", { name: "Run recorded tests", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "Run these tests" });
  expect(fake.posts.filter(post => post.path.endsWith("/run-quote"))).toHaveLength(1);
  await dialog.getByText("What happens when I run?", { exact: true }).click();
  await expect(dialog).toContainText("No new replies are generated and your app is not called.");
  await expect(dialog).not.toContainText("selected instructions, materials");
  expect(fake.posts.filter(post => post.path.endsWith("/messages"))).toEqual([]);
});
