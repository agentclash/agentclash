// Version 1 matches backend/internal/vibe/interaction/v1.schema.json.
// State describes conversation context; it does not authorize Run, Save or edits.
export type ConversationSource = {
  message_id: string;
  quote: string;
  sha256: string;
};
export type ConversationFact = {
  id: string;
  kind: "job" | "rule" | "has_agent" | "has_pack";
  status: "unknown" | "stated" | "proposed" | "accepted" | "superseded";
  text: string | null;
  sources: ConversationSource[];
  adoption_message_id: string | null;
  supersedes_id: string | null;
};
export type ConversationQuestion = {
  id: string;
  scope_id: string;
  revision: number;
  origin_message_id: string;
  purpose: "clarify_job" | "clarify_rule" | "choose_source" | "adopt_proposal" | "offer_help";
  status: "active" | "answered" | "dismissed" | "superseded";
  text: string;
  options: { id: string; label: string }[];
  max_selections: number;
  proposal_id: string | null;
  proposal_revision: number | null;
};
export type ConversationAnswer = {
  obsolete?: boolean;
  question: ConversationQuestion;
  source: ConversationSource;
  option_ids?: string[];
  unknown: boolean;
};
export type ConversationState = {
  version: 1;
  through_message_id?: string;
  brief: { scope_id: string; revision: number; facts: ConversationFact[] };
  pending_question?: ConversationQuestion;
  answers?: ConversationAnswer[];
  guidance: {
    events?: { message_id: string; kind: "explanation" | "example" | "dismissed"; topic: string }[];
    brevity_preference?: ConversationSource;
  };
};
