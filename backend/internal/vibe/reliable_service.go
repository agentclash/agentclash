package vibe

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Empty versions preserve the v1 contract of existing admission fixtures and
// persisted plans. LoadConfig selects the current version for new operations.
func (c Config) reviewVersion() string {
	if c.SuiteReviewVersion == "" {
		return SuiteValidatorVersion
	}
	return c.SuiteReviewVersion
}
func (p Plan) reviewVersion() string {
	if p.Conversation == nil || p.Conversation.ValidatorVersion == "" {
		return SuiteValidatorVersion
	}
	return p.Conversation.ValidatorVersion
}
func (s *Service) freezeReviewVersion(p *Plan) error {
	version := s.Config.reviewVersion()
	if version != SuiteValidatorVersion && version != ConsistencySuiteValidatorVersion && version != LatestSuiteValidatorVersion {
		return fault("invalid_configuration", "The configured test review version is unavailable.")
	}
	p.Conversation.ValidatorVersion = version
	if version != SuiteValidatorVersion {
		l := p.limits()
		// The grounded consistency ledger needs room in the response. Admission
		// prices every call using this same frozen allowance before dispatch.
		l.OutputTokens = max(l.OutputTokens, 4096)
		if version == LatestSuiteValidatorVersion {
			// Live grounded reviews take longer than the old prose-only reply.
			// Freeze enough time for the bounded response and complete graph.
			l.ProviderSeconds = max(l.ProviderSeconds, 90)
			l.OperationSeconds = min(15*60, max(l.OperationSeconds, l.QueueSeconds+p.Calls*l.ProviderSeconds+30))
		}
		p.ExecutionLimits = &l
	}
	return nil
}

// Explicit editor writes use the same candidate gate as conversation. The
// provider runs in the worker, never inside the session edit transaction.
func (s *Service) PrepareSuiteEdit(ctx context.Context, actor string, id uuid.UUID, revision int64, artifactID uuid.UUID, blueprint json.RawMessage) (Operation, error) {
	v, err := s.Store.GetSession(ctx, actor, id)
	if err != nil {
		return Operation{}, err
	}
	if !s.Config.Enabled || s.Config.Credential == "" {
		return Operation{}, fault("hosted_disabled", "Test validation is unavailable right now. Your previous tests are unchanged.")
	}
	if err = s.Gate.Check(ctx, actor, s.Config.Limits(v.Anonymous)); err != nil {
		return Operation{}, err
	}
	if err = s.Config.ValidateModels(v.Document.Models, v.Anonymous); err != nil {
		return Operation{}, err
	}
	var a *Artifact
	for _, item := range v.Document.Artifacts {
		if item.ID == artifactID && item.IsTestSuite() {
			copy := item
			a = &copy
			break
		}
	}
	if a == nil {
		return Operation{}, fault("artifact_required", "Choose the tests to edit.")
	}
	if s.Config.SourcePolicyVersion != "" && !suppliedImport(v.Document, *a) {
		policy := policyFor(v.Document, a)
		if policy == nil {
			return Operation{}, sourceReviewRequired()
		}
		if _, err := verifiedPolicySources(v.Document, *policy); err != nil {
			return Operation{}, err
		}
	}
	l := s.Config.Limits(v.Anonymous)
	if _, err = s.Compiler.Compile(blueprint, v.Document.Models.Evaluator, artifactID, l); err != nil {
		return Operation{}, err
	}
	sub := Submission{ClientID: uuid.New(), Revision: revision, Kind: "message", Content: "Check and apply these manually edited test cases and expected answers. Preserve my existing business rules.", ArtifactID: &artifactID, Models: v.Document.Models, TestJourney: true}
	p := Plan{Submission: sub, Artifact: a, Document: v.Document, Anonymous: v.Anonymous, Free: s.Config.FreeOnly, LocalTesting: s.Config.TestingLocally()}
	if err = prepareReliableContext(&p, v, s.Config.SourcePolicyVersion); err != nil {
		return Operation{}, err
	}
	if err = s.freezeReviewVersion(&p); err != nil {
		return Operation{}, err
	}
	l = p.limits()
	if p.Conversation.Policy == nil {
		if p.sourceBoundary() {
			return Operation{}, sourceReviewRequired()
		}
		policy, sources, e := legacySuitePolicy(v.Document, *a)
		if e != nil {
			return Operation{}, e
		}
		p.Conversation.Policy = &policy
		p.Conversation.Sources = append(sources, p.Conversation.CurrentRequest)
	}
	p.Conversation.Manual = &ManualSuiteEdit{Blueprint: blueprint}
	p.Calls = 1
	profile, err := s.Config.Profile(sub.Models.Assistant)
	if err != nil {
		return Operation{}, err
	}
	p.Conversation.Profile = &profile
	p.MaxCost, err = profile.BoundCost(profile.inputLimit(l), l.OutputTokens)
	if err != nil {
		return Operation{}, err
	}
	return s.Store.Submit(ctx, actor, id, sub, p, s.Config)
}

