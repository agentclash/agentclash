import { describe, expect, it } from "vitest";
import { defaultModels, type Session } from "./vibe";
import { pendingQuickCheck, quickCheckClientID } from "./vibe-quick-check";

const artifactID = "42316676-f6e5-4af9-8912-ce6936d42c3c";
function ready(): Session {
  return {
    id: "session",
    revision: 2,
    anonymous: true,
    operations: [],
    document: {
      models: defaultModels,
      requirements: [],
      messages: [
        { id: "source", role: "user", content: "Check this conversation" },
      ],
      active_evidence_id: "evidence",
      evidence_sets: [
        {
          id: "evidence",
          label: "Chat",
          raw: "Customer: Hello\nAgent: Hi",
          conversations: [
            {
              key: "chat-1",
              title: "Chat",
              messages: [
                { id: "m1", role: "user", content: "Hello" },
                { id: "m2", role: "assistant", content: "Hi" },
              ],
            },
          ],
        },
      ],
      artifacts: [
        {
          id: artifactID,
          kind: "conversation_evaluation",
          quick_check: true,
          title: "Chat",
          agent_prompt: "",
          blueprint: null,
          accepted: false,
          source_message_id: "source",
          conversation_evaluation: {
            evidence_set_id: "evidence",
            expectations: [{ id: "rule", statement: "Answer the user" }],
          },
        },
      ],
    },
  };
}

describe("server-authorized quick checks", () => {
  it("continues a ready check with an identity that survives a page reload", () => {
    const session = ready();
    expect(pendingQuickCheck(session)?.id).toBe(artifactID);
    expect(quickCheckClientID(artifactID)).toBe(
      quickCheckClientID(structuredClone(session).document.artifacts[0].id),
    );
    expect(quickCheckClientID(artifactID)).not.toBe(artifactID);
  });

  it("does not interpret an ordinary draft, newer question, or a different source as permission", () => {
    const draft = ready();
    draft.document.artifacts[0].quick_check = false;
    expect(pendingQuickCheck(draft)).toBeUndefined();
    const changed = ready();
    changed.document.messages.push({
      id: "new-question",
      role: "user",
      content: "Wait, explain this first",
    });
    expect(pendingQuickCheck(changed)).toBeUndefined();
    const source = ready();
    source.document.active_evidence_id = "other-evidence";
    expect(pendingQuickCheck(source)).toBeUndefined();
  });

  it("never repeats a completed, stopped, or failed check automatically", () => {
    for (const state of ["COMPLETED", "STOPPED", "FAILED"]) {
      const session = ready();
      session.operations.push({
        id: "run",
        kind: "check",
        state,
        models: defaultModels,
        billing: "RELEASED",
        max_cost_nano_usd: 0,
        actual_cost_nano_usd: 0,
        results: [],
        source: {
          kind: "provided_conversations",
          artifact_id: artifactID,
          evidence_set_id: "evidence",
          label: "Chat",
        },
      });
      expect(pendingQuickCheck(session)).toBeUndefined();
    }
  });

  it("waits when speakers are unresolved or a different operation is running", () => {
    const unknown = ready();
    unknown.document.evidence_sets![0].conversations[0].messages[0].role =
      "unknown";
    expect(pendingQuickCheck(unknown)).toBeUndefined();
    const active = ready();
    active.operations.push({
      id: "authoring",
      kind: "message",
      state: "RUNNING",
      models: defaultModels,
      billing: "HELD",
      max_cost_nano_usd: 0,
      actual_cost_nano_usd: null,
      results: [],
    });
    expect(pendingQuickCheck(active)).toBeUndefined();
  });
});
