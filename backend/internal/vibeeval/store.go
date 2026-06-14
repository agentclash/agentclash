package vibeeval

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Message is one persisted turn record (vibe_eval_messages, migration 00058). For
// role=user/assistant, Content is verbatim text (minus a narrow secret scrub once Step 2/3
// adds the chat scrubber); for role=tool, Content is the redacted evidence form (§11.6).
type Message struct {
	ID             uuid.UUID
	ConversationID uuid.UUID
	Seq            int64
	Role           string // user | assistant | tool
	Content        string
	RedactionState string          // none | applied | not_applicable
	ToolCallID     string          // for role=tool / assistant tool-call linkage
	ToolName       string          // for role=tool
	ToolArgs       json.RawMessage // for assistant tool calls (validated)
	ToolCalls      json.RawMessage // assistant rows: full provider tool-call array, so cross-turn replay preserves tool_use/tool_result pairing.
	// NOTE: tool-call ARGS here are persisted and replayed into provider history verbatim
	// (unlike tool RESULTS, which pass through EvidenceRedactor). Safe while tool arg schemas
	// stay narrow (UUIDs/enums, validated on Execute); any future freeform-string tool arg
	// must be reviewed for injection/secret-echo before relying on this replay path.
	Usage     json.RawMessage // token usage on assistant rows (observational, §11.3)
	CreatedAt time.Time
}

// Message roles.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// Redaction states (mirrors the vibe_eval_messages.redaction_state CHECK).
const (
	RedactionNone          = "none"
	RedactionApplied       = "applied"
	RedactionNotApplicable = "not_applicable"
)

// MessageStore persists and replays a conversation's transcript. The api layer supplies a
// repository-backed implementation; vibeeval depends only on this interface.
type MessageStore interface {
	// Append persists a message and returns it with the DB-assigned Seq/ID/CreatedAt.
	Append(ctx context.Context, msg Message) (Message, error)
	// History returns the conversation's messages in seq order.
	History(ctx context.Context, conversationID uuid.UUID) ([]Message, error)
}
