import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { Session } from "@/lib/vibe";
import { ConversationActions } from "./conversation-actions";

const scope = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const questionID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
let node: HTMLDivElement;
let root: Root;
beforeEach(() => { vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true); node = document.createElement("div"); document.body.append(node); root = createRoot(node); });
afterEach(async () => { await act(async () => root.unmount()); node.remove(); vi.unstubAllGlobals(); });
function fixture(): Session {
  return { id: scope, revision: 12, operations: [], document: { messages: [], artifacts: [], requirements: [], attachment_count: 0, models: { assistant: "fixture", target: "fixture", evaluator: "fixture" }, conversation_state: {
    version: 1, actions_version: 1, brief: { scope_id: scope, revision: 3, facts: [] }, guidance: {}, through_message_id: scope,
    pending_question: { id: questionID, scope_id: scope, revision: 2, origin_message_id: scope, purpose: "clarify_rule", status: "active", text: "How long can items be returned?", options: [{ id: "fourteen", label: "14 days" }, { id: "thirty", label: "30 days" }], max_selections: 1, proposal_id: null, proposal_revision: null },
  } } } as Session;
}
function button(label: string) { return [...node.querySelectorAll("button")].find(button => button.textContent === label)!; }
async function render(session: Session, onAction = vi.fn().mockResolvedValue(undefined), busy = false) {
  await act(async () => root.render(<ConversationActions session={session} busy={busy} onAction={onAction} onReload={vi.fn()} />));
  return onAction;
}
it("viewing choices invokes nothing; selection sends the displayed IDs and version", async () => {
  const onAction = await render(fixture());
  expect(onAction).not.toHaveBeenCalled();
  await act(async () => button("14 days").click());
  expect(onAction).toHaveBeenCalledTimes(1);
  expect(onAction.mock.calls[0][0]).toMatchObject({ kind: "answer_question", target_id: questionID, target_revision: 2, session_revision: 12, scope_id: scope, option_ids: ["fourteen"], text: null });
});
it("keeps the same idempotency key after a lost acknowledgment and prevents duplicate clicks", async () => {
  let reject!: (reason: Error) => void;
  const onAction = vi.fn().mockImplementationOnce(() => new Promise((_, no) => { reject = no; })).mockResolvedValue(undefined);
  await render(fixture(), onAction);
  await act(async () => { button("14 days").click(); button("14 days").click(); });
  expect(onAction).toHaveBeenCalledTimes(1);
  await act(async () => reject(new Error("Connection interrupted.")));
  expect(node.querySelector('[role="alert"]')?.textContent).toContain("Connection interrupted.");
  await act(async () => button("Try again").click());
  expect(onAction.mock.calls[1][0]).toEqual(onAction.mock.calls[0][0]);
});
it("adopts the displayed group only on explicit click and renders supplied text safely", async () => {
  const session = fixture(); const state = session.document.conversation_state!;
  state.proposal = { id: scope, scope_id: scope, revision: 7, status: "proposed", question: state.pending_question!, facts: [{ id: "fact", kind: "rule", status: "proposed", text: '<script>fetch("/save")</script>', sources: [], adoption_message_id: null, supersedes_id: null }] };
  const onAction = await render(session);
  expect(onAction).not.toHaveBeenCalled();
  expect(node.querySelector("script")).toBeNull();
  expect(node.textContent).toContain('<script>fetch("/save")</script>');
  await act(async () => button("Use these checks").click());
  expect(onAction.mock.calls[0][0]).toMatchObject({ kind: "adopt_proposal", target_id: scope, target_revision: 7, option_ids: [], text: null });
  expect(button("14 days")).toBeUndefined();
});
it("requires an explicit submission for grouped answers", async () => {
  const session = fixture(); session.document.conversation_state!.pending_question!.max_selections = 2;
  const onAction = await render(session);
  await act(async () => (node.querySelector('input[type="checkbox"]') as HTMLInputElement).click());
  expect(onAction).not.toHaveBeenCalled();
  await act(async () => button("Use these answers").click());
  expect(onAction.mock.calls[0][0].option_ids).toEqual(["fourteen"]);
});
it("binds Undo to its saved receipt and hides it when a newer turn arrives", async () => {
  const session = fixture(); session.document.last_change = { id: questionID, scope_id: scope, revision: 11, message_id: scope, summary: "Window updated.", rule_ids: ["window"] };
  const onAction = await render(session);
  expect(node.textContent).toContain("1 rule changed.");
  await act(async () => button("Undo").click());
  expect(onAction.mock.calls[0][0]).toMatchObject({ kind: "undo", target_id: questionID, target_revision: 11 });
  session.document.conversation_state!.through_message_id = questionID;
  await render(session); expect(button("Undo")).toBeUndefined();
});
it("disables controls while busy and hides other-scope or legacy questions", async () => {
  const session = fixture(); const onAction = await render(session, undefined, true);
  expect(button("14 days").disabled).toBe(true);
  await act(async () => button("14 days").click()); expect(onAction).not.toHaveBeenCalled();
  session.document.conversation_state!.pending_question!.scope_id = questionID;
  await render(session); expect(node.querySelector("button")).toBeNull();
  delete session.document.conversation_state!.actions_version;
  await render(session); expect(node.textContent).toBe("");
});
