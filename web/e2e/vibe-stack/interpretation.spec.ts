import { expect, test } from "@playwright/test";

test("v15 real stack: joke, vague job, clarification, three clean tests", async ({ page }, info) => {
  test.skip(process.env.VIBE_BROWSER_V15 !== "1", "Versioned opt-in fixture");
  const api = `http://127.0.0.1:${process.env.VIBE_BROWSER_API_PORT ?? "55441"}`;
  const errors: string[] = []; page.on("pageerror", error => errors.push(error.message));
  await page.goto("/vibe-evals");
  const composer = page.getByRole("textbox", { name: "Message Vibe Evals" });
  const send = page.getByRole("button", { name: "Send message", exact: true });
  await composer.fill("drin vodka hewhe");
  await expect(send).toBeEnabled();
  await composer.press("Enter");
  await expect(page.getByText("Tell me what your agent should help with.", { exact: true })).toBeVisible();
  await composer.fill("Build me a returns agent for Shopify");
  await expect(send).toBeEnabled();
  await composer.press("Enter");
  await expect(page.getByText("Which returns should qualify?", { exact: true }).first()).toBeVisible();
  const session = new URL(page.url()).searchParams.get("session");
  let evidence = await (await page.request.get(`${api}/__fixture/evidence?session=${session}`)).json();
  expect(evidence.session.document.artifacts).toHaveLength(0);
  expect(evidence.session.document.conversation_state.pending_question.purpose).toBe("clarify_rule");
  await page.reload();
  await composer.fill("Answer shop return questions. Only unopened items bought within 30 days are eligible. Ask only for missing purchase age or item condition. Never claim to process a refund. Prepare exactly three tests.");
  await expect(send).toBeEnabled();
  await composer.press("Enter");
  await expect.poll(async () => {
    evidence = await (await page.request.get(`${api}/__fixture/evidence?session=${session}`)).json();
    return evidence.session.operations.at(-1)?.completion_receipt?.action;
  }, { timeout: 60_000 }).toBe("prepare_tests");
  expect(evidence.session.document.artifacts).toHaveLength(1);
  const suite = evidence.session.document.artifacts[0];
  expect(suite.blueprint.cases).toHaveLength(3);
  expect(JSON.stringify(suite)).not.toContain("vodka");
  expect(evidence.attempt_count).toBe(5);
  expect(evidence.unsettled_attempts).toBe(0);
  expect(evidence.delivered_operations).toBe(3);
  expect(errors).toEqual([]);
  await page.screenshot({ path: info.outputPath("v15-three-tests.png"), fullPage: true });
});
