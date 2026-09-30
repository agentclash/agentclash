package vibe

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ConversationDecision struct {
	Intent          string    `json:"intent"`
	SourceMessageID uuid.UUID `json:"source_message_id"`
}
type ContextQuote struct {
	Quote        string `json:"quote"`
	SupersedesID string `json:"supersedes_id"`
}
type InstructionEdit struct {
	Before string `json:"before"`
	After  string `json:"after"`
}
type CaseChange struct {
	Action   string  `json:"action"`
	CaseKey  string  `json:"case_key"`
	Input    *string `json:"input"`
	Expected *string `json:"expected"`
}

func validConversationIntent(intent string) bool {
	switch intent {
	case "chat", "clarify", "prepare_tests", "edit_tests", "explain_results", "suggest_fix":
		return true
	}
	return false
}

func (s *Store) commitConversationDecision(ctx context.Context, o Operation, p Plan, intent string) error {
	return s.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		locked, err := lockOperation(ctx, tx, o.ID, projectWrite)
		if err != nil {
			return err
		}
		if locked.State != Running {
			return fault("operation_stopped", "The operation was stopped.")
		}
		decision := ConversationDecision{Intent: intent, SourceMessageID: p.sourceMessageID()}
		if locked.Decision != nil {
			if *locked.Decision != decision {
				return fault("action_changed", "The response changed its action unexpectedly. Your tests are unchanged; please try again.")
			}
			return nil
		}
		if _, err := tx.Exec(ctx, "UPDATE vibe_operations SET conversation_decision=$2 WHERE id=$1", o.ID, raw(decision)); err != nil {
			return err
		}
		return event(ctx, tx, o.SessionID, &o.ID, "conversation.resolved")
	})
}

func hasBehaviorFailure(results []CaseResult) bool {
	for _, c := range results {
		if c.Verdict == Fail && c.Error == nil && strings.TrimSpace(c.Output) != "" {
			for _, check := range c.Checks {
				if check.Verdict == Fail && check.Error == nil {
					return true
				}
			}
		}
	}
	return false
}

func applyInstructionEdits(original string, edits []InstructionEdit, l Limits) (string, error) {
	if len(edits) == 0 || len(edits) > 4 {
		return "", fmt.Errorf("provide one to four focused instruction edits")
	}
	updated := original
	for _, edit := range edits {
		if strings.TrimSpace(edit.Before) == "" || edit.Before == edit.After || strings.Count(original, edit.Before) != 1 || strings.Count(updated, edit.Before) != 1 {
			return "", fmt.Errorf("each before must uniquely match the original instructions and change its wording; edits must not overlap")
		}
		updated = strings.Replace(updated, edit.Before, edit.After, 1)
	}
	if strings.TrimSpace(updated) == "" || len(updated) > l.MessageBytes {
		return "", fmt.Errorf("updated instructions must remain nonempty and fit the instruction size")
	}
	return updated, nil
}
