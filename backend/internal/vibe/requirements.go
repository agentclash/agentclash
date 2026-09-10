package vibe

import (
	"fmt"
	"github.com/google/uuid"
	"strings"
)

type RequirementChange struct {
	Action        string `json:"action"`
	RequirementID string `json:"requirement_id"`
	Statement     string `json:"statement"`
}

func normalizedRequirement(s string) string { return strings.Join(strings.Fields(s), " ") }

// ReconcileRequirements is also used on a copy before a reply is committed.
// The model proposes changes; it never gets authority to confirm a rule.
func ReconcileRequirements(d *Document, changes []RequirementChange, source, reply uuid.UUID) error {
	for _, change := range changes {
		statement := strings.TrimSpace(change.Statement)
		if len(statement) > 4000 {
			return fmt.Errorf("requirement exceeds 4000 bytes")
		}
		var parent *Requirement
		if change.Action == "add" {
			if change.RequirementID != "" || statement == "" {
				return fmt.Errorf("add requires statement and an empty requirement_id")
			}
			duplicate := false
			for _, q := range d.Requirements {
				if q.Status != "rejected" && q.Status != "superseded" && q.Change != "remove" && normalizedRequirement(q.Statement) == normalizedRequirement(statement) {
					duplicate = true
					break
				}
			}
			if duplicate {
				continue
			}
		} else if change.Action == "replace" || change.Action == "remove" {
			id, err := uuid.Parse(change.RequirementID)
			if err != nil {
				return fmt.Errorf("replacement/removal needs an existing requirement_id")
			}
			for i := range d.Requirements {
				q := &d.Requirements[i]
				if q.ID == id && (q.Status == "proposed" || q.Status == "accepted") {
					parent = q
					break
				}
			}
			if parent == nil {
				return fmt.Errorf("requirement %s is not active", id)
			}
			if change.Action == "replace" && statement == "" {
				return fmt.Errorf("replacement statement is required")
			}
			if change.Action == "remove" {
				statement = parent.Statement
			}
			if change.Action == "replace" && normalizedRequirement(statement) == normalizedRequirement(parent.Statement) {
				continue
			}
			if change.Action == "replace" {
				for _, q := range d.Requirements {
					isAlternative := q.SupersedesID != nil && *q.SupersedesID == id
					if q.ID != id && !isAlternative && (q.Status == "accepted" || q.Status == "proposed") && q.Change != "remove" && normalizedRequirement(q.Statement) == normalizedRequirement(statement) {
						return fmt.Errorf("replacement duplicates an active requirement; propose removing the redundant predecessor by ID instead")
					}
				}
			}
			duplicate := false
			for i := range d.Requirements {
				q := &d.Requirements[i]
				if q.Status == "proposed" && q.SupersedesID != nil && *q.SupersedesID == id {
					if q.Change == change.Action && normalizedRequirement(q.Statement) == normalizedRequirement(statement) {
						duplicate = true
						break
					}
					q.Status = "superseded"
				}
			}
			if duplicate {
				continue
			}
		} else {
			return fmt.Errorf("requirement action must be add, replace or remove")
		}
		next := Requirement{ID: uuid.New(), Statement: statement, Status: "proposed", SourceMessageID: source, ProposalMessageID: &reply, ProposedBy: "assistant", Change: change.Action}
		if parent != nil {
			id := parent.ID
			next.SupersedesID = &id
			if parent.Status == "proposed" {
				// A revision of a pending change still needs confirmation against
				// the confirmed rule, rather than just retiring the pending text.
				if change.Action == "replace" && parent.SupersedesID != nil {
					for _, original := range d.Requirements {
						if original.ID == *parent.SupersedesID && original.Status == "accepted" {
							originalID := original.ID
							next.SupersedesID = &originalID
						}
					}
				}
				parent.Status = "superseded"
				if change.Action == "remove" && *next.SupersedesID == id {
					next.Status = "superseded"
				}
			}
		}
		d.Requirements = append(d.Requirements, next)
	}
	return nil
}

// DecideRequirement runs only inside the authenticated, revision-checked edit
// transaction. Neither authoring output nor conversation summaries can call it.
func DecideRequirement(v *Session, id uuid.UUID, status string, replacement *string) error {
	for i := range v.Document.Requirements {
		q := &v.Document.Requirements[i]
		if q.ID != id {
			continue
		}
		if q.Status == "proposed" && replacement == nil && (status == "accepted" || status == "rejected") {
			if status == "accepted" && q.SupersedesID != nil {
				for j := range v.Document.Requirements {
					old := &v.Document.Requirements[j]
					if old.ID == *q.SupersedesID {
						if old.Status != "accepted" && old.Status != "superseded" {
							return fault("invalid_requirement", "The original requirement changed. Review the current proposal.")
						}
						old.Status = "superseded"
					}
				}
			}
			q.Status = status
			if status == "accepted" {
				now := timestamp()
				q.AcceptedBy, q.AcceptedAt = v.Actor, &now
				if q.Change == "remove" {
					q.Status = "superseded"
				}
			}
			return nil
		}
		if q.Status == "accepted" && status == "superseded" && replacement != nil {
			if strings.TrimSpace(*replacement) == "" || len(*replacement) > 4096 {
				return fault("invalid_requirement", "Write a replacement requirement of at most 4096 bytes.")
			}
			now, source, next := timestamp(), uuid.New(), uuid.New()
			q.Status = "superseded"
			for j := range v.Document.Requirements {
				pending := &v.Document.Requirements[j]
				if pending.Status == "proposed" && pending.SupersedesID != nil && *pending.SupersedesID == id {
					pending.Status = "superseded"
				}
			}
			v.Document.Messages = append(v.Document.Messages, Message{ID: source, Role: "user", Content: "Replace the confirmed requirement with: " + *replacement, CreatedAt: now})
			v.Document.Requirements = append(v.Document.Requirements, Requirement{ID: next, Statement: *replacement, Status: "accepted", SourceMessageID: source, ProposedBy: "user", AcceptedBy: v.Actor, AcceptedAt: &now, SupersedesID: &id})
			return nil
		}
		return fault("invalid_requirement", "Only a proposal can be confirmed or dismissed. Confirmed requirements need an explicit replacement.")
	}
	return fault("not_found", "Requirement is unavailable.")
}
