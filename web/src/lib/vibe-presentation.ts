import type { Session } from "./vibe";

export type ExampleCard = { id: string; scope_id: string; origin_message_id: string; kind: "example"; input: string; expected: string; illustrative: true };
const fields = ["id", "scope_id", "origin_message_id", "kind", "input", "expected", "illustrative"];
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

// Fail closed for unknown cards/fields. The model cannot define layout, links,
// evidence, progress, callbacks or execution permissions through this surface.
export function readExampleCard(value: unknown, scope: string | undefined, message: string): ExampleCard | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  const card = value as Record<string, unknown>;
  if (Object.keys(card).length !== fields.length || fields.some(key => !(key in card))) return null;
  if (card.kind !== "example" || card.illustrative !== true || card.scope_id !== scope || card.origin_message_id !== message) return null;
  if (![card.id, card.scope_id, card.origin_message_id].every(id => typeof id === "string" && uuid.test(id))) return null;
  if (![card.input, card.expected].every(text => typeof text === "string" && text.trim() && [...text].length <= 1600)) return null;
  return card as ExampleCard;
}

export function hasConversationChoice(session: Session | null) {
  const state = session?.document.conversation_state;
  if (state?.actions_version !== 1) return false;
  const scope = state.brief.scope_id;
  return (state.proposal?.scope_id === scope && state.proposal.status === "proposed") ||
    (state.pending_question?.scope_id === scope && state.pending_question.status === "active" && (state.pending_question.options.length > 0 || state.pending_question.purpose === "offer_help"));
}

export type PrimarySurface = "composer" | "choices" | "tests" | "results" | "recovery" | "none";
export function primarySurface(input: { busy: boolean; dirty: boolean; typing: boolean; choice: boolean; results: boolean; tests: boolean; recovery: boolean }): PrimarySurface {
  if (input.busy) return "none";
  if (input.dirty) return "tests";
  if (input.typing) return "composer";
  if (input.recovery) return "recovery";
  if (input.results) return "results";
  if (input.choice) return "choices";
  if (input.tests) return "tests";
  return "composer";
}
