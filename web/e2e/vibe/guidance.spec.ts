import { expect, test } from "@playwright/test";
import { actionConfig, actionSession } from "./interaction-fixture";

for (const width of [320, 390, 768, 1280, 1440]) {
  test(`contextual example, choice and draft stay separate at ${width}px`, async ({ page }, info) => {
    await page.setViewportSize({ width, height: 960 });
    await page.emulateMedia({ reducedMotion: width === 390 ? "reduce" : "no-preference" });
    const session = structuredClone(actionSession);
    session.document.test_journey = true;
    session.document.messages[0].content = "My agent converts PDFs properly.";
    session.document.messages[1].content = "A test compares an example task with the result you want. Which formatting matters most?";
    const question = session.document.conversation_state!.pending_question!;
    question.text = "Which formatting matters most?";
    question.options = [{ id: "headings", label: "Headings" }, { id: "tables", label: "Tables and the prices beside each product" }];
    session.document.messages[1].cards = [{ id: session.id, kind: "example", illustrative: true,
      scope_id: question.scope_id, origin_message_id: session.document.messages[1].id,
      input: "PDF: Product | Price\nBasic | $20\nPro | $50",
      expected: "| Product | Price |\n| --- | --- |\n| Basic | $20 |\n| Pro | $50 |" }];
    const mutations: { path: string; body: { payload: unknown } }[] = [];
    const errors: string[] = []; page.on("pageerror", error => errors.push(error.message));
    await page.route("**/v1/vibe/**", async route => {
      const req = route.request(), path = new URL(req.url()).pathname;
      const headers = { "Access-Control-Allow-Origin": new URL(info.project.use.baseURL as string).origin, "Access-Control-Allow-Credentials": "true", "Access-Control-Allow-Headers": "Content-Type,Authorization", "Access-Control-Allow-Methods": "GET,POST,OPTIONS" };
      const send = (body: unknown) => route.fulfill({ headers, contentType: "application/json", body: JSON.stringify(body) });
      if (req.method() === "OPTIONS") return route.fulfill({ status: 204, headers });
      if (path.endsWith("/config")) return send(actionConfig);
      if (req.method() !== "GET") {
        mutations.push({ path, body: req.postDataJSON() });
        if (path.endsWith("/actions")) { question.status = "answered"; session.revision++; }
      }
      if (path.endsWith("/events")) return route.fulfill({ headers, contentType: "text/event-stream", body: ": connected\n\n" });
      return send(session);
    });
    await page.goto(`/vibe-evals?session=${session.id}`);
    const example = page.getByRole("region", { name: "Illustrative test" });
    await expect(example).toBeVisible(); await expect(example).toContainText("Example only");
    await expect(example).toContainText("Nothing was run");
    await expect(page.getByRole("group", { name: question.text })).toBeVisible();
    await page.getByRole("button", { name: "Hide example", exact: true }).click();
    await expect(example).toHaveCount(0); expect(mutations).toHaveLength(0);
    await page.getByRole("button", { name: "Show example", exact: true }).click();
    const composer = page.getByRole("textbox", { name: "Message Vibe Evals" });
    await composer.fill("Also preserve hyperlinks.");
    await expect(page.locator(".vibe-button-primary:visible")).toHaveCount(1);
    await expect(page.locator(".vibe-button-primary:visible")).toHaveAttribute("aria-label", "Send message");
    await page.screenshot({ path: info.outputPath(`guidance-${width}.png`), fullPage: true });
    const option = page.getByRole("button", { name: "Tables and the prices beside each product", exact: true });
    await option.focus(); await page.keyboard.press("Enter");
    expect(mutations).toHaveLength(1);
    expect(mutations[0].path).toMatch(/\/actions$/);
    expect(mutations[0].body.payload).toMatchObject({ kind: "answer_question", option_ids: ["tables"], target_id: question.id, session_revision: 4 });
    await expect(composer).toHaveValue("Also preserve hyperlinks.");
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    expect(errors).toEqual([]);
  });
}
