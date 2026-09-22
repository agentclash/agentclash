import type { Session } from "../../src/lib/vibe";

export const actionModels = { assistant: "openai/gpt-4.1-mini", target: "openai/gpt-4.1-mini", evaluator: "openai/gpt-4.1-mini" };
export const actionConfig = { enabled: true, interaction_actions: true, defaults: actionModels, models: [{ id: actionModels.assistant, name: "Fixture model" }] };
export const actionSession: Session = {
  id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", revision: 4, anonymous: true,
  operations: [], document: { models: actionModels, requirements: [], artifacts: [], messages: [
    { id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", role: "user", content: "My agent answers shop return questions." },
    { id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", role: "assistant", content: "How long can items be returned: 14 days or 30 days?" },
  ], conversation_state: { version: 1, actions_version: 1, through_message_id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", brief: { scope_id: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", revision: 1, facts: [] }, guidance: {}, pending_question: {
    id: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", revision: 1, scope_id: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", origin_message_id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", purpose: "clarify_rule", status: "active", text: "How long can items be returned: 14 days or 30 days?", options: [{ id: "14", label: "14 days" }, { id: "30", label: "30 days" }], max_selections: 1, proposal_id: null, proposal_revision: null,
  } } },
};
