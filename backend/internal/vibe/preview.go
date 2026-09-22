package vibe

import (
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
)

// Approval is a revision-checked user action, never model-authored metadata.
// Call only inside the transaction that admits checks or saves the artifact.
func acceptArtifact(v *Session, id *uuid.UUID) error {
	if id != nil {
		for i := range v.Document.Artifacts {
			a := &v.Document.Artifacts[i]
			if a.ID == *id && !a.IsTestPlan() {
				a.Accepted = true
				v.Document.ActiveArtifactID = &a.ID
				return nil
			}
		}
	}
	return fault("artifact_required", "Choose the agent and checks to use.")
}

// Only completed exchanges from this exact trial enter target context. The
// client supplies a thread identifier, never assistant messages or answer keys.
func previewMessages(v Session, sub Submission, artifact Artifact) ([]provider.Message, error) {
	messages := []provider.Message{{Role: "system", Content: PreviewPrompt(artifact.AgentPrompt)}}
	if sub.PreviewThreadID != nil {
		if *sub.PreviewThreadID == uuid.Nil {
			return nil, fault("invalid_message", "Choose a valid trial conversation.")
		}
		operations := map[uuid.UUID]Operation{}
		for _, operation := range v.Operations {
			operations[operation.ID] = operation
		}
		for _, message := range v.Document.Messages {
			if message.PreviewThreadID == nil || *message.PreviewThreadID != *sub.PreviewThreadID {
				continue
			}
			if message.Origin != "playground" || message.ArtifactID == nil || *message.ArtifactID != artifact.ID || message.OperationID == nil {
				return nil, fault("preview_changed", "This conversation uses another agent version. Start a new conversation.")
			}
			op, exists := operations[*message.OperationID]
			if !exists || op.Models.Target != sub.Models.Target || op.Kind != "playground" {
				return nil, fault("preview_changed", "This conversation uses another model. Start a new conversation.")
			}
			if op.State != Completed || message.ID == sub.ClientID {
				continue
			}
			if message.Role == "user" || message.Role == "assistant" {
				messages = append(messages, provider.Message{Role: message.Role, Content: message.Content})
			}
		}
	}
	return append(messages, provider.Message{Role: "user", Content: sub.Content}), nil
}