func legacySuitePolicy(d Document, a Artifact) (PolicySnapshot, []SourceBlock, error) {
	policy := PolicySnapshot{ID: deterministicID(a.ID, "legacy-policy"), ScopeID: a.ID, SourceMessageID: a.SourceMessageID}
	sources := []SourceBlock{}
	foundSource := false
	for _, m := range d.Messages {
		if m.Role == "user" && m.Origin != "playground" {
			b := originalBlock(m.ID, m.Content)
			sources = append(sources, b)
			policy.Rules = append(policy.Rules, PolicyRule{ID: fmt.Sprintf("source-%d", len(sources)), Statement: m.Content, SourceBlockIDs: []string{b.ID}})
		}
		if m.ID == a.SourceMessageID {
			foundSource = true
			break
		}
	}
	if !foundSource || len(sources) == 0 || len(policy.Rules) > MaxRequirements {
		return policy, nil, fault("rules_required", "Describe the intended rules before running these older tests. Their original version is preserved.")
	}
	return policy, sources, nil
}
func suppliedImport(d Document, a Artifact) bool {
	if a.Provenance == "imported" {
		return true
	}
	if a.Provenance != "" {
		return false
	}
	for _, m := range d.Messages {
		if m.ID == a.SourceMessageID && m.OperationID == nil && strings.HasPrefix(m.Content, "Imported ") {
			return true
		}
	}
	return false
}
func validArtifactPolicy(d Document, a Artifact) bool {
	p := policyFor(d, &a)
	return p != nil && SuiteValidationMatches(a.Validation, a.Blueprint, *p)
}
func (s *Service) currentArtifactPolicy(d Document, a Artifact) bool {
	if s.Config.SourcePolicyVersion != "" {
		policy := policyFor(d, &a)
		if policy == nil {
			return false
		}
		if _, err := verifiedPolicySources(d, *policy); err != nil {
			return false
		}
	}
	return validArtifactPolicy(d, a) && (!s.Config.ReliableAuthoring || a.Validation.ValidatorVersion == s.Config.reviewVersion())
}
func (s *Service) prepareRunValidation(p *Plan, v Session) error {
	if p.Artifact != nil && s.verifiedSample(*p.Artifact, p.limits()) {
		return nil
	}
	if p.Artifact == nil || !p.Artifact.IsTestSuite() || suppliedImport(v.Document, *p.Artifact) {
		return nil
	}
	if s.Config.SourcePolicyVersion != "" {
		policy := policyFor(v.Document, p.Artifact)
		if policy == nil {
			return sourceReviewRequired()
		}
		if _, err := verifiedPolicySources(v.Document, *policy); err != nil {
			return err
		}
	}
	if s.currentArtifactPolicy(v.Document, *p.Artifact) {
		return nil
	}
	integrityValid := validArtifactPolicy(v.Document, *p.Artifact)
	if !integrityValid && (p.Artifact.Validation != nil || p.Artifact.Provenance == "ai_generated") {
		return fault("tests_not_ready", "These tests have changed since they were checked. Prepare or review the update before running them.")
	}
	if !s.Config.ReliableAuthoring {
		return nil
	}
	var policy PolicySnapshot
	var sources []SourceBlock
	if integrityValid {
		policy = *policyFor(v.Document, p.Artifact)
		if policy.SourceVersion == SourcePolicyVersion {
			sources, _ = verifiedPolicySources(v.Document, policy)
		} else {
			wanted := map[string]bool{}
			for _, rule := range policy.Rules {
				for _, id := range rule.SourceBlockIDs {
					wanted[id] = true
				}
			}
			wanted[policy.SourceMessageID.String()] = true
			for _, m := range v.Document.Messages {
				if m.Role == "user" && m.Origin != "playground" && wanted[m.ID.String()] {
					sources = append(sources, originalBlock(m.ID, m.Content))
				}
			}
		}
	} else {
		var err error
		policy, sources, err = legacySuitePolicy(v.Document, *p.Artifact)
		if err != nil {
			return err
		}
	}
	current := originalBlock(p.Submission.ClientID, "Review this existing suite against the original user rules before running it unchanged. Do not prepare a new suite.")
	p.Conversation = &ConversationContext{Policy: &policy, Sources: append(sources, current), CurrentRequest: current, RequiredCount: suiteCaseCount(p.Artifact.Blueprint), ContractVersion: "vibe-v11"}
	p.Conversation.SourceVersion = s.Config.SourcePolicyVersion
	p.AuthoringVersion = 11
	if err := s.freezeReviewVersion(p); err != nil {
		return err
	}
	profile, err := s.Config.Profile(p.Submission.Models.Assistant)
	if err != nil {
		return err
	}
	p.Conversation.Profile = &profile
	l := p.limits()
	// A historical review upgrade may increase output room. Reprice the entire
	// frozen graph so the target and judge cannot exceed the reserved amount.
	compiled, err := s.Compiler.Compile(p.Artifact.Blueprint, p.Submission.Models.Evaluator, p.Artifact.ID, l)
	if err != nil {
		return err
	}
	judges := len(compiled.Bundle.Version.EvaluationSpec.LLMJudges)
	target, err := s.Config.Profile(p.Submission.Models.Target)
	if err != nil {
		return err
	}
	judge, err := s.Config.Profile(p.Submission.Models.Evaluator)
	if err != nil {
		return err
	}
	targetCost, err := target.BoundCost(target.inputLimit(l), l.OutputTokens)
	if err != nil {
		return err
	}
	judgeCost, err := judge.BoundCost(judge.inputLimit(l), l.OutputTokens)
	if err != nil {
		return err
	}
	p.MaxCost = int64(len(p.Cases)) * (targetCost + int64(judges)*judgeCost)
	bound, err := profile.BoundCost(profile.inputLimit(l), l.OutputTokens)
	if err != nil {
		return err
	}
	p.Calls++
	p.MaxCost += bound
	if p.Calls > l.ModelCalls {
		return fault("graph_limit", "The tests and their initial review do not fit this operation.")
	}
	// Freeze the additional preflight allowance without changing the Temporal
	// activity ceiling. Large existing runs retain that ceiling.
	l.OperationSeconds = min(15*60, max(l.OperationSeconds, l.QueueSeconds+p.Calls*l.ProviderSeconds+30))
	p.ExecutionLimits = &l
	return nil
}

