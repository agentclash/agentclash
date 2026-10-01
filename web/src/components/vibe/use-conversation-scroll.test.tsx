import React, { act, useLayoutEffect } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { defaultModels, type Session, type Operation } from "@/lib/vibe";
import { buildReadingAnchor, useConversationScroll } from "./use-conversation-scroll";

function state(): Session {
  const operation: Operation = { id: "trial", kind: "playground", state: "COMPLETED", billing: "SETTLED", models: defaultModels, results: [], max_cost_nano_usd: 0, actual_cost_nano_usd: 0 };
  return { id: "anchor", anonymous: true, revision: 1, document: {
    messages: [{ id: "answer", role: "assistant", content: "Invoice total 42", origin: "playground", artifact_id: "v1", operation_id: "trial" }],
    artifacts: [{ id: "v1", title: "Invoice", blueprint: {}, agent_prompt: "Extract fields", proposal_message_id: "answer" }], requirements: [], models: defaultModels,
    build: { cycle_id: "cycle", phase: "checking", clarifications_used: 0, artifact_id: "v1", trial_id: "trial", check_id: "automatic" },
  }, operations: [operation, { ...operation, id: "automatic", kind: "check", state: "RUNNING", source: { kind: "prompt", label: "Invoice", artifact_id: "v1" } }] };
}
it("keeps automatic checks anchored to their actual reply, but gives an explicit rerun its own anchor", () => {
  const session = state();
  const original = JSON.stringify(session);
  expect(buildReadingAnchor(session)).toBe("message:answer");
  expect(JSON.stringify(session)).toBe(original);
  session.operations.push({ ...session.operations[1], id: "explicit-rerun" });
  expect(buildReadingAnchor(session)).toBe("run:explicit-rerun");
  expect(buildReadingAnchor(session, "pending")).toBe("message:pending");
});
it("never borrows an answer from another version or trial", () => {
  const session = state();
  session.document.messages[0].artifact_id = "old-version";
  expect(buildReadingAnchor(session)).toBe("run:automatic");
  session.document.messages[0].artifact_id = "v1";
  session.document.messages[0].operation_id = "old-trial";
  expect(buildReadingAnchor(session)).toBe("run:automatic");
});

let root: Root, container: HTMLDivElement;
let scroll: ReturnType<typeof useConversationScroll>;
function Harness({ sessionID = "reading", stamp = "1", anchor = "old", build = true }: {sessionID?: string; stamp?: string; anchor?: string; build?: boolean}) {
  const { region, end, onScroll, ...state } = useConversationScroll({ sessionID, stamp, anchor, build, view: "build", reduced: true });
  useLayoutEffect(() => { scroll = { region, end, onScroll, ...state }; });
  return <div ref={region} onScroll={onScroll}><div data-build-entry="answer">Answer</div><div ref={end} /></div>;
}
beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  container = document.createElement("div"); document.body.append(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); vi.unstubAllGlobals(); });
it("follows a new answer once; checks with the same anchor and unrelated rerenders do not move it", async () => {
  await act(async () => root.render(<Harness />));
  const region = scroll.region.current!;
  Object.defineProperties(region, { scrollHeight: { value: 2000 }, clientHeight: { value: 500 } });
  const move = vi.fn(); region.scrollTo = move;
  await act(async () => root.render(<Harness stamp="2" anchor="answer" />));
  expect(move).toHaveBeenCalledTimes(1);
  expect(move.mock.calls[0][0].behavior).toBe("auto");
  await act(async () => root.render(<Harness stamp="3" anchor="answer" />));
  expect(move).toHaveBeenCalledTimes(1);
  await act(async () => root.render(<Harness stamp="3" anchor="answer" />));
  expect(move).toHaveBeenCalledTimes(1);
});
it("preserves history reading and per-context positions, and only jumps on explicit Latest", async () => {
  await act(async () => root.render(<Harness sessionID="saved-reading" />));
  const region = scroll.region.current!;
  Object.defineProperties(region, { scrollHeight: { value: 2000 }, clientHeight: { value: 500 } });
  const move = vi.fn(), jump = vi.fn(); region.scrollTo = move; scroll.end.current!.scrollIntoView = jump;
  await act(async () => { region.scrollTop = 180; region.dispatchEvent(new Event("scroll")); });
  await act(async () => root.render(<Harness sessionID="saved-reading" stamp="2" anchor="answer" />));
  expect(move).not.toHaveBeenCalled(); expect(region.scrollTop).toBe(180); expect(scroll.hasNewResponse).toBe(true);
  await act(async () => root.render(<Harness sessionID="other-context" />));
  expect(region.scrollTop).toBe(0);
  await act(async () => root.render(<Harness sessionID="saved-reading" stamp="2" anchor="answer" />));
  expect(region.scrollTop).toBe(180);
  await act(async () => scroll.jumpToLatest());
  expect(jump).toHaveBeenCalledWith({ behavior: "auto", block: "nearest" });
  expect(scroll.hasNewResponse).toBe(false);
});
