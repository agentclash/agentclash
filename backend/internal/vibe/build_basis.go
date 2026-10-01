package vibe

import (
	"strings"

	"github.com/google/uuid"
)

// Recover a test-only Build draft from independent, source-checked requirements.
// This creates instructions only; execution must still revalidate the tests.
func RecoverBuildInstructions(d Document, a Artifact) (string, error) {
	if d.Evaluation == nil || d.Evaluation.Door != "build" || !a.IsTestSuite() || a.AgentPrompt != "" {
		return "", fault("invalid_request", "Choose a saved Build draft that needs a prototype.")
	}
	p := policyFor(d, &a)
	if p == nil {
		return "", sourceReviewRequired()
	}
	if _, err := verifiedPolicySources(d, *p); err != nil {
		return "", err
	}
	rules := prototypeSources(*p)
	if len(rules) == 0 {
		return "", sourceReviewRequired()
	}
	return PreviewPrompt("Help with the following job, following these supplied rules:\n\n" + strings.Join(rules, "\n")), nil
}

// A sample supplies a scoped demonstration contract, never user facts. Its
// source is the server registry, independently of test inputs/expected answers.
func samplePolicy(kind string, scope uuid.UUID) PolicySnapshot {
	text := samplePrototype(kind).SuccessCriteria
	id := deterministicID(scope, "sample-assumptions-v1:"+kind)
	source := originalBlock(id, text)
	rule := PolicyRule{ID: "sample-assumptions", Statement: text, SourceBlockIDs: []string{source.ID}, Evidence: []RuleEvidence{{SourceBlockID: source.ID, Quote: text, Kind: "requirement"}}}
	return PolicySnapshot{ID: id, ScopeID: scope, SourceMessageID: id, SourceVersion: SourcePolicyVersion, SampleBasis: kind, Rules: []PolicyRule{rule}, Sources: projectedRuleSources([]PolicyRule{rule}, []SourceBlock{source})}
}

func verifiedSampleSources(d Document, p PolicySnapshot) ([]SourceBlock, error) {
	if p.SampleBasis != "returns" && p.SampleBasis != "email" && p.SampleBasis != "email_sorting" {
		return nil, sourceReviewRequired()
	}
	if d.Evaluation == nil || d.Evaluation.Door != "build" || d.ConversationState == nil || p.ScopeID.String() != d.ConversationState.Brief.ScopeID {
		return nil, sourceReviewRequired()
	}
	expected := samplePolicy(p.SampleBasis, p.ScopeID)
	if !sameJSON(raw(expected), raw(p)) {
		return nil, sourceReviewRequired()
	}
	return p.Sources, nil
}

func (s *Service) attachSampleBasis(p *Plan, v Session) {
	if p.Artifact == nil || p.Artifact.SampleBasis != nil || v.Document.ConversationState == nil || !s.verifiedSample(*p.Artifact, p.limits()) {
		return
	}
	// Only a persisted, server-created sample linked to this Build can be
	// recovered. A copied pack with the same contents is not a sample source.
	if p.Artifact.Provenance != "server_sample" || v.Document.Build == nil || v.Document.Build.ArtifactID == nil || *v.Document.Build.ArtifactID != p.Artifact.ID {
		return
	}
	scope, err := uuid.Parse(v.Document.ConversationState.Brief.ScopeID)
	if err != nil {
		return
	}
	basis := samplePolicy(p.Artifact.Sample, scope)
	p.Artifact.SampleBasis = &basis
}

func activeBuildArtifactID(d Document) *uuid.UUID {
	if d.ActiveArtifactID != nil {
		return d.ActiveArtifactID
	}
	if d.Build != nil && d.Build.ArtifactID != nil {
		return d.Build.ArtifactID
	}
	return nil
}

func prepareBuildContinuation(p *Plan, route *reliableRoute) {
	if route.Memory == nil || p.Cycle != nil || p.Conversation.Policy == nil || p.Conversation.Policy.SampleBasis == "" {
		return
	}
	if route.Intent == "clarify" && len(route.Memory.Facts) == 0 {
		route.Intent, route.Reply = "chat", "Your sample is still ready to try. We can check tougher situations using its sample rules, or you can tell me a rule to change."
		p.Conversation.NextState.PendingQuestion = p.Conversation.State.PendingQuestion
	}
	for _, fact := range route.Memory.Facts {
		if fact.Kind != "rule" {
			continue
		}
		// The actual rule starts a separate baseline. The original sample stays
		// selected/usable until the new version is explicitly tried or run.
		p.Conversation.Policy = nil
		p.Conversation.Sources = []SourceBlock{p.Conversation.CurrentRequest}
		p.Conversation.ObservedPolicy = nil
		p.ObservedArtifact, p.Observations = nil, nil
		route.Intent, route.Count = "prepare_tests", 3
		p.Conversation.NextState.PendingQuestion = nil
		p.Conversation.NextState.PendingPreparation = nil
		return
	}
}