func (r *Runner) validateBeforeRun(ctx context.Context, o Operation, p *Plan) error {
	if p.Conversation == nil || p.Conversation.Policy == nil {
		return nil
	}
	profile, err := r.reliableProfile(o, *p)
	if err != nil {
		return err
	}
	input, err := BuildSuiteReviewInput(p.Artifact.Blueprint, *p.Conversation.Policy, p.Conversation.Sources, p.Conversation.CurrentRequest, len(p.Cases), p.limits())
	if err != nil {
		return err
	}
	if p.reviewVersion() != SuiteValidatorVersion {
		input.ValidatorVersion = p.reviewVersion()
	}
	resp, err := r.reliableCall(ctx, o, "suite-review", SuiteReviewMessages(input), SuiteReviewFormatFor(profile, input))
	if err != nil {
		return err
	}
	v, err := ParseSuiteReview([]byte(resp.OutputText), input, p.limits())
	reviewErr := err
	if err == nil && v.Status != SuiteSupported {
		reviewErr = fault(v.Status, v.ProblemSummary())
	}
	if e := r.journalReliable(ctx, o, "suite-review", "review", *p, p.Artifact.Blueprint, reviewErr); e != nil {
		return e
	}
	if err != nil {
		return fault("validation_unavailable", "The original tests could not be checked. No agent tests were run.")
	}
	if v.Status != SuiteSupported {
		return fault("test_policy_conflict", "These older tests need an update to match the supplied rules. No agent tests were run. Describe the intended change to prepare a corrected version.")
	}
	v.Model = o.Models.Assistant
	v.ProfileHash = Hash(raw(profile))
	p.Artifact.Validation = v
	p.Artifact.PolicyID = &p.Conversation.Policy.ID
	return r.Service.Store.recordSuiteValidation(ctx, o, p.Artifact, *p.Conversation.Policy)
}
func (s *Store) recordSuiteValidation(ctx context.Context, o Operation, a *Artifact, policy PolicySnapshot) error {
	return s.transaction(ctx, func(tx pgx.Tx) error {
		var state Execution
		if err := tx.QueryRow(ctx, "SELECT state FROM vibe_operations WHERE id=$1 FOR UPDATE", o.ID).Scan(&state); err != nil {
			return err
		}
		if state != Running {
			return fault("operation_stopped", "The operation was stopped.")
		}
		v, err := scanSession(tx.QueryRow(ctx, sessionSelect+" FOR UPDATE", o.SessionID))
		if err != nil {
			return err
		}
		for i, old := range v.Document.Artifacts {
			if old.ID == a.ID {
				before, _ := CanonicalJSONHash(old.Blueprint)
				after, _ := CanonicalJSONHash(a.Blueprint)
				if before != after {
					return fault("revision_conflict", "The tests changed before validation completed.")
				}
				if !SuiteValidationMatches(a.Validation, a.Blueprint, policy) {
					return fault("tests_not_ready", "The validation does not match these tests.")
				}
				v.Document.Artifacts[i].Validation = a.Validation
				v.Document.Artifacts[i].PolicyID = &policy.ID
				found := false
				for _, p := range v.Document.Policies {
					if p.ID == policy.ID {
						if Hash(raw(p)) != Hash(raw(policy)) {
							return fault("revision_conflict", "The rule revision changed before validation completed.")
						}
						found = true
					}
				}
				if !found {
					if len(v.Document.Policies) >= MaxRevisions {
						return fault("document_limit", "This conversation has too many saved rule versions.")
					}
					v.Document.Policies = append(v.Document.Policies, policy)
				}
				if err = s.updateDocument(ctx, tx, v); err != nil {
					return err
				}
				return event(ctx, tx, v.ID, &o.ID, "tests.validated")
			}
		}
		return fault("artifact_required", "The reviewed tests are unavailable.")
	})
}
