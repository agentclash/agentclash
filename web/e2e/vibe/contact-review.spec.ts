import { expect, test, type Page } from "@playwright/test";

const models = { assistant: "fixture/a", target: "fixture/a", evaluator: "fixture/a" };
const artifact = { id: "prototype", kind: "test_suite", title: "Customer email sorter", agent_prompt: "Suggest a label for each email.", accepted: true, source_message_id: "request", proposal_message_id: "prepared", scope_note: "Suggests labels using supplied text. Your inbox is not connected.", blueprint: { cases: [{ key: "spam", payload: { question: "You won a prize. Send bank details." }, expectations: [{ key: "expected_behavior", kind: "text", value: "Label as Spam." }] }] } };
const example = { case_key: "spam", title: "A prize email", version: "prototype", input: { question: "You won a prize. Send bank details." }, expected: "Label as Spam.", output: "Suggested label: Spam", verdict: "PASS", checks: [{ key: "behavior", verdict: "PASS", evidence: "It identified the prize offer as spam." }] };
const session = { id: "contact-fixture", revision: 1, event_cursor: 1, anonymous: true, document: { format_version: 1, evaluation: { id: "contact-fixture", chat_id: "contact-fixture", door: "build" }, test_journey: true, models, requirements: [], artifacts: [artifact], active_artifact_id: artifact.id, messages: [{ id: "request", role: "user", content: "Sort customer emails.", operation_id: "prepare" }, { id: "prepared", role: "assistant", content: "", operation_id: "prepare", artifact_id: artifact.id }], build: { cycle_id: "cycle", phase: "results", clarifications_used: 0, artifact_id: artifact.id, check_id: "check" } }, operations: [{ id: "prepare", kind: "message", state: "COMPLETED", billing: "SETTLED", models, max_cost_nano_usd: 0, actual_cost_nano_usd: 0, results: [] }, { id: "check", kind: "check", state: "COMPLETED", billing: "SETTLED", models, source: { kind: "prompt", artifact_id: artifact.id, label: artifact.title }, max_cost_nano_usd: 0, actual_cost_nano_usd: 0, results: [example], scorecard: { total: 1, passed: 1, failed: 0, unknown: 0, evaluated: 1, coverage: 1, pass_rate: 1 } }] };
type ContactMode = "configured" | "email-only" | "unconfigured";

async function installFixture(page: Page, mode: ContactMode, posts: string[]) {
  const origin = new URL(process.env.VIBE_TEST_BASE_URL ?? "http://127.0.0.1:53517").origin;
  let receiptStatus = "received";
  await page.route("**/v1/vibe/**", async route => {
    const req = route.request(), url = new URL(req.url()), pathname = url.pathname;
    const headers = { "Access-Control-Allow-Origin": origin, "Access-Control-Allow-Credentials": "true" };
    if (req.method() === "OPTIONS") return route.fulfill({ status: 204, headers: { ...headers, "Access-Control-Allow-Headers": "Content-Type,Authorization", "Access-Control-Allow-Methods": "GET,POST,PATCH,OPTIONS" } });
    if (pathname.endsWith("/config")) return route.fulfill({ headers, json: { enabled: true, two_door: true, defaults: models, models: [{ id: "fixture/a", name: "Fixture model" }], contact: { available: mode === "configured", email: mode === "unconfigured" ? "" : "fixture@example.test" } } });
    if (pathname.endsWith("/saved-checks") || pathname.endsWith("/inputs")) return route.fulfill({ headers, json: [] });
    if (pathname.endsWith("/sessions") || pathname.endsWith("/evaluations")) return route.fulfill({ headers, json: [session] });
    if (pathname.endsWith("/events")) return route.fulfill({ headers, contentType: "text/event-stream", body: `event: snapshot\ndata: ${JSON.stringify(session)}\n\n` });
    if (pathname.endsWith("/case")) return route.fulfill({ headers, json: example });
    if (pathname.endsWith("/enquiries") && req.method() === "POST") {
      posts.push(req.postData()!);
      if (posts.length === 1) return route.fulfill({ status: 503, headers, json: { error: { code: "temporary_unavailable", message: "Fixture connection lost" } } });
      return route.fulfill({ headers, json: { id: "receipt", status: "received" } });
    }
    if (pathname.includes("/enquiries/") && req.method() === "GET") {
      receiptStatus = "provider_accepted";
      return route.fulfill({ headers, json: { id: "receipt", status: receiptStatus } });
    }
    if (req.method() !== "GET") throw Error(`Unexpected mutation ${req.method()} ${pathname}`);
    return route.fulfill({ headers, json: session });
  });
}

async function openContact(page: Page) {
  await page.goto("/vibe-evals?session=contact-fixture");
  await page.locator('[data-message-id="request"]').waitFor();
  const opener = page.getByRole("button", { name: "Discuss this with AgentClash" }).first();
  await opener.scrollIntoViewIfNeeded();
  await opener.click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  return { opener, dialog };
}

