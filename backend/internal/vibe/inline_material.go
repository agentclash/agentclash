package vibe

import (
	"context"
	"fmt"
	"github.com/agentclash/agentclash/backend/internal/vibe/inputs"
	"github.com/google/uuid"
	"strings"
)

// Exact byte positions and the original message hash make inline task data
// auditable without copying it into rules or another raw-text snapshot.
type InlineInput struct {
	Binding     inputs.Binding `json:"binding"`
	MessageID   uuid.UUID      `json:"message_id"`
	MessageHash string         `json:"message_hash"`
	Start       int            `json:"start"`
	End         int            `json:"end"`
}

func validateMaterialInterpretation(p Plan, v interpretation) error {
	if v.MaterialQuote == "" && len(v.RequiredCapabilities) == 0 {
		return nil
	}
	if p.AuthoringVersion < materialBuildAuthoringVersion {
		return fmt.Errorf("material interpretation is not available in this version")
	}
	seen := map[string]bool{}
	for _, c := range v.RequiredCapabilities {
		switch c {
		case "text_generation", "pdf_text_extraction", "live_search", "audio_transcription", "database_execution", "business_action":
		default:
			return fmt.Errorf("unknown requested capability")
		}
		if seen[c] {
			return fmt.Errorf("duplicate requested capability")
		}
		seen[c] = true
	}
	if v.MaterialQuote == "" {
		return nil
	}
	if p.Cycle == nil || p.Cycle.Step != "prepare" || hasTaskInput(p.Submission.Inputs) || len(p.Submission.Inputs) >= 2 {
		return fmt.Errorf("inline material is for the first Build without an attached task input")
	}
	start := strings.Index(p.Submission.Content, v.MaterialQuote)
	if start < 0 || strings.Count(p.Submission.Content, v.MaterialQuote) != 1 || inputs.ValidateText(v.MaterialQuote) != nil {
		return fmt.Errorf("material_quote must be the complete unique exact input excerpt")
	}
	for _, obs := range v.Observations {
		pos := strings.Index(p.Submission.Content, obs.Quote)
		if pos < start+len(v.MaterialQuote) && pos+len(obs.Quote) > start {
			return fmt.Errorf("task material cannot also be a job or rule observation")
		}
	}
	return nil
}

func (r *Runner) bindInterpretedMaterial(ctx context.Context, o Operation, p *Plan, route reliableRoute) error {
	p.RequiredCapabilities = route.RequiredCapabilities
	if route.MaterialQuote == "" || route.Intent != "prepare_tests" && route.Intent != "clarify" {
		return nil
	}
	record, err := r.Service.Store.Inputs.Create(ctx, o.SessionID, o.Actor, deterministicID(o.ID, "inline-material"), "text", "Material from your message", []byte(route.MaterialQuote))
	if err != nil {
		return err
	}
	if !record.Usable(timestamp()) {
		return fault("input_unavailable", "The material from this message was deleted or expired. Add it again to continue.")
	}
	start := strings.Index(p.Submission.Content, route.MaterialQuote)
	p.InlineInput = &InlineInput{Binding: inputs.Binding{ID: record.ID, Hash: record.Hash, Usage: "task_input"}, MessageID: p.sourceMessageID(), MessageHash: Hash([]byte(p.Submission.Content)), Start: start, End: start + len(route.MaterialQuote)}
	return nil
}

func validateInlineInput(ctx context.Context, q dbQuery, session uuid.UUID, p Plan, input *InlineInput) error {
	if p.AuthoringVersion < materialBuildAuthoringVersion || input.MessageID != p.sourceMessageID() || input.MessageHash != Hash([]byte(p.Submission.Content)) || input.Start < 0 || input.End <= input.Start || input.End > len(p.Submission.Content) {
		return fault("invalid_input", "The inline material no longer matches its message.")
	}
	record, err := inputs.Resolve(ctx, q, session, input.Binding)
	if err != nil || len(record.Pages) != 1 || record.Pages[0].Text != p.Submission.Content[input.Start:input.End] {
		return fault("input_unavailable", "The material from this message is unavailable. Add it again to continue.")
	}
	return nil
}

func initialTrialInputs(quote BuildQuote, progress *BuildProgress) []inputs.Binding {
	bindings := append([]inputs.Binding(nil), quote.Request.Inputs...)
	if progress.InlineInput != nil {
		bindings = append(bindings, progress.InlineInput.Binding)
	}
	return bindings
}
