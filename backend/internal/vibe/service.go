package vibe

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/agentclash/agentclash/runtime/challengepack"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
	"strings"
)

type Compiled struct {
	Bundle      challengepack.Bundle
	Composition json.RawMessage
	Cases       []challengepack.CaseDefinition
}
type Compiler interface {
	Instructions() string
	Draft(DraftProposal, Limits) (json.RawMessage, error)
	Compile(json.RawMessage, string, uuid.UUID, Limits) (Compiled, error)
}
type Service struct {
	Store    *Store
	Config   Config
	Gate     Gate
	Compiler Compiler
}

func (s *Service) Prepare(ctx context.Context, actor string, id uuid.UUID, sub Submission) (Operation, error) {
	if !s.Config.Enabled || s.Config.Credential == "" {
		return Operation{}, fault("hosted_disabled", "Hosted Vibe execution is not configured yet. You can still import and review an evaluation.")
	}
	v, err := s.Store.GetSession(ctx, actor, id)
	if err != nil {
		return Operation{}, err
	}
	l := s.Config.Limits(v.Anonymous)
	if sub.Purpose != "" && !((sub.Purpose == "suggest_change" && sub.Kind == "message" || sub.Purpose == "regrade" && sub.Kind == "retest" && sub.Content == "" && sub.EvidenceSetID == nil && sub.Interaction == nil) && sub.BaselineID != nil && sub.Instructions == "") {
		return Operation{}, fault("invalid_message", "Choose a completed check for this action.")
	}
	if sub.JourneyMode != "" && (sub.Kind != "message" || (sub.JourneyMode != "idea" && sub.JourneyMode != "existing" && sub.JourneyMode != "exploring")) {
		return Operation{}, fault("invalid_message", "Choose idea, existing agent, or exploration for a design message.")
	}
	if sub.ClientID == uuid.Nil || sub.Revision < 0 || len(sub.Content) > l.MessageBytes || len(sub.Instructions) > l.MessageBytes || (sub.Instructions != "" && sub.Kind != "message") {
		return Operation{}, fault("invalid_message", "The message is missing an ID or exceeds its size limit.")
	}
	if sub.ApproveArtifact && sub.Kind != "check" && sub.Kind != "retest" || sub.PreviewThreadID != nil && sub.Kind != "playground" {
		return Operation{}, fault("invalid_message", "This action does not support that approval or trial conversation.")
	}
	if sub.QuickCheck && (sub.Kind != "message" || (!sub.EvaluationFirst && !v.Document.EvaluationFirst) || sub.Instructions != "") {
		return Operation{}, fault("invalid_message", "Quick checks apply to messages containing recorded replies.")
	}
	if (sub.Kind == "message" || sub.Kind == "build") && !sub.EvaluationFirst && !v.Document.EvaluationFirst {
		question := briefQuestion(v.Document, sub.Content)
		// A pre-upgrade starter may already be queued. Let Store.Submit recover
		// its immutable receipt (or reject changed content) using the same ID.
		for _, message := range v.Document.Messages {
			if message.ID == sub.ClientID {
				question = ""
				break
			}
		}
		if question != "" {
			// Existing clients already recognize this as a pre-admission rejection.
			return Operation{}, fault("invalid_message", question)
		}
	}
	if err = s.validateSubmissionModels(sub, v); err != nil {
		return Operation{}, err
	}
	if err = s.Gate.Check(ctx, actor, l); err != nil {
		return Operation{}, err
	}
	// Recover an immutable receipt before rebuilding a trial's now-longer
	// context. A lost acknowledgement must not turn into a new context error.
	if receipt, e := s.Store.submissionReceipt(ctx, id, sub); receipt != nil || e != nil {
		if e != nil {
			return Operation{}, e
		}
		return *receipt, nil
	}
	if sub.Interaction != nil {
		if !s.Config.PreciseActions {
			return Operation{}, fault("invalid_request", "Reload to use the current conversation controls.")
		}
		if err := validateSourceAction(v, sub); err != nil {
			return Operation{}, err
		}
	}
	p := Plan{AuthoringVersion: 4, Submission: sub, Document: v.Document, Anonymous: v.Anonymous, Free: s.Config.FreeOnly, LocalTesting: s.Config.TestingLocally()}
	if sub.Purpose == "regrade" {
		return s.prepareRegrade(ctx, actor, v, sub, p)
	}
	if (sub.TestJourney || v.Document.TestJourney) && (sub.Kind == "message" || sub.Kind == "build") {
		return s.prepareTestConversation(ctx, actor, v, sub, p)
	}
	if !sub.TestJourney && !v.Document.TestJourney && (sub.EvaluationFirst || v.Document.EvaluationFirst) {
		p.Document.EvaluationFirst = true
		p.Evidence = findEvidence(v.Document, sub.EvidenceSetID)
		if sub.Kind == "message" && sub.Instructions == "" && sub.EvidenceSetID == nil && sub.BaselineID == nil {
			p.InlineEvidence, err = ParseMessageEvidence(sub.Content, l)
			if err != nil {
				return Operation{}, err
			}
			if p.InlineEvidence != nil {
				p.Evidence = p.InlineEvidence
			}
		}
		if sub.Instructions != "" {
			p.Evidence = nil
		}
		selected := sub.ArtifactID
		if selected == nil && sub.Instructions == "" {
			selected = v.Document.ActiveArtifactID
		}
		for _, a := range v.Document.Artifacts {
			if selected != nil && a.ID == *selected {
				copy := a
				p.Artifact = &copy
			}
		}
		if sub.Instructions != "" || p.InlineEvidence != nil {
			p.Artifact = nil
		}
		if p.Artifact != nil && !p.Artifact.IsConversationEvaluation() && sub.Kind != "message" && sub.Kind != "build" {
			// A later chat upload cannot change an older instruction test or preview.
			p.Evidence = nil
		}
		if p.Evidence != nil || p.Artifact != nil && p.Artifact.IsConversationEvaluation() || (sub.Kind == "message" && p.Artifact == nil && sub.Instructions == "") {
			return s.prepareConversations(ctx, actor, v, sub, p)
		}
		// Explicitly supplied instructions opt into a local text test.
		if sub.Instructions != "" {
			p.Document.Journey.Mode = "idea"
			p.Document.Journey.PreviewConsent = true
		}
	}
	if sub.JourneyMode != "" {
		p.Document.Journey.Mode = sub.JourneyMode
	}
	if sub.Kind == "message" || sub.Kind == "build" {
		if strings.TrimSpace(sub.Content) == "" {
			return Operation{}, fault("invalid_message", "Write a message first.")
		}
		selected := sub.ArtifactID
		if sub.Instructions != "" {
			selected = nil
			p.Document.Artifacts = nil
		}
		if selected == nil && sub.Instructions == "" {
			selected = v.Document.ActiveArtifactID
		}
		if selected != nil {
			found := false
			for _, a := range v.Document.Artifacts {
				if a.ID == *selected {
					found = true
					p.Document.Artifacts = []Artifact{a}
					if a.Accepted && !a.IsTestPlan() {
						copy := a
						p.Artifact = &copy
					}
					break
				}
			}
			if !found {
				return Operation{}, fault("artifact_required", "Choose an existing agent version.")
			}
		}
		p.Document.Messages = nil
		for _, message := range v.Document.Messages {
			if message.Origin != "playground" {
				p.Document.Messages = append(p.Document.Messages, message)
			}
		}
		if len(p.Document.Messages) > 6 {
			p.Document.Messages = p.Document.Messages[len(p.Document.Messages)-6:]
		}
		if len(p.Document.Artifacts) > 1 {
			p.Document.Artifacts = p.Document.Artifacts[len(p.Document.Artifacts)-1:]
		}
		for i := len(v.Operations) - 1; i >= 0; i-- {
			if sub.BaselineID != nil && v.Operations[i].ID != *sub.BaselineID {
				continue
			}
			if len(v.Operations[i].Results) > 0 {
				if selected != nil {
					if !v.Operations[i].State.Terminal() || v.Operations[i].Results[0].Version != selected.String() {
						continue
					}
					// Coach against the checks that produced this version's
					// evidence, including a retest with a different baseline.
					original, e := s.Store.Operation(ctx, v.Operations[i].ID)
					if e != nil {
						return Operation{}, e
					}
					var tested Plan
					if e = json.Unmarshal(original.Input, &tested); e != nil {
						return Operation{}, e
					}
					if tested.Artifact != nil {
						copy := p.Document.Artifacts[0]
						copy.Blueprint = tested.Artifact.Blueprint
						p.Artifact = &copy
					}
				}
				for _, result := range v.Operations[i].Results {
					if len(p.Observations) == 3 {
						break
					}
					result, err = s.Store.GetCase(ctx, actor, v.Operations[i].ID, result.CaseKey)
					if err != nil {
						return Operation{}, err
					}
					if len(result.Output) > 2048 {
						result.Output = result.Output[:2048] + " [excerpt; full output is saved in the scorecard]"
					}
					p.Observations = append(p.Observations, result)
				}
				break
			}
		}
		if sub.BaselineID != nil && (selected == nil || len(p.Observations) == 0) {
			return Operation{}, fault("baseline_required", "Choose a completed check for this agent version before improving from its evidence.")
		}
		p.Calls = 2 // one authoring call and, only if needed, one repair
		profile, e := s.Config.Profile(sub.Models.Assistant)
		if e != nil {
			return Operation{}, e
		}
		cost, e := profile.BoundCost(profile.inputLimit(l), l.OutputTokens)
		if e != nil {
			return Operation{}, e
		}
		p.MaxCost = cost * int64(p.Calls)
	} else if sub.Kind == "check" || sub.Kind == "retest" || sub.Kind == "playground" {
		p.Document = Document{}
		if sub.ArtifactID == nil {
			return Operation{}, fault("artifact_required", "Choose an agent first.")
		}
		for _, a := range v.Document.Artifacts {
			if a.ID == *sub.ArtifactID && (a.Accepted || sub.Kind == "playground" || sub.ApproveArtifact) {
				copy := a
				p.Artifact = &copy
				break
			}
		}
		if p.Artifact != nil && p.Artifact.IsTestPlan() {
			return Operation{}, fault("artifact_required", "This is a test plan. Review or export it to test in your own environment; it cannot run a customer trial or evaluation here.")
		}
		if p.Artifact == nil {
			return Operation{}, fault("artifact_required", "Review the expected behavior, then run these checks.")
		}
		if p.Artifact.IsTestSuite() && strings.TrimSpace(p.Artifact.AgentPrompt) == "" {
			return Operation{}, fault("agent_required", "Your tests are ready. Add the agent you want to test before running them.")
		}
		if !p.Artifact.IsTestSuite() && v.Document.Journey.Mode == "existing" && !v.Document.Journey.PreviewConsent {
			return Operation{}, fault("preview_consent_required", "Your existing agent is not connected. Choose a text simulation explicitly to try its replies here.")
		}
		if sub.Kind == "retest" {
			if sub.BaselineID == nil {
				return Operation{}, fault("baseline_required", "Choose the original check for a fair retest.")
			}
			var baseline *Operation
			for i := range v.Operations {
				if v.Operations[i].ID == *sub.BaselineID {
					baseline = &v.Operations[i]
				}
			}
			if baseline == nil || !baseline.State.Terminal() || (baseline.Kind != "check" && baseline.Kind != "retest") {
				return Operation{}, fault("baseline_required", "The original completed check is unavailable.")
			}
			var old Plan
			original, e := s.Store.Operation(ctx, baseline.ID)
			if e != nil {
				return Operation{}, e
			}
			if err = json.Unmarshal(original.Input, &old); err != nil {
				return Operation{}, err
			}
			if old.Artifact == nil {
				return Operation{}, fault("baseline_required", "The original evaluation is unavailable.")
			}
			if p.Artifact.IsTestSuite() && (!old.Artifact.IsTestSuite() || Hash(p.Artifact.Blueprint) != Hash(old.Artifact.Blueprint)) {
				return Operation{}, fault("comparison_changed", "These tests changed since the last run. Run them as a new test set to keep that difference clear.")
			}
			p.Artifact.Blueprint = old.Artifact.Blueprint // unchanged cases, validators and rubric
			if sub.Models.Evaluator != baseline.Models.Evaluator {
				return Operation{}, fault("comparison_changed", "A fair retest must keep the original evaluator model. Start a new check to change it.")
			}
		}
		if sub.Kind == "playground" {
			if sub.Content == "" {
				return Operation{}, fault("invalid_message", "Write a test message for the agent.")
			}
			p.Calls = 1
			profile, e := s.Config.Profile(sub.Models.Target)
			if e != nil {
				return Operation{}, e
			}
			p.MaxCost, err = profile.BoundCost(profile.inputLimit(l), l.OutputTokens)
			if err == nil {
				p.PreviewMessages, err = previewMessages(v, sub, *p.Artifact)
			}
			if err == nil {
				_, err = CountContext(provider.Request{Messages: p.PreviewMessages, MaxOutputTokens: l.OutputTokens}, profile, l)
				if f, ok := err.(*Fault); ok && f.Code == "context_limit" {
					f.Message = "This trial conversation has reached its context limit. Start a new conversation; your previous messages are preserved. Nothing was sent."
				}
			}
		} else {
			p.Source = &EvaluationSource{Kind: "prompt", Label: "Text test · instructions + model", ArtifactID: p.Artifact.ID}
			compiled, e := s.Compiler.Compile(p.Artifact.Blueprint, sub.Models.Evaluator, p.Artifact.ID, l)
			if e != nil {
				return Operation{}, e
			}
			if len(compiled.Cases) > l.Cases {
				return Operation{}, fault("case_limit", fmt.Sprintf("This evaluation has %d cases; the current limit is %d. Create an explicitly smaller draft to preview it.", len(compiled.Cases), l.Cases))
			}
			judges := len(compiled.Bundle.Version.EvaluationSpec.LLMJudges)
			p.ChecksPerCase = judges + len(compiled.Bundle.Version.EvaluationSpec.Validators)
			p.Calls, err = GraphCalls(len(compiled.Cases), 1, 1, judges, 1, 0, 0, l)
			if err != nil {
				return Operation{}, err
			}
			target, _ := s.Config.Profile(sub.Models.Target)
			judge, _ := s.Config.Profile(sub.Models.Evaluator)
			tc, e := target.BoundCost(target.inputLimit(l), l.OutputTokens)
			if e != nil {
				return Operation{}, e
			}
			jc, e := judge.BoundCost(judge.inputLimit(l), l.OutputTokens)
			if e != nil {
				return Operation{}, e
			}
			p.MaxCost = int64(len(compiled.Cases)) * (tc + int64(judges)*jc)
			for _, c := range compiled.Cases {
				p.Cases = append(p.Cases, c.CaseKey)
				p.CasePreviews = append(p.CasePreviews, CaseResult{CaseKey: c.CaseKey, Input: raw(c.Payload), Expected: ExpectedBehavior(c)})
			}
		}
	} else {
		return Operation{}, fault("invalid_operation", "This operation is not supported.")
	}
	if err != nil {
		return Operation{}, err
	}
	if sub.Kind == "check" || sub.Kind == "retest" {
		if err = s.prepareRunValidation(&p, v); err != nil {
			return Operation{}, err
		}
		if err = s.freezeGrading(&p); err != nil {
			return Operation{}, err
		}
		if err = s.verifyComparison(ctx, p); err != nil {
			return Operation{}, err
		}
	}
	return s.Store.Submit(ctx, actor, id, sub, p, s.Config)
}
func (s *Service) Import(ctx context.Context, actor string, id uuid.UUID, revision int64, content []byte) error {
	v, err := s.Store.GetSession(ctx, actor, id)
	if err != nil {
		return err
	}
	l := s.Config.Limits(v.Anonymous)
	if err = s.Gate.Check(ctx, actor, l); err != nil {
		return err
	}
	b, err := ImportJSON(content, l)
	if err != nil {
		return err
	}
	// Round-trip our explicitly versioned export. Model preferences are data;
	// importing a file cannot change the active model policy or start execution.
	agentPrompt := ""
	var envelope map[string]json.RawMessage
	if err = json.Unmarshal(b, &envelope); err != nil {
		return err
	}
	if _, ok := envelope["format"]; ok {
		var exported struct {
			Format      string          `json:"format"`
			AgentPrompt string          `json:"agent_prompt"`
			Evaluation  json.RawMessage `json:"evaluation"`
			Models      Models          `json:"models"`
		}
		if err = Decode(b, l, &exported); err != nil {
			return err
		}
		if exported.Format != "agentclash-vibe-v1" || strings.TrimSpace(exported.AgentPrompt) == "" || len(exported.AgentPrompt) > l.MessageBytes {
			return fault("invalid_pack", "The agent export format or instructions are invalid.")
		}
		agentPrompt, b = exported.AgentPrompt, exported.Evaluation
	}
	artifactID := uuid.New()
	c, err := s.Compiler.Compile(b, v.Document.Models.Evaluator, artifactID, l)
	if err != nil {
		return fault("invalid_pack", "This evaluation could not be imported without changing its coverage: "+err.Error())
	}
	return s.Store.Edit(ctx, actor, id, revision, func(v *Session) error {
		if !s.Config.TestingLocally() && v.Document.AttachmentCount >= l.Files {
			return fault("attachment_limit", "This conversation has reached its import count limit. Save it and start another.")
		}
		total := len(content)
		for _, a := range v.Document.Artifacts {
			total += len(a.Blueprint)
		}
		if !s.Config.TestingLocally() && total > l.StoredBytes {
			return fault("attachment_limit", "The conversation's attachment allowance is full.")
		}
		v.Document.AttachmentCount++
		msg := Message{ID: uuid.New(), Role: "user", Content: fmt.Sprintf("Imported %s (%d cases).", c.Bundle.Pack.Name, len(c.Cases)), CreatedAt: timestamp()}
		v.Document.Messages = append(v.Document.Messages, msg)
		replyID := uuid.New()
		v.Document.TestJourney = true
		v.Document.Artifacts = append(v.Document.Artifacts, Artifact{ID: artifactID, Kind: "test_suite", Provenance: "imported", Title: c.Bundle.Pack.Name, AgentPrompt: agentPrompt, Blueprint: b, SourceMessageID: msg.ID, ProposalMessageID: &replyID, CreatedAt: timestamp()})
		v.Document.Messages = append(v.Document.Messages, Message{ID: replyID, Role: "assistant", Content: fmt.Sprintf("Your %d imported tests are ready to review.", len(c.Cases)), ArtifactID: &artifactID, CreatedAt: timestamp()})
		return nil
	})
}
func (s *Service) Save(ctx context.Context, actor string, id uuid.UUID, revision int64, artifactID, ws uuid.UUID, selected *Models, approve ...bool) (uuid.UUID, error) {
	return s.SaveWithBaseline(ctx, actor, id, revision, artifactID, ws, selected, nil, approve...)
}
func (s *Service) SaveWithBaseline(ctx context.Context, actor string, id uuid.UUID, revision int64, artifactID, ws uuid.UUID, selected *Models, baseline *uuid.UUID, approve ...bool) (uuid.UUID, error) {
	v, err := s.Store.GetSession(ctx, actor, id)
	if err != nil {
		return uuid.Nil, err
	}
	models := v.Document.Models
	if selected != nil {
		models = *selected
	}
	// Recover an acknowledged durable save before consulting mutable policy,
	// pricing or revision state. Authorization and immutable choices still match.
	if receipt, err := s.Store.SavedDraftReceipt(ctx, actor, id, ws, artifactID, models, selected != nil, baseline); err != nil || receipt != uuid.Nil {
		return receipt, err
	}

	var artifact *Artifact
	for i := range v.Document.Artifacts {
		if v.Document.Artifacts[i].ID == artifactID && (v.Document.Artifacts[i].Accepted || len(approve) > 0 && approve[0]) {
			artifact = &v.Document.Artifacts[i]
		}
	}
	if artifact == nil || artifact.IsTestPlan() {
		return uuid.Nil, fault("artifact_required", "Review the agent and checks, then save them to your workspace.")
	}
	if artifact.IsTestSuite() && !suppliedImport(v.Document, *artifact) && s.Config.SourcePolicyVersion != "" {
		policy := policyFor(v.Document, artifact)
		if policy == nil {
			return uuid.Nil, sourceReviewRequired()
		}
		if _, err := verifiedPolicySources(v.Document, *policy); err != nil {
			return uuid.Nil, err
		}
	}
	if artifact.IsTestSuite() && !suppliedImport(v.Document, *artifact) && (s.Config.ReliableAuthoring || artifact.Validation != nil || artifact.Provenance == "ai_generated") && !s.currentArtifactPolicy(v.Document, *artifact) {
		return uuid.Nil, fault("tests_not_ready", "These tests need a rule review before they can be saved. Run the existing tests to review them, or describe the intended rules to prepare an update.")
	}
	if selected != nil {
		// Saving does not run a model. Explicit choices must still belong to the
		// configured catalog, not user-controlled provider/routing overrides.
		if err := s.Config.ValidateModels(*selected, false); err != nil {
			return uuid.Nil, err
		}
		models = *selected
	}
	c, err := s.Compiler.Compile(artifact.Blueprint, models.Evaluator, artifactID, s.Config.Limits(v.Anonymous))
	if err != nil {
		return uuid.Nil, err
	}
	return s.Store.saveDraft(ctx, actor, id, revision, ws, *artifact, c.Composition, models, selected != nil, baseline, approve...)
}
