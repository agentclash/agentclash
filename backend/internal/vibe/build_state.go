package vibe

// Build lifecycle changes run inside the existing admission/completion
// transaction. General operation persistence does not decide Build stages.
func admitBuildProgress(d *Document, p Plan, o Operation) {
	if p.Cycle == nil {
		return
	}
	previous := d.Build
	b := &BuildProgress{CycleID: p.Cycle.ID, Phase: "preparing", ClarificationsUsed: p.Cycle.ClarificationsUsed, Sample: p.Cycle.Sample}
	if previous != nil && previous.CycleID == b.CycleID {
		b.TrialID = previous.TrialID
		b.InlineInput = previous.InlineInput
	}
	switch p.Cycle.Step {
	case "check":
		b.Phase = "checking"
		b.ArtifactID = p.Submission.ArtifactID
		b.CheckID = &o.ID
	case "initial_trial":
		b.Phase = "trying"
		b.ArtifactID = p.Submission.ArtifactID
	default:
		if p.taskBuild() && p.Cycle.Step != "prepare" && d.ConversationState != nil && d.ConversationState.PendingQuestion != nil {
			b.RespondingToQuestion = d.ConversationState.PendingQuestion.ID
		}
	}
	d.Build = b
}
func completeBuildProgress(d *Document, p Plan, o Operation, a *Artifact, receipt CompletionReceipt, completion []AuthoringCompletion) error {
	if p.Cycle == nil {
		return nil
	}
	b := &BuildProgress{CycleID: p.Cycle.ID, Phase: "ready", ClarificationsUsed: p.Cycle.ClarificationsUsed, Sample: p.Cycle.Sample}
	if d.Build != nil && d.Build.CycleID == b.CycleID {
		b.TrialID = d.Build.TrialID
		b.ArtifactID = d.Build.ArtifactID
		b.InlineInput = d.Build.InlineInput
	}
	if len(completion) == 1 && completion[0].InlineInput != nil {
		b.InlineInput = completion[0].InlineInput
	}
	if o.Kind == "playground" {
		b.TrialID = &o.ID
		b.ArtifactID = p.Submission.ArtifactID
		d.Build = b
		return nil
	}
	if a != nil {
		b.ArtifactID = &a.ID
		b.Sample = a.Sample
	}
	if a != nil && a.Kind == "task_brief" {
		b.Phase = "blocked"
	}
	if receipt.Action == "clarify" {
		if b.ClarificationsUsed >= 1 {
			return fault("question_budget", "The initial Build cycle cannot ask another question.")
		}
		b.ClarificationsUsed++
		b.Phase = "clarifying"
	}
	if p.taskBuild() && a == nil && receipt.Action != "clarify" {
		b.Phase = "blocked"
		if receipt.Action == "chat" {
			b.Phase = "waiting"
			if len(completion) > 0 && completion[0].ConversationState != nil {
				if q := completion[0].ConversationState.PendingQuestion; q != nil && q.Status == "active" {
					b.Phase = "clarifying"
				}
			}
		}
	}
	d.Build = b
	return nil
}
