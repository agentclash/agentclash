package vibe

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/agentclash/agentclash/runtime/challengepack"
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
	l := LimitsFor(v.Anonymous)
	if sub.ClientID == uuid.Nil || sub.Revision < 0 || len(sub.Content) > l.MessageBytes {
		return Operation{}, fault("invalid_message", "The message is missing an ID or exceeds its size limit.")
	}
	if err = s.Config.ValidateModels(sub.Models, v.Anonymous); err != nil {
		return Operation{}, err
	}
	if err = s.Gate.Check(ctx, actor, l); err != nil {
		return Operation{}, err
	}
	p := Plan{Submission: sub, Document: v.Document, Anonymous: v.Anonymous, Free: s.Config.FreeOnly}
	if sub.Kind == "message" || sub.Kind == "build" {
		if strings.TrimSpace(sub.Content) == "" {
			return Operation{}, fault("invalid_message", "Write a message first.")
		}
		if v.Document.ActiveArtifactID != nil {
			for _, a := range v.Document.Artifacts {
				if a.ID == *v.Document.ActiveArtifactID && a.Accepted {
					copy := a
					p.Artifact = &copy
					break
				}
			}
		}
		if len(p.Document.Messages) > 6 {
			p.Document.Messages = p.Document.Messages[len(p.Document.Messages)-6:]
		}
		if len(p.Document.Artifacts) > 1 {
			p.Document.Artifacts = p.Document.Artifacts[len(p.Document.Artifacts)-1:]
		}
		for i := len(v.Operations) - 1; i >= 0; i-- {
			if len(v.Operations[i].Results) > 0 {
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
		p.Calls = 2 // one authoring call and, only if needed, one repair
		profile, e := s.Config.Profile(sub.Models.Assistant)
		if e != nil {
			return Operation{}, e
		}
		cost, e := profile.BoundCost(l.ContextTokens, l.OutputTokens)
		if e != nil {
			return Operation{}, e
		}
		p.MaxCost = cost * int64(p.Calls)
	} else if sub.Kind == "check" || sub.Kind == "retest" || sub.Kind == "playground" {
		p.Document = Document{}
		if sub.ArtifactID == nil {
			return Operation{}, fault("artifact_required", "Choose an accepted agent draft first.")
		}
		for _, a := range v.Document.Artifacts {
			if a.ID == *sub.ArtifactID && a.Accepted {
				copy := a
				p.Artifact = &copy
				break
			}
		}
		if p.Artifact == nil {
			return Operation{}, fault("artifact_required", "Accept the draft before checking it.")
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
			p.MaxCost, err = profile.BoundCost(l.ContextTokens, l.OutputTokens)
		} else {
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
			tc, e := target.BoundCost(l.ContextTokens, l.OutputTokens)
			if e != nil {
				return Operation{}, e
			}
			jc, e := judge.BoundCost(l.ContextTokens, l.OutputTokens)
			if e != nil {
				return Operation{}, e
			}
			p.MaxCost = int64(len(compiled.Cases)) * (tc + int64(judges)*jc)
			for _, c := range compiled.Cases {
				p.Cases = append(p.Cases, c.CaseKey)
			}
		}
	} else {
		return Operation{}, fault("invalid_operation", "This operation is not supported.")
	}
	if err != nil {
		return Operation{}, err
	}
	return s.Store.Submit(ctx, actor, id, sub, p, s.Config)
}
func (s *Service) Import(ctx context.Context, actor string, id uuid.UUID, revision int64, content []byte) error {
	v, err := s.Store.GetSession(ctx, actor, id)
	if err != nil {
		return err
	}
	l := LimitsFor(v.Anonymous)
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
	if agentPrompt == "" {
		agentPrompt = c.Bundle.Challenges[0].Instructions
	}
	return s.Store.Edit(ctx, actor, id, revision, func(v *Session) error {
		if v.Document.AttachmentCount >= l.Files {
			return fault("attachment_limit", "This conversation has reached its import count limit. Save it and start another.")
		}
		total := len(content)
		for _, a := range v.Document.Artifacts {
			total += len(a.Blueprint)
		}
		if total > l.StoredBytes {
			return fault("attachment_limit", "The conversation's attachment allowance is full.")
		}
		v.Document.AttachmentCount++
		msg := Message{uuid.New(), "user", fmt.Sprintf("Imported %s (%d cases).", c.Bundle.Pack.Name, len(c.Cases)), timestamp()}
		v.Document.Messages = append(v.Document.Messages, msg)
		v.Document.Artifacts = append(v.Document.Artifacts, Artifact{ID: artifactID, Title: c.Bundle.Pack.Name, AgentPrompt: agentPrompt, Blueprint: b, SourceMessageID: msg.ID, CreatedAt: timestamp()})
		v.Document.Messages = append(v.Document.Messages, Message{uuid.New(), "assistant", "Your evaluation is ready to review. Check the agent instructions and the examples, then accept the draft when they match what you want to test. Nothing has run yet.", timestamp()})
		return nil
	})
}
func (s *Service) Save(ctx context.Context, actor string, id uuid.UUID, revision int64, artifactID, ws uuid.UUID, selected *Models) (uuid.UUID, error) {
	v, err := s.Store.GetSession(ctx, actor, id)
	if err != nil {
		return uuid.Nil, err
	}
	var artifact *Artifact
	for i := range v.Document.Artifacts {
		if v.Document.Artifacts[i].ID == artifactID && v.Document.Artifacts[i].Accepted {
			artifact = &v.Document.Artifacts[i]
		}
	}
	if artifact == nil {
		return uuid.Nil, fault("artifact_required", "Accept an evaluation draft before saving.")
	}
	models := v.Document.Models
	if selected != nil {
		// Saving does not run a model. Explicit choices must still belong to the
		// configured catalog, not user-controlled provider/routing overrides.
		if err := s.Config.ValidateModels(*selected, false); err != nil {
			return uuid.Nil, err
		}
		models = *selected
	}
	c, err := s.Compiler.Compile(artifact.Blueprint, models.Evaluator, artifactID, LimitsFor(v.Anonymous))
	if err != nil {
		return uuid.Nil, err
	}
	return s.Store.SaveDraft(ctx, actor, id, revision, ws, *artifact, c.Composition, models, selected != nil)
}