test("unconfigured and direct-email modes do not collect contact details or send", async ({ page }) => {
  for (const mode of ["unconfigured", "email-only"] as const) {
    const posts: string[] = [];
    await installFixture(page, mode, posts);
    await page.setViewportSize({ width: 320, height: 740 });
    const { opener, dialog } = await openContact(page);
    await expect(dialog.getByLabel("Your email")).toHaveCount(0);
    await expect(dialog.getByLabel("Name (optional)")).toHaveCount(0);
    await expect(dialog.getByRole("button", { name: "Send enquiry" })).toHaveCount(0);
    await expect(dialog.getByRole("button", { name: "Copy summary" })).toBeVisible();
    if (mode === "email-only") await expect(dialog.getByRole("link", { name: "Prefer email? Email us directly" })).toHaveAttribute("href", /^mailto:/);
    else await expect(dialog.getByRole("status")).toContainText("Contact isn’t set up yet");
    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
    await expect(opener).toBeFocused();
    expect(posts).toHaveLength(0);
    await page.unrouteAll();
  }
});

for (const viewport of [{ width: 320, height: 740 }, { width: 390, height: 600 }, { width: 1440, height: 900 }]) {
  test(`configured review fits, validates, and retries the same body at ${viewport.width}×${viewport.height}`, async ({ page }) => {
    const posts: string[] = [];
    await installFixture(page, "configured", posts);
    await page.setViewportSize(viewport);
    const { opener, dialog } = await openContact(page);
    await expect(dialog.getByRole("heading", { name: "Let’s discuss building this for your business" })).toBeFocused();
    const box = await dialog.boundingBox();
    expect(box).toBeTruthy();
    expect(box!.x).toBeGreaterThanOrEqual(15);
    expect(box!.x + box!.width).toBeLessThanOrEqual(viewport.width - 15);
    expect(box!.y).toBeGreaterThanOrEqual(15);
    expect(box!.y + box!.height).toBeLessThanOrEqual(viewport.height - 15);
    const overflow = await dialog.evaluate(el => ({ popup: el.scrollWidth - el.clientWidth, body: (el.querySelector(".vibe-dialog-body")!.scrollWidth - el.querySelector(".vibe-dialog-body")!.clientWidth) }));
    expect(overflow.popup).toBeLessThanOrEqual(1);
    expect(overflow.body).toBeLessThanOrEqual(1);
    expect(posts).toHaveLength(0);
    await dialog.getByRole("button", { name: "Send enquiry" }).click();
    await expect(dialog.getByRole("alert")).toContainText("valid email");
    await expect(dialog.getByRole("alert")).toBeFocused();
    expect(posts).toHaveLength(0);
    await dialog.getByLabel("Your email").fill("person@example.test");
    await dialog.getByRole("button", { name: "Send enquiry" }).click();
    await expect(dialog.getByRole("alert")).toContainText("Fixture connection lost");
    await expect(dialog.getByRole("alert")).toBeFocused();
    expect(posts).toHaveLength(1);
    await page.keyboard.press("Escape");
    await expect(opener).toBeFocused();
    await opener.click();
    await expect(dialog.getByRole("button", { name: "Retry enquiry" })).toBeVisible();
    await dialog.getByRole("button", { name: "Retry enquiry" }).click();
    await expect(dialog.getByRole("status")).toContainText("notification is queued");
    expect(posts).toHaveLength(2);
    expect(posts[1]).toBe(posts[0]);
    await dialog.getByRole("button", { name: "Refresh status" }).click();
    await expect(dialog.getByRole("status")).toContainText("provider accepted");
  });
}

test("an open review dialog refits after desktop-to-phone resize", async ({ page }, info) => {
  const posts: string[] = [];
  await installFixture(page, "configured", posts);
  await page.setViewportSize({ width: 1440, height: 900 });
  const { dialog } = await openContact(page);
  await expect(dialog).toBeVisible();
  await page.setViewportSize({ width: 320, height: 740 });
  await expect.poll(async () => {
    const rect = await dialog.boundingBox();
    if (!rect) return null;
    return { left: Math.round(rect.x), right: Math.round(rect.x + rect.width), top: Math.round(rect.y), bottom: Math.round(rect.y + rect.height) };
  }).toEqual({ left: 16, right: 304, top: expect.any(Number), bottom: expect.any(Number) });
  const rect = await dialog.boundingBox();
  expect(rect!.y).toBeGreaterThanOrEqual(15);
  expect(rect!.y + rect!.height).toBeLessThanOrEqual(725);
  const overflow = await dialog.evaluate(el => ({ popup: el.scrollWidth - el.clientWidth, body: el.querySelector(".vibe-dialog-body")!.scrollWidth - el.querySelector(".vibe-dialog-body")!.clientWidth }));
  expect(overflow.popup).toBeLessThanOrEqual(1);
  expect(overflow.body).toBeLessThanOrEqual(1);
  await page.screenshot({ path: info.outputPath("contact-resized-320.png") });
  expect(posts).toHaveLength(0);
});
