import { expect, test } from "@playwright/test";
import { actionConfig, actionSession } from "./interaction-fixture";

for (const width of [320, 390, 768, 1280]) {
  test(`versioned choices and Undo at ${width}px`, async ({ page }, info) => {
    await page.setViewportSize({ width, height: 900 });
    const session = structuredClone(actionSession);
    const actions: Record<string, unknown>[] = [];
    const otherMutations: string[] = [];
    const errors: string[] = [];
    page.on("pageerror", error => errors.push(error.message));
    await page.route("**/v1/vibe/**", async route => {
      const req = route.request(); const path = new URL(req.url()).pathname;
      const headers = { "Access-Control-Allow-Origin": new URL(info.project.use.baseURL as string).origin, "Access-Control-Allow-Credentials": "true", "Access-Control-Allow-Headers": "Content-Type,Authorization", "Access-Control-Allow-Methods": "GET,POST,OPTIONS" };
      const send = (body: unknown) => route.fulfill({ contentType: "application/json", headers, body: JSON.stringify(body) });
      if (req.method() === "OPTIONS") return route.fulfill({ status: 204, headers });
      if (path.endsWith("/config")) return send(actionConfig);
      if (path.endsWith("/actions")) {
        const envelope = req.postDataJSON(); expect(envelope.version).toBe(1); expect(envelope.kind).toBe("action");
        const action = envelope.payload; actions.push(action); session.revision++;
        if (action.kind === "answer_question") {
          session.document.conversation_state!.pending_question!.status = "answered";
          session.document.last_change = { id: action.idempotency_key, revision: session.revision, scope_id: action.scope_id, message_id: actionSession.document.conversation_state!.through_message_id!, summary: "Answer noted." };
        } else { expect(action.kind).toBe("undo"); delete session.document.last_change; }
        return send(session);
      }
      if (req.method() !== "GET") otherMutations.push(path);
      if (path.endsWith("/events")) return route.fulfill({ contentType: "text/event-stream", headers, body: ": connected\n\n" });
      return send(session);
    });
    await page.goto(`/vibe-evals?session=${session.id}`);
    const choices = page.getByRole("region", { name: "Conversation choices" });
    const option = choices.getByRole("button", { name: "14 days", exact: true });
    await expect(option).toBeVisible(); await option.scrollIntoViewIfNeeded();
    await option.focus(); await page.keyboard.press("Enter");
    await expect(choices.getByRole("button", { name: "Undo", exact: true })).toBeVisible();
    expect(actions).toHaveLength(1);
    expect(actions[0]).toMatchObject({ scope_id: actionSession.document.conversation_state!.brief.scope_id, session_revision: 4, kind: "answer_question", target_revision: 1, target_id: actionSession.document.conversation_state!.pending_question!.id, option_ids: ["14"] });
    await page.screenshot({ path: info.outputPath(`choice-${width}.png`), fullPage: true });
    const size = await choices.evaluate(el => ({ width: el.clientWidth, content: el.scrollWidth }));
    expect(size.content).toBeLessThanOrEqual(size.width + 1);
    await choices.getByRole("button", { name: "Undo", exact: true }).click();
    await expect(choices.getByRole("button", { name: "Undo", exact: true })).toHaveCount(0);
    expect(actions[1]).toMatchObject({ kind: "undo", session_revision: 5, target_id: actions[0].idempotency_key, target_revision: 5 });
    expect(otherMutations).toEqual([]); expect(errors).toEqual([]);
  });
}
